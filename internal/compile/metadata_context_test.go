package compile

import (
	"context"
	"errors"
	"testing"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/check"

	"zeroparse.dev/zp/grammar"
)

func TestCompileMetadataCollectionCancellation(t *testing.T) {
	g := &grammar.Grammar{}
	a := &grammar.Attribute{Name: "error"}
	for range 1024 {
		g.File.Decls = append(g.File.Decls, &grammar.TypeDef{Name: "T"}, &grammar.RuleDef{Name: "a"})
		a.Args = append(a.Args, "label")
	}
	expr := &grammar.Attributed{Expr: &grammar.Literal{Value: "a"}, Attrs: []*grammar.Attribute{a}}
	for _, phase := range []struct {
		name string
		run  func(*cancel.Control)
	}{
		{"rules", func(c *cancel.Control) { c.Rules(g) }},
		{"types", func(c *cancel.Control) { c.Types(g) }},
		{"arguments", func(c *cancel.Control) { expressionArgs(c, a) }},
		{"call-walk", func(c *cancel.Control) { allCallsControlled(expr, nil, c) }},
		{"type-check-arguments", func(c *cancel.Control) {
			check.PollPredicates(c, expr)
		}},
		{"stream-walk", func(c *cancel.Control) { (&Compiler{Control: c}).checkStream(expr) }},
	} {
		t.Run(phase.name, func(t *testing.T) {
			control := &cancel.Control{Ctx: compileTestContext(t, 2)}
			defer func() {
				failure := recover()
				stopped, ok := failure.(cancel.Cancellation)
				if !ok || !errors.Is(stopped.Err, context.Canceled) {
					t.Fatalf("collection did not unwind as compilation cancellation: %v", failure)
				}
			}()
			phase.run(control)
		})
	}
}

func TestProjectionMetadataCancellation(t *testing.T) {
	var expr grammar.Expr = &grammar.Literal{Value: "a"}
	var term grammar.Term = &grammar.StringLit{Value: "a"}
	for range 1024 {
		expr = &grammar.Optional{Expr: expr}
		term = &grammar.Member{Name: "field", X: term}
	}
	for _, phase := range []struct {
		name string
		run  func(*cancel.Control)
	}{
		{"candidate-walk", func(c *cancel.Control) {
			ruleProjectionsControlled(&Program{}, &Rule{Def: &grammar.RuleDef{Expr: expr}, Action: &grammar.StringLit{}}, c)
		}},
		{"action-use", func(c *cancel.Control) { markProjectedMaps(term, nil, map[*grammar.Call]bool{}, c) }},
	} {
		t.Run(phase.name, func(t *testing.T) {
			control := &cancel.Control{Ctx: compileTestContext(t, 1)}
			defer func() {
				failure := recover()
				stopped, ok := failure.(cancel.Cancellation)
				if !ok || !errors.Is(stopped.Err, context.Canceled) {
					t.Fatalf("projection metadata did not stop: %v", failure)
				}
			}()
			phase.run(control)
		})
	}
}

func TestTypeCheckEmptyAttributeCancellation(t *testing.T) {
	attrs := make([]*grammar.Attribute, 1024)
	for i := range attrs {
		attrs[i] = &grammar.Attribute{Name: "error"}
	}
	control := &cancel.Control{Ctx: compileTestContext(t, 1)}
	defer func() {
		failure := recover()
		stopped, ok := failure.(cancel.Cancellation)
		if !ok || !errors.Is(stopped.Err, context.Canceled) {
			t.Fatalf("empty attribute rows did not stop: %v", failure)
		}
	}()
	check.PollPredicates(control, &grammar.Attributed{Expr: &grammar.Literal{Value: "a"}, Attrs: attrs})
}
