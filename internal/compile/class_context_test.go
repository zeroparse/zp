package compile

import (
	"testing"

	"zeroparse.dev/zp/grammar"
)

func TestFirstTerminalFacts(t *testing.T) {
	t.Parallel()
	leaf := &grammar.Literal{Value: "a"}
	a := &Rule{Rec: true, Def: &grammar.RuleDef{Name: "a", Expr: &grammar.Ref{Name: "b"}}}
	b := &Rule{Rec: true, Def: &grammar.RuleDef{Name: "b", Expr: &grammar.Capture{Name: "x", Expr: &grammar.Atomic{Expr: &grammar.Discard{Expr: &grammar.Seq{Items: []grammar.Expr{leaf}}}}}}}
	p := &Program{ByName: map[string]*Rule{"a": a, "b": b}}
	got, depth := firstTerminal(p, &grammar.Ref{Name: "a"}, 0, map[string]bool{})
	if got != leaf || depth != 2 {
		t.Fatalf("terminal=%v depth=%d", got, depth)
	}
	for _, expr := range []grammar.Expr{&grammar.Literal{}, &grammar.Ref{Name: "missing"}, &grammar.Optional{Expr: leaf}} {
		if got, depth := firstTerminal(p, expr, 0, map[string]bool{}); got != nil || depth != 0 {
			t.Fatal("guard crossed an unsupported boundary")
		}
	}
	// Only calls of recursive rules count a level.
	a.Rec, b.Rec = false, false
	if got, depth := firstTerminal(p, &grammar.Ref{Name: "a"}, 0, map[string]bool{}); got != leaf || depth != 0 {
		t.Fatalf("terminal=%v depth=%d without recursive rules", got, depth)
	}
	a.Rec, b.Rec = true, true
	b.Def.Expr = &grammar.Ref{Name: "a"}
	if got, depth := firstTerminal(p, &grammar.Ref{Name: "a"}, 0, map[string]bool{}); got != nil || depth != 0 {
		t.Fatal("cyclic guard did not stop")
	}
}
