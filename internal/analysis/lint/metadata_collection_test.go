package lint

import (
	"context"
	"errors"
	"testing"

	"zeroparse.dev/zp/pkg/grammar/ast"
)

func TestLintMetadataCollectionCancellation(t *testing.T) {
	g := &ast.Grammar{File: ast.File{EndBreak: &ast.LineBreak{}}}
	a := &ast.Attribute{Name: "error"}
	var rules []*ast.RuleDef
	for range 1024 {
		r := &ast.RuleDef{Name: "a", Expr: &ast.Literal{Value: "a"}}
		rules = append(rules, r)
		g.File.Decls = append(g.File.Decls, &ast.TypeDef{Name: "T"}, r)
		g.File.EndBreak.Comments = append(g.File.EndBreak.Comments, &ast.Comment{})
		a.Args = append(a.Args, "label")
	}
	expr := &ast.Attributed{Expr: &ast.Literal{Value: "a"}, Attrs: []*ast.Attribute{a}}
	for _, phase := range []struct {
		name     string
		run      func(*lintControl)
		cancelAt int
	}{
		{"rules", func(c *lintControl) { c.rules(g) }, 2},
		{"types", func(c *lintControl) { c.types(g) }, 2},
		{"comments", func(c *lintControl) { c.comments(g) }, 2},
		{"arguments", func(c *lintControl) { c.expressionArgs(a) }, 2},
		{"call-walk", func(c *lintControl) { callsControlled(expr, c) }, 2},
		{"unused-capture-rules", func(c *lintControl) { (&linter{control: c, an: &analysis{g: g, order: rules}}).unusedCaptures() }, 2},
		// With no statements, two Err calls finish rule enumeration; the next
		// collection must stop inside the large comment copy, before reporting.
		{"suppression-comments", func(c *lintControl) {
			(&linter{control: c}).suppress(&ast.Grammar{File: ast.File{EndBreak: g.File.EndBreak}})
		}, 4},
	} {
		t.Run(phase.name, func(t *testing.T) {
			control := &lintControl{ctx: lintTestContext(t, phase.cancelAt)}
			defer func() {
				failure := recover()
				stopped, ok := failure.(lintCancellation)
				if !ok || !errors.Is(stopped.err, context.Canceled) {
					t.Fatalf("collection did not unwind as lint cancellation: %v", failure)
				}
			}()
			phase.run(control)
		})
	}
}
