// Package frontend turns a grammar into its checked form: the compiled program with its types
// and rule flags, the Core IR module built from it, and the facts computed on the module. It is
// the one place that answers what a grammar means; the backends and the tools that analyze a
// grammar (lint, the language server) read its Result.
//
// Run reports the errors the compiler finds. Until the compiler is only the lowering of a
// Result, Run depends on it for the program (the front end sits above the compiler, and this
// edge is removed with that change).
package frontend

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/desugar"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/ir/core/build"
	"zeroparse.dev/zp/internal/ir/core/facts"
)

// Result is the checked form of a grammar.
type Result struct {
	// Program is the compiled program: rules, flags and types.
	Program *compile.Program
	// Module is the Core IR of the program.
	Module *core.Module
	// Origin maps the nodes of Module to the expressions they were built from.
	Origin map[core.Node]grammar.Expr
	// Facts are the grammar facts of Module.
	Facts *facts.Facts
}

// Run checks g with the compiler's options opts. A non-nil control polls for cancellation. The
// error is the compiler's error list, with the names of the hidden rules that inline actions
// generate shown as written.
func Run(g *grammar.Grammar, opts compile.Options, control *cancel.Control) (*Result, error) {
	names := desugar.InlineNames{}
	prog, err := compile.CompileProgram(g, opts, nil, names, control, func(*compile.Compiler) compile.ExprBuilder[struct{}] { return compile.NoExprs{} })
	if err != nil {
		return nil, names.Rewrite(err)
	}
	origin := map[core.Node]grammar.Expr{}
	m, err := build.CoreFrom(prog, origin)
	if err != nil {
		return nil, err
	}
	return &Result{Program: prog, Module: m, Origin: origin, Facts: facts.Compute(m, control)}, nil
}
