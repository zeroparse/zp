// Package entry is the compile entry of the driver: it compiles a grammar to a program that the
// runtime runs (engine.Program), with the checked program on the compiler side of it
// (checked.Source), and gives the tools that read the checked program (the generators, the output
// schema, the language server) access to it.
package entry

import (
	"context"

	"zeroparse.dev/zp/internal/frontend/parser"

	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/driver/checked"
	"zeroparse.dev/zp/internal/frontend/schemaexport"
	"zeroparse.dev/zp/internal/runtime/engine"
	"zeroparse.dev/zp/internal/runtime/program"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// Options configures compilation.
type Options = compile.Options

// A checked source exports the output schemas of the parsers of its programs.
var _ engine.SchemaSource = (*checked.Source)(nil)

// Compile compiles the grammar.
func Compile(g *ast.Grammar, opts Options) (*engine.Program, error) {
	s, err := checked.Compile(g, opts, nil)
	if err != nil {
		return nil, err
	}
	return engine.New(s), nil
}

// CompileParser compiles g and returns a parser that starts at the rule start, which must be a
// start rule; an empty start selects the default start rule.
func CompileParser(g *ast.Grammar, start string, opts Options) (*engine.Parser, error) {
	prog, err := Compile(g, opts)
	if err != nil {
		return nil, err
	}
	return engine.NewParser(prog, start)
}

// CompileSource parses the grammar source src and compiles it with CompileParser.
func CompileSource(src, start string, opts Options) (*engine.Parser, error) {
	g, err := parser.Parse(src)
	if err != nil {
		return nil, err
	}
	return CompileParser(g, start, opts)
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
func CompileContext(ctx context.Context, g *ast.Grammar, opts Options) (prog *engine.Program, err error) {
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
	return engine.New(s), nil
}

// Source returns the compiler side of p, or nil for a program loaded from a compiled grammar.
func Source(p *engine.Program) *checked.Source {
	s, _ := p.Source().(*checked.Source)
	return s
}

// Checked returns the checked program that p was compiled from, or nil for a program loaded from a
// compiled grammar.
func Checked(p *engine.Program) *compile.Program {
	if s := Source(p); s != nil {
		return s.Checked()
	}
	return nil
}

// Grammar returns the grammar that p was compiled from, as written, or nil for a program loaded
// from a compiled grammar.
func Grammar(p *engine.Program) *ast.Grammar {
	if s := Source(p); s != nil {
		return s.Grammar()
	}
	return nil
}

// SchemaSource returns the checked grammar that an output schema of the start rule is exported
// from, after checking that the export can run: p must not be nil, start must be able to start a
// parse, and p must hold its grammar, which a program loaded from a compiled grammar does not.
// The errors of a missing program or grammar wrap program.ErrSchemaUnavailable.
func SchemaSource(p *engine.Program, start string) (schemaexport.Source, error) {
	if p == nil {
		return schemaexport.Source{}, program.ErrSchemaUnavailable
	}
	if err := program.EntryError(start); err != nil {
		return schemaexport.Source{}, err
	}
	s := Source(p)
	if s == nil {
		return schemaexport.Source{}, program.ErrSchemaNeedsSource
	}
	return s.Schema(), nil
}
