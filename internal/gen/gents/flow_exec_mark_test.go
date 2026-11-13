package gents

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/flow"
)

// markInIfFunc returns a framed function whose mark of the parser state is set in the body of an If
// and reset there (resetInIf) or by a later If, with the position and a capture changed in between.
// The lowering of a rule makes the mark at the level of the scope that backtracks, never inside an If
// today; the printer must still print such a mark if the lowering ever does.
func markInIfFunc(resetInIf bool) *flow.Func {
	m := &flow.Var{Name: "x1", Type: flow.TypeMark}
	fail := &flow.Label{Name: "fail"}
	cond := &flow.Cmp{Op: ">=", L: &flow.Read{R: flow.RegPos}, R: &flow.Int{N: 3}}
	change := []flow.Stmt{
		&flow.Set{Dst: []flow.LValue{flow.RegPos}, X: []flow.Expr{&flow.Int{N: 7}}},
		&flow.SetCapture{Slot: 0, X: &flow.Nil{}},
	}
	then := []flow.Stmt{&flow.Set{Dst: []flow.LValue{m}, X: []flow.Expr{&flow.MarkState{Trail: true, Env: true}}}}
	then = append(then, change...)
	body := []flow.Stmt{}
	if resetInIf {
		then = append(then, &flow.Reset{M: &flow.Use{V: m}})
		body = append(body, &flow.If{Cond: cond, Then: then})
	} else {
		body = append(body, &flow.If{Cond: cond, Then: then},
			&flow.If{Cond: cond, Then: []flow.Stmt{&flow.Reset{M: &flow.Use{V: m}}}})
	}
	return &flow.Func{Name: "markInIf", Vars: []*flow.Var{m}, Body: body, Fail: fail, Frame: true}
}

// TestFlowExecPrintsMarkInIfBody checks that the mark of a framed function that is set in the body of
// an If prints its parts that a Reset reads, and the Reset, in generated TypeScript.
func TestFlowExecPrintsMarkInIfBody(t *testing.T) {
	t.Parallel()
	for name, resetInIf := range map[string]bool{"reset in the If": true, "reset after the If": false} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			g := &Generator{}
			text, decls, _ := g.printTSFlowExec(markInIfFunc(resetInIf))
			for _, want := range []string{"x1Pos = p.pos", "x1Env = p.env", "x1Trail = p.trail.length", "p.reset(x1Pos, x1Env, x1Trail,"} {
				if !strings.Contains(text, want) {
					t.Errorf("the printed body has no %q:\n%s", want, text)
				}
			}
			for _, want := range []string{"let x1Pos", "let x1Env", "let x1Trail"} {
				if !strings.Contains(strings.Join(decls, ""), want) {
					t.Errorf("the declarations have no %q: %q", want, decls)
				}
			}
		})
	}
}

// TestFlowExecOmitsUnreadMarkInIfBody is the negative control: a mark set in the body of an If that no
// Reset reads prints nothing, so the assertions above do not pass on any text.
func TestFlowExecOmitsUnreadMarkInIfBody(t *testing.T) {
	t.Parallel()
	f := markInIfFunc(true)
	then := f.Body[0].(*flow.If).Then
	f.Body[0].(*flow.If).Then = then[:len(then)-1] // without the Reset
	text, decls, _ := (&Generator{}).printTSFlowExec(f)
	if strings.Contains(text, "x1") || strings.Contains(strings.Join(decls, ""), "x1") {
		t.Errorf("an unread mark prints:\n%s%q", text, decls)
	}
}
