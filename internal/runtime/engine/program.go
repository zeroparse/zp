package engine

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"

	"zeroparse.dev/zp/internal/runtime/program"
	"zeroparse.dev/zp/internal/runtime/state"
	"zeroparse.dev/zp/internal/runtime/vm"
	"zeroparse.dev/zp/pkg/bytecode"
)

// sharedProgram names the part of a program that every backend shares, so that Program embeds it
// under an unexported field name.
type sharedProgram = state.SharedProgram

// A Source is the compiler side of a program compiled from a grammar (state.Source).
type Source = state.Source

// Program is a compiled grammar. It can be used by multiple parses concurrently.
type Program struct {
	sharedProgram
	// src is the compiler side of a program compiled from a grammar, nil for a program loaded from
	// a compiled grammar, which only the bytecode backends run.
	src Source
	// plainDispatch leaves out the budget charge of the loop iterations in the closure families
	// (Source.Family): the metered program that runs the parses under budget controls charges them.
	plainDispatch bool
	// families holds the rules of each closure family (state.Inlined, state.CallPreserving), built
	// on first use from core.
	families [2]closureFamily
	core     *closureCore

	budgetOnce    sync.Once
	budgetProgram *Program

	recOnce sync.Once
	rec     *Program // program that builds no tree (recognizer)
	recErr  error
	// vmTerms holds the programs of the module's terms, which both VMs run; a loaded program has
	// them from the checks of its file.
	vmTermsOnce sync.Once
	vmTerms     []*state.TermProgram
	vmTermsErr  error
	// vms holds the VM programs of the module, prepared on first use, by VM (vmIndex) and family
	// (state.Family).
	vms [2][2]familyVM
	// loaded reports that the program was loaded from a compiled grammar, whose module serves the
	// parses of mode only (checkMode).
	loaded bool
	mode   CompiledMode
	// ins is the program that the parses with missing-token insertion enabled run when the
	// grammar has left recursion through an insertion (InsertionCycles); it is built on first use.
	insOnce sync.Once
	ins     *Program
	insErr  error
}

// closureFamily holds the rules of a closure family once a parse needs them. The build runs under
// mu; built is set once rules and err are final. A build that the parse's context cancels leaves
// the family unbuilt, and the next parse that needs it builds it.
type closureFamily struct {
	built atomic.Bool
	mu    sync.Mutex
	rules []*state.Rule
	err   error
}

// New returns the program that runs the program compiled from a grammar that src is the compiler
// side of.
func New(src Source) *Program {
	return newProgram(src.Runtime(), src)
}

// newProgram returns the program that runs the run-time program rt; src is the compiler side of a
// program compiled from a grammar (Source), or nil for a program loaded from a compiled grammar.
func newProgram(rt *program.Program, src Source) *Program {
	prog := &Program{src: src, plainDispatch: true}
	if src != nil {
		prog.core = newClosureCore(src)
	}
	state.InitSharedProgram(&prog.sharedProgram, rt)
	return prog
}

// Runtime returns the run-time program that p runs.
func (p *Program) Runtime() *program.Program { return state.StaticProgramOf(&p.sharedProgram) }

// Source returns the compiler side of p (Source), nil for a program loaded from a compiled
// grammar.
func (p *Program) Source() Source { return p.src }

// closureCore is what the closure families of a program, and of the metered program that runs its
// parses under budget controls (prepareControlledProgram), are built from (Source.Core). The first
// build of a family builds it. Nothing else reads it, so it is dropped once no family that reads it
// is left to build: pending counts the families of those programs whose build has not returned. A
// family that needs it after that builds it again.
type closureCore struct {
	src     Source
	mu      sync.Mutex
	m       any
	pending int
}

// newClosureCore returns the core of src, not built yet, for the closure families of one program.
func newClosureCore(src Source) *closureCore {
	c := &closureCore{src: src}
	c.share()
	return c
}

// share counts the families of one more program that read the core.
func (c *closureCore) share() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pending += len(Program{}.families)
}

// module returns the core for the build of a family, building it if it is not held; the build
// calls done when it returns. The build runs without holding c.mu, so a context method that parses on
// the same program cannot deadlock it: concurrent builds may race, and the first core published
// wins. A core built while no family is pending (the families were built meanwhile) is returned
// without being held. The build polls ctx, the context of the parse that needs the family (nil for
// none): if it ends, module returns a state.BuildCancelled with its error and holds no core; a panic
// of a method of ctx unwinds as state.ContextPanic and also leaves no core held.
func (c *closureCore) module(ctx context.Context) (m any, err error) {
	c.mu.Lock()
	m = c.m
	c.mu.Unlock()
	if m != nil {
		return m, nil
	}
	if m, err = c.src.Core(ctx); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m != nil {
		return c.m, nil
	}
	if c.pending > 0 {
		c.m = m
	}
	return m, nil
}

// done records that the build of a family returned, and drops the core after the last one.
func (c *closureCore) done() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pending--; c.pending == 0 {
		c.m = nil
	}
}

// SourceInfo reports whether the program records source information in its results
// (Options.SourceInfo).
func (p *Program) SourceInfo() bool { return p.RecordsSourceInfo }

// DefaultValue reports whether the program was compiled to make default values (Options.DefaultValue).
func (p *Program) DefaultValue() bool { return p.MakesDefaultValue }

// Starts returns the start rules of the grammar, the rules marked @start where
// parsing may begin, in declaration order.
func (prog *Program) Starts() []string { return slices.Clone(prog.StartRules) }

// DefaultStart returns the default start rule: the one marked
// @start(default), or the only start rule.
func (prog *Program) DefaultStart() string { return prog.DefaultStartRule }

// CheckStart reports an error unless name selects a start rule: it is the name
// of a start rule, or empty for the default start rule.
func (prog *Program) CheckStart(name string) error { return prog.Runtime().CheckStart(name) }

// Module returns the bytecode module of the program: the module of a program loaded from a
// compiled grammar, or the compilation of a program compiled from a grammar (Source.Module), which
// the source makes once and reuses. It panics for a program whose grammar uses #missing, which has no bytecode form,
// and on an internal error.
func (prog *Program) Module() *bytecode.Module {
	if prog.HasMissing {
		panic(program.MissingUnsupported("bytecode module construction"))
	}
	if prog.src == nil {
		return prog.Bytecode
	}
	m, err := prog.src.Module()
	if err != nil {
		panic(err)
	}
	return m
}

// moduleTerms returns the module m (Module) and the programs of its terms, loading them once for
// both VMs.
func (prog *Program) moduleTerms() (*bytecode.Module, []*state.TermProgram, error) {
	m := prog.Module()
	prog.vmTermsOnce.Do(func() {
		prog.vmTerms, prog.vmTermsErr = vm.LoadTerms(m)
		if prog.vmTermsErr != nil {
			// A loaded file's terms were checked when it was loaded; a compiled module's are the
			// compiler's.
			prog.vmTermsErr = fmt.Errorf("internal error: %w", prog.vmTermsErr)
		}
	})
	return m, prog.vmTerms, prog.vmTermsErr
}

// RuleType returns the type of the value of the rule name: its declared type, or the inferred
// type of its value (for example "[]Expr | node[[]node[string]]"). In an inferred type, a
// reference to another rule is that rule's declared type, or the rule's name if it has none. It
// reports false if the program has no such rule or its types were not checked
// (a program loaded from a compiled grammar).
func (prog *Program) RuleType(name string) (string, bool) {
	if prog.src == nil {
		return "", false
	}
	return prog.src.RuleType(name)
}

// RuleTypeWithPoll is RuleType with invocation-owned polling for editor display.
// It does not retain poll. Callback panics propagate unchanged to the caller.
// Individual allocations and string comparisons/concatenations may finish
// before the next callback. A nil callback uses the ordinary type renderer.
func (prog *Program) RuleTypeWithPoll(name string, poll func()) (string, bool) {
	if prog.src == nil {
		return "", false
	}
	return prog.src.RuleTypeWithPoll(name, poll)
}

// ShapeSignature returns a text that describes the elements and captures that every subexpression
// of the rule name contributes to the tuple of its scope, in preorder, leaving out the lookaheads
// and what they hold. The type of a rule hides a change inside an action, such as a discard that
// moved to another element, but the shapes of the subexpressions show it. It reports false if
// the program has no such rule or its types were not checked. poll, when not nil, is called at
// every subexpression.
func (prog *Program) ShapeSignature(name string, poll func()) (string, bool) {
	if prog == nil || prog.src == nil {
		return "", false
	}
	return prog.src.ShapeSignature(name, poll)
}

// ValueType returns the type of the value that the rule name builds in the value model (for
// example "node[(*node[string],)]"). It reports false if the program has no such rule or its
// types were not checked.
func (prog *Program) ValueType(name string) (string, bool) {
	if prog.src == nil {
		return "", false
	}
	return prog.src.ValueType(name)
}
