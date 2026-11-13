package engine

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/desugar"
	"zeroparse.dev/zp/internal/engine/closure"
	"zeroparse.dev/zp/internal/engine/core"
	"zeroparse.dev/zp/internal/engine/vm"
	ir "zeroparse.dev/zp/internal/ir/core"
)

// sharedProgram names the part of a program that every backend shares, so that Program embeds it
// under an unexported field name.
type sharedProgram = core.SharedProgram

// Program is a compiled grammar. It can be used by multiple parses concurrently.
type Program struct {
	sharedProgram
	// buildFamily builds, from the program's Core IR (coreIR), the rules that the closure backend
	// runs in a family, by ID (rule.id); it is nil when the closure backend cannot run the program (a
	// program loaded without its grammar). families holds the rules of each family (closure.Inlined,
	// closure.CallPreserving), built on first use.
	buildFamily func(*ir.Module, closure.Family) ([]*core.Rule, error)
	families    [2]closureFamily
	coreIR      *closureCore

	budgetOnce    sync.Once
	budgetProgram *Program

	recOnce sync.Once
	rec     *Program // program that builds no tree (recognizer)
	recErr  error
	// vmTerms holds the programs of the module's terms, which both VMs run; a loaded program has
	// them from the checks of its file.
	vmTermsOnce sync.Once
	vmTerms     []*core.TermProgram
	vmTermsErr  error
	vmOnce      sync.Once
	vm          *vm.Program
	ivmOnce     sync.Once
	ivm         *vm.Program
	// loaded reports that the program was loaded from a compiled grammar, whose module serves the
	// parses of mode only (checkMode).
	loaded bool
	mode   CompiledMode
	// ins is the program that the parses with missing-token insertion enabled run when the
	// grammar has left recursion through an insertion (InsertionFlags); it is built on first use.
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
	rules []*core.Rule
	err   error
}

// newProgram returns the program that runs the static program sp; buildFamily builds the rules of
// the closure backend in a family from the Core IR of c, or is nil when that backend cannot run sp.
func newProgram(sp *compile.Program, c *closureCore, buildFamily func(*ir.Module, closure.Family) ([]*core.Rule, error)) *Program {
	prog := &Program{buildFamily: buildFamily, coreIR: c}
	core.InitSharedProgram(&prog.sharedProgram, sp)
	return prog
}

// closureProgram returns the program that runs the static program sp with the closure families
// built from the Core IR of c; plainDispatch leaves out the budget charge of the loop iterations
// (closure.Build).
func closureProgram(sp *compile.Program, c *closureCore, plainDispatch bool) *Program {
	return newProgram(sp, c, func(m *ir.Module, f closure.Family) ([]*core.Rule, error) {
		return closure.Build(sp, m, plainDispatch, f)
	})
}

// closureCore is the Core IR of the static program sp that the closure families of a program, and
// of the metered program that runs its parses under budget controls (prepareControlledProgram), are
// built from. The first build of a family builds it. Nothing else reads it, so it is dropped once no
// family that reads it is left to build: pending counts the families of those programs whose build
// has not returned. A family that needs it after that builds it again.
type closureCore struct {
	sp      *compile.Program
	mu      sync.Mutex
	m       *ir.Module
	pending int
}

// newClosureCore returns the Core IR of sp, not built yet, for the closure families of one program.
func newClosureCore(sp *compile.Program) *closureCore {
	c := &closureCore{sp: sp}
	c.share()
	return c
}

// share counts the families of one more program that read the Core IR.
func (c *closureCore) share() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pending += len(Program{}.families)
}

// module returns the Core IR for the build of a family, building it if it is not held; the build
// calls done when it returns. The build runs without holding c.mu, so a context method that parses on
// the same program cannot deadlock it: concurrent builds may race, and the first module published
// wins. A module built while no family is pending (the families were built meanwhile) is returned
// without being held. The build polls ctx, the context of the parse that needs the family (nil for
// none): if it ends, module returns a buildCancelled with its error and holds no module; a panic of a
// method of ctx unwinds as core.ContextPanic and also leaves no module held.
func (c *closureCore) module(ctx context.Context) (m *ir.Module, err error) {
	c.mu.Lock()
	m = c.m
	c.mu.Unlock()
	if m != nil {
		return m, nil
	}
	if m, err = buildCore(c.sp, ctx); err != nil {
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

// buildCore builds the Core IR of sp for the closures (closure.Core), polling ctx (nil for none):
// an ended context returns a buildCancelled.
func buildCore(sp *compile.Program, ctx context.Context) (m *ir.Module, err error) {
	if ctx == nil {
		return closure.Core(sp, nil)
	}
	defer func() {
		if x := recover(); x != nil {
			cancelled, ok := x.(cancel.Cancellation)
			if !ok {
				panic(x)
			}
			m, err = nil, buildCancelled{cancelled.Err}
		}
	}()
	return closure.Core(sp, &cancel.Control{Ctx: attributedContext{ctx}})
}

// attributedContext is a parse's context whose Err attributes a panic to the caller's context
// (core.ContextPanic), as the parse's own checks do, so the parse re-raises the caller's value.
type attributedContext struct{ context.Context }

func (c attributedContext) Err() error {
	defer func() {
		if x := recover(); x != nil {
			switch x.(type) {
			case core.Fatal, core.ContextPanic:
				panic(x) // already attributed
			default:
				panic(core.ContextPanic{Value: x})
			}
		}
	}()
	return c.Context.Err()
}

// buildCancelled is the error of a build of the Core IR that the parse's context ended: err is the
// context's error, which the parse reports.
type buildCancelled struct{ err error }

func (e buildCancelled) Error() string { return e.err.Error() }

// done records that the build of a family returned, and drops the Core IR after the last one.
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

// SourceGrammar returns the grammar the program was compiled from: the grammar as written, without
// the hidden rules that inline actions are lowered to.
func (p *Program) SourceGrammar() *grammar.Grammar {
	if p.Source != nil {
		return p.Source
	}
	return p.Grammar
}

// Starts returns the start rules of the grammar, the rules marked @start where
// parsing may begin, in declaration order.
func (prog *Program) Starts() []string { return slices.Clone(prog.StartRules) }

// DefaultStart returns the default start rule: the one marked
// @start(default), or the only start rule.
func (prog *Program) DefaultStart() string { return prog.DefaultStartRule }

// CheckStart reports an error unless name selects a start rule: it is the name
// of a start rule, or empty for the default start rule.
func (prog *Program) CheckStart(name string) error { return prog.Static().CheckStart(name) }

// Module compiles the program to bytecode (vm.Compile). The result is stored in the program and
// reused on subsequent calls. It panics for a program whose grammar uses #missing, which has no
// bytecode form, and on an internal error.
func (prog *Program) Module() *bytecode.Module {
	s := prog.Static()
	if s.HasMissing {
		panic(compile.MissingUnsupported("bytecode module construction"))
	}
	s.BytecodeOnce.Do(func() { s.Bytecode, s.BytecodeErr = vm.Compile(s) })
	if s.BytecodeErr != nil {
		panic(s.BytecodeErr)
	}
	return s.Bytecode
}

// moduleTerms returns the module m (Module) and the programs of its terms, loading them once for
// both VMs.
func (prog *Program) moduleTerms() (*bytecode.Module, []*core.TermProgram, error) {
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
func (prog *Program) RuleType(name string) (string, bool) { return prog.Static().RuleType(name) }

// RuleTypeWithPoll is RuleType with invocation-owned polling for editor display.
// It does not retain poll. Callback panics propagate unchanged to the caller.
// Individual allocations and string comparisons/concatenations may finish
// before the next callback. A nil callback uses the ordinary type renderer.
func (prog *Program) RuleTypeWithPoll(name string, poll func()) (string, bool) {
	return prog.Static().RuleTypeWithPoll(name, poll)
}

// ShapeSignature returns a text that describes the elements and captures that every subexpression
// of the rule name contributes to the tuple of its scope, in preorder, leaving out the lookaheads
// and what they hold. The type of a rule hides a change inside an action, such as a discard that
// moved to another element, but the shapes of the subexpressions show it. It reports false if
// the program has no such rule or its types were not checked. poll, when not nil, is called at
// every subexpression.
func (prog *Program) ShapeSignature(name string, poll func()) (string, bool) {
	if prog == nil {
		return "", false
	}
	return prog.Static().ShapeSignature(name, poll)
}

// ValueType returns the type of the value that the rule name builds in the value model (for
// example "node[(*node[string],)]"). It reports false if the program has no such rule or its
// types were not checked.
func (prog *Program) ValueType(name string) (string, bool) { return prog.Static().ValueType(name) }

// Compile compiles the grammar.
func Compile(g *grammar.Grammar, opts Options) (*Program, error) {
	return compileChecked(g, opts, nil)
}

// build builds a program from the grammar. If flags is not nil, it skips static analysis and
// type checking and uses flags instead.
func build(g *grammar.Grammar, opts compile.Options, flags []compile.RuleFlags) (*Program, error) {
	return buildControlled(g, opts, flags, nil)
}

func buildControlled(g *grammar.Grammar, opts compile.Options, flags []compile.RuleFlags, control *cancel.Control) (*Program, error) {
	names := desugar.InlineNames{}
	prog, err := compile.Check(g, opts, flags, names, control)
	if err != nil {
		return nil, names.Rewrite(err)
	}
	return lower(prog), nil
}

// compileChecked builds the program of g and checks its declarations of linear cost. The programs
// that a compiled program builds again from its grammar, for other modes, skip the check, which
// the first compilation made.
func compileChecked(g *grammar.Grammar, opts compile.Options, control *cancel.Control) (*Program, error) {
	prog, err := buildControlled(g, opts, nil, control)
	if err != nil {
		return nil, err
	}
	if err := checkDeclaredLinear(g, control); err != nil {
		return nil, err
	}
	return prog, nil
}

// CompileContext is the internal editor compilation entry point. Cancellation
// discards the entire program and accumulated diagnostics. Existing Compile and
// saved-program loading keep their ordinary path with no active control.
// Polls cover grammar AST validation, source normalization, rule/dependency
// loops, shared success proofs, expression checking and type checking. The Core
// IR and the closures built from it are made when a parse first needs them,
// outside the compilation: the parse's own context interrupts the Core IR build
// and its passes, not the closures. Shared metadata and individual operations
// still have uncancellable stretches.
// This is not a wall-clock latency guarantee.
func CompileContext(ctx context.Context, g *grammar.Grammar, opts Options) (prog *Program, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if ctx.Done() == nil {
		return Compile(g, opts)
	}
	defer func() {
		if failure := recover(); failure != nil {
			if cancelled, ok := failure.(cancel.Cancellation); ok {
				prog, err = nil, cancelled.Err
			} else {
				panic(failure)
			}
		}
	}()
	return compileChecked(g, opts, &cancel.Control{Ctx: ctx})
}

// lower returns the program that runs the checked program prog. Its Core IR and the closures of
// each family built from it are made on first use, inside the parse that needs them and outside the
// compilation and its control; the parse's context interrupts the Core IR build (closureCore).
func lower(prog *compile.Program) *Program {
	return closureProgram(prog, newClosureCore(prog), true)
}
