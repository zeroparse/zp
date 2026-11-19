package engine

import (
	"context"
	"zeroparse.dev/zp/internal/bytecode"

	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/driver/checked"
	"zeroparse.dev/zp/internal/frontend/schemaexport"
	"zeroparse.dev/zp/internal/runtime/program"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// The tests of this package compile grammars through the compiler side of the driver
// (checked.Source), which the compile entry of the driver (entry.Compile) wraps the same way; the
// entry itself imports this package, so the tests cannot call it.

// Compile compiles the grammar, as entry.Compile does.
func Compile(g *ast.Grammar, opts compile.Options) (*Program, error) {
	s, err := checked.Compile(g, opts, nil)
	if err != nil {
		return nil, err
	}
	return New(s), nil
}

// CompileContext compiles the grammar under ctx, as entry.CompileContext does.
func CompileContext(ctx context.Context, g *ast.Grammar, opts compile.Options) (prog *Program, err error) {
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
	s, err := checked.Compile(g, opts, &cancel.Control{Ctx: ctx})
	if err != nil {
		return nil, err
	}
	return New(s), nil
}

// build builds a program from the grammar without the check of its declarations of linear cost. If
// flags is not nil, it skips static analysis and type checking and uses flags instead.
func build(g *ast.Grammar, opts compile.Options, flags []compile.RuleFlags) (*Program, error) {
	s, err := checked.Check(g, opts, flags, nil)
	if err != nil {
		return nil, err
	}
	return New(s), nil
}

// checkedProgram returns the program that runs the checked program sp.
func checkedProgram(sp *compile.Program) *Program { return New(checked.Of(sp)) }

// Static returns the checked program that p was compiled from, or nil for a program loaded from a
// compiled grammar.
func (p *Program) Static() *compile.Program {
	if s, ok := p.src.(*checked.Source); ok {
		return s.Checked()
	}
	return nil
}

// SourceGrammar returns the grammar as written that p was compiled from, or nil for a program
// loaded from a compiled grammar.
func (p *Program) SourceGrammar() *ast.Grammar {
	if s, ok := p.src.(*checked.Source); ok {
		return s.Grammar()
	}
	return nil
}

// SchemaSource is entry.SchemaSource.
func (p *Program) SchemaSource(start string) (schemaexport.Source, error) {
	if p == nil {
		return schemaexport.Source{}, program.ErrSchemaUnavailable
	}
	if err := program.EntryError(start); err != nil {
		return schemaexport.Source{}, err
	}
	sp := p.Static()
	if sp == nil {
		return schemaexport.Source{}, program.ErrSchemaNeedsSource
	}
	return checked.Of(sp).Schema(), nil
}

// lazyModule returns the bytecode module that p holds: for a program compiled from a grammar, the
// module that its compiler side has built on first use (checked.Source.Module), or nil before that.
func lazyModule(p *Program) *bytecode.Module {
	if sp := p.Static(); sp != nil {
		return sp.Bytecode
	}
	return p.Bytecode
}
