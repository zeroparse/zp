// Package frontend turns a grammar into its checked form: the compiled program with its types
// and rule flags, the Core IR module built from it, and the facts computed on the module. It is
// the one place that answers what a grammar means; the backends and the tools that analyze a
// grammar (lint, the language server) read its Result.
//
// Run reports the errors the compiler finds. The compiler's Check makes the program; the program
// and the type model still live in the compiler, so the front end imports it until they move to
// packages of their own.
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

// A fact error of the untyped module that the compiler does not report is added to its errors
// by mergeErrors.

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
	prog, err := compile.Check(g, opts, nil, names, control)
	if err != nil {
		// The facts of a grammar that checks have no error (the compiler reports the same ones),
		// so they are looked for only when checking fails.
		return nil, mergeErrors(names.Rewrite(err), structuralErrors(g, opts, names, control))
	}
	return FromProgram(prog, control)
}

// FromProgram builds the Result of a program that was compiled already, for a caller that keeps
// the program for its own use and must not compile the grammar twice.
func FromProgram(prog *compile.Program, control *cancel.Control) (*Result, error) {
	origin := map[core.Node]grammar.Expr{}
	m, err := build.CoreFrom(prog, origin)
	if err != nil {
		return nil, err
	}
	return &Result{Program: prog, Module: m, Origin: origin, Facts: facts.Compute(m, control)}, nil
}
