package compile

import (
	"testing"

	"zeroparse.dev/zp/pkg/grammar/ast"
)

func TestFirstTerminalFacts(t *testing.T) {
	t.Parallel()
	leaf := &ast.Literal{Value: "a"}
	a := &Rule{Rec: true, Def: &ast.RuleDef{Name: "a", Expr: &ast.Ref{Name: "b"}}}
	b := &Rule{Rec: true, Def: &ast.RuleDef{Name: "b", Expr: &ast.Capture{Name: "x", Expr: &ast.Atomic{Expr: &ast.Discard{Expr: &ast.Seq{Items: []ast.Expr{leaf}}}}}}}
	p := &Program{ByName: map[string]*Rule{"a": a, "b": b}}
	got, depth := firstTerminal(p, &ast.Ref{Name: "a"}, 0, map[string]bool{})
	if got != leaf || depth != 2 {
		t.Fatalf("terminal=%v depth=%d", got, depth)
	}
	for _, expr := range []ast.Expr{&ast.Literal{}, &ast.Ref{Name: "missing"}, &ast.Optional{Expr: leaf}} {
		if got, depth := firstTerminal(p, expr, 0, map[string]bool{}); got != nil || depth != 0 {
			t.Fatal("guard crossed an unsupported boundary")
		}
	}
	// Only calls of recursive rules count a level.
	a.Rec, b.Rec = false, false
	if got, depth := firstTerminal(p, &ast.Ref{Name: "a"}, 0, map[string]bool{}); got != leaf || depth != 0 {
		t.Fatalf("terminal=%v depth=%d without recursive rules", got, depth)
	}
	a.Rec, b.Rec = true, true
	b.Def.Expr = &ast.Ref{Name: "a"}
	if got, depth := firstTerminal(p, &ast.Ref{Name: "a"}, 0, map[string]bool{}); got != nil || depth != 0 {
		t.Fatal("cyclic guard did not stop")
	}
}
