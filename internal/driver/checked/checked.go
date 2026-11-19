// Package checked is the compiler side of a program compiled from a grammar: it compiles a grammar
// to a checked program and supplies, to the parses that run its run-time program, what they build
// from the grammar on first use (state.Source): the Core IR and closures of the closure backend,
// the bytecode module of the VMs, and the programs compiled again for recognition and for
// missing-token insertion. The runtime reads none of the compiler's representations; it reaches them
// only through a Source.
package checked

import (
	"context"

	"zeroparse.dev/zp/internal/backend/closure"
	"zeroparse.dev/zp/internal/backend/vmlower"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/frontend"
	"zeroparse.dev/zp/internal/frontend/desugar"
	"zeroparse.dev/zp/internal/frontend/schemaexport"
	ir "zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/runtime/program"
	"zeroparse.dev/zp/internal/runtime/state"
	"zeroparse.dev/zp/pkg/bytecode"
	"zeroparse.dev/zp/pkg/grammar/ast"
	"zeroparse.dev/zp/pkg/runtime/schema"
)

// Source is the compiler side of a checked program (state.Source).
type Source struct {
	prog *compile.Program
}

var _ state.Source = (*Source)(nil)

// Compile compiles the grammar and checks its declarations of linear cost; control, if not nil,
// polls the compilation and may cancel it with a cancel.Cancellation panic.
func Compile(g *ast.Grammar, opts compile.Options, control *cancel.Control) (*Source, error) {
	s, err := Check(g, opts, nil, control)
	if err != nil {
		return nil, err
	}
	if err := checkDeclaredLinear(g, control); err != nil {
		return nil, err
	}
	return s, nil
}

// Check compiles the grammar to a checked program without checking its declarations of linear
// cost. If flags is not nil, it skips the static analysis of the rules and uses flags instead. The
// errors name the rules as written, not the hidden rules that inline actions are lowered to.
func Check(g *ast.Grammar, opts compile.Options, flags []compile.RuleFlags, control *cancel.Control) (*Source, error) {
	names := desugar.InlineNames{}
	prog, err := frontend.Check(g, opts, flags, names, control)
	if err != nil {
		return nil, names.Rewrite(err)
	}
	return &Source{prog: prog}, nil
}

// Of returns the Source of the checked program prog.
func Of(prog *compile.Program) *Source { return &Source{prog: prog} }

// Checked returns the checked program, which the generators, the analyses and the output schema
// read.
func (s *Source) Checked() *compile.Program { return s.prog }

// Schema returns the checked grammar that the output schema is exported from.
func (s *Source) Schema() schemaexport.Source {
	return schemaexport.Source{Model: s.prog.Model, Checker: s.prog.Checker(), SourceInfo: s.prog.RecordsSourceInfo}
}

// Grammar returns the grammar the program was compiled from: the grammar as written, without the
// hidden rules that inline actions are lowered to.
func (s *Source) Grammar() *ast.Grammar {
	if s.prog.Source != nil {
		return s.prog.Source
	}
	return s.prog.Grammar
}

// Runtime returns the run-time program of the checked program.
func (s *Source) Runtime() *program.Program { return s.prog.Runtime() }

// Module compiles the program to a bytecode module (vmlower.Compile), once.
func (s *Source) Module() (*bytecode.Module, error) {
	p := s.prog
	p.BytecodeOnce.Do(func() { p.Bytecode, p.BytecodeErr = vmlower.Compile(p) })
	return p.Bytecode, p.BytecodeErr
}

// Core builds the Core IR of the program for the closures (closure.Core), polling ctx (nil for
// none): an ended context returns a state.BuildCancelled.
func (s *Source) Core(ctx context.Context) (m any, err error) {
	if ctx == nil {
		return closure.Core(s.prog, nil)
	}
	defer func() {
		if x := recover(); x != nil {
			cancelled, ok := x.(cancel.Cancellation)
			if !ok {
				panic(x)
			}
			m, err = nil, state.BuildCancelled{Err: cancelled.Err}
		}
	}()
	return closure.Core(s.prog, &cancel.Control{Ctx: attributedContext{ctx}})
}

// attributedContext is a parse's context whose Err attributes a panic to the caller's context
// (state.ContextPanic), as the parse's own checks do, so the parse re-raises the caller's value.
type attributedContext struct{ context.Context }

func (c attributedContext) Err() error {
	defer func() {
		if x := recover(); x != nil {
			switch x.(type) {
			case state.Fatal, state.ContextPanic:
				panic(x) // already attributed
			default:
				panic(state.ContextPanic{Value: x})
			}
		}
	}()
	return c.Context.Err()
}

// Family builds the closures of the family f from the Core IR core (closure.Build).
func (s *Source) Family(core any, f state.Family, plainDispatch bool) ([]*state.Rule, error) {
	return closure.Build(s.prog, core.(*ir.Module), plainDispatch, f)
}

// Recognizer compiles the grammar again as a program that builds no tree, with the rule flags of
// this one.
func (s *Source) Recognizer() (state.Source, error) {
	return s.again(func(o *compile.Options) { o.Recognition = compile.Recognize }, s.prog.Flags())
}

// Insertion compiles the grammar again with the rule flags of missing-token insertion.
func (s *Source) Insertion() (state.Source, error) {
	return s.again(func(*compile.Options) {}, s.prog.InsertionFlags)
}

// again compiles the grammar again with the rule flags and the options the program was compiled
// with, changed by derive for what the build is for. This is the one place that makes them, so no
// option is dropped from a program built again (TestLazyProgramsCarryEveryOption).
func (s *Source) again(derive func(*compile.Options), flags []compile.RuleFlags) (state.Source, error) {
	o := s.prog.Opts
	derive(&o)
	return Check(s.Grammar(), o, flags, nil)
}

// RuleType is compile.Program.RuleType.
func (s *Source) RuleType(name string) (string, bool) { return s.prog.RuleType(name) }

// RuleTypeWithPoll is compile.Program.RuleTypeWithPoll.
func (s *Source) RuleTypeWithPoll(name string, poll func()) (string, bool) {
	return s.prog.RuleTypeWithPoll(name, poll)
}

// ShapeSignature is compile.Program.ShapeSignature.
func (s *Source) ShapeSignature(name string, poll func()) (string, bool) {
	return s.prog.ShapeSignature(name, poll)
}

// ValueType is compile.Program.ValueType.
func (s *Source) ValueType(name string) (string, bool) { return s.prog.ValueType(name) }

// OutputSchema exports the output schema of the rule start, or of every rule and type with
// allRules, from the checked grammar.
func (s *Source) OutputSchema(start string, allRules bool) (*schema.Output, error) {
	return schemaexport.Export(s.Schema(), start, allRules)
}
