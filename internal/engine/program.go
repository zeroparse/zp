package engine

import (
	"context"
	"slices"
	"sync"
	"zeroparse.dev/zp/internal/cancel"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/desugar"
	"zeroparse.dev/zp/internal/engine/closure"
	"zeroparse.dev/zp/internal/engine/core"
	"zeroparse.dev/zp/internal/engine/vm"
)

// sharedProgram names the part of a program that every backend shares, so that Program embeds it
// under an unexported field name.
type sharedProgram = core.SharedProgram

// Program is a compiled grammar. It can be used by multiple parses concurrently.
type Program struct {
	sharedProgram
	// buildFamily builds the rules that the closure backend runs in a family, by ID (rule.id); it is
	// nil when the closure backend cannot run the program (a program loaded without its grammar).
	// families holds the rules of each family (closure.Inlined, closure.CallPreserving), built on
	// first use.
	buildFamily func(closure.Family) ([]*core.Rule, error)
	families    [2]closureFamily

	budgetOnce    sync.Once
	budgetProgram *Program
	budgetErr     error

	recOnce sync.Once
	rec     *Program // program that builds no tree (recognizer)
	recErr  error
	vmOnce  sync.Once
	vm      *vm.Program
	ivmOnce sync.Once
	ivm     *vm.Program
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

// closureFamily holds the rules of a closure family once a parse needs them.
type closureFamily struct {
	once  sync.Once
	rules []*core.Rule
	err   error
}

// newProgram returns the program that runs the static program sp; buildFamily builds the rules of
// the closure backend in a family, or is nil when that backend cannot run sp.
func newProgram(sp *compile.Program, buildFamily func(closure.Family) ([]*core.Rule, error)) *Program {
	prog := &Program{buildFamily: buildFamily}
	core.InitSharedProgram(&prog.sharedProgram, sp)
	return prog
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

// Module compiles the program to bytecode. The result is stored in the program and reused on
// subsequent calls.
func (prog *Program) Module() *bytecode.Module { return prog.BytecodeModule() }

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
	return build(g, opts, nil)
}

// build builds a program from the grammar. If flags is not nil, it skips static analysis and
// type checking and uses flags instead.
func build(g *grammar.Grammar, opts compile.Options, flags []compile.RuleFlags) (*Program, error) {
	return buildControlled(g, opts, flags, nil)
}

func buildControlled(g *grammar.Grammar, opts compile.Options, flags []compile.RuleFlags, control *cancel.Control) (*Program, error) {
	names := desugar.InlineNames{}
	prog, err := buildNamed(g, opts, flags, names, control)
	if err != nil {
		return nil, names.Rewrite(err)
	}
	return prog, nil
}

// CompileContext is the internal editor compilation entry point. Cancellation
// discards the entire program and accumulated diagnostics. Existing Compile and
// saved-program loading keep their ordinary path with no active control.
// Polls cover grammar AST validation, source normalization, rule/dependency
// loops, shared success proofs, expression checking, type checking, the Core IR
// build, its analysis, passes and verification. The closures are built from the
// Core IR when a parse first needs them, outside the compilation, and are not
// interrupted. Shared metadata and individual operations still have
// uncancellable stretches.
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
	return buildControlled(g, opts, nil, &cancel.Control{Ctx: ctx})
}

// buildNamed is buildControlled; names receives the hidden rules that the lowering of inline actions
// generates.
func buildNamed(g *grammar.Grammar, opts compile.Options, flags []compile.RuleFlags, names desugar.InlineNames, control *cancel.Control) (*Program, error) {
	prog, err := compile.CompileProgram(g, opts, flags, names, control, func(*compile.Compiler) compile.ExprBuilder[struct{}] { return compile.NoExprs{} })
	if err != nil {
		return nil, err
	}
	m, err := closure.Core(prog, control)
	if err != nil {
		return nil, err
	}
	// The closures of each family are built from m on first use, inside the parse that needs them
	// and outside the compilation and its control (Build only reads m).
	plainDispatch := !opts.ControlledDispatch
	return newProgram(prog, func(f closure.Family) ([]*core.Rule, error) { return closure.Build(prog, m, plainDispatch, f) }), nil
}
