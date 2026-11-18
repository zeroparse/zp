package print

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
)

// markInIfFunc returns a framed function whose mark of the parser state is set in the body of an If
// and reset there (resetInIf) or by a later If, with the position and a capture changed in between.
// The lowering of a rule makes the mark at the level of the scope that backtracks, never inside an If
// today; the printer must still print such a mark if the lowering ever does.
func markInIfFunc(t *testing.T, resetInIf bool) *code.Module {
	t.Helper()
	do := func(name string, imm any, args ...code.Expr) code.Stmt {
		return &code.Do{Call: &code.Call{Callee: name, Imm: imm, Args: args}}
	}
	cond := func() code.Expr { return code.Bin(">=", code.R(flow.Pos), code.Int(3)) }
	reset := func() code.Stmt { return do(flow.OpReset, nil, code.L("x1")) }
	then := []code.Stmt{
		code.Assign("x1", &code.Call{Callee: flow.OpMarkState, Imm: flow.StateMark{Trail: true, Env: true}}),
		&code.Set{Dst: []code.Place{{Name: flow.Pos, Reg: true}}, Values: []code.Expr{code.Int(7)}},
		do(flow.OpSetCapture, flow.Slot(0), code.Null(code.THandle(flow.TValue))),
	}
	var body []code.Stmt
	if resetInIf {
		then = append(then, reset())
		body = append(body, &code.If{Cond: cond(), Then: &code.Block{Stmts: then}})
	} else {
		body = append(body, &code.If{Cond: cond(), Then: &code.Block{Stmts: then}},
			&code.If{Cond: cond(), Then: &code.Block{Stmts: []code.Stmt{reset()}}})
	}
	return framed(t, "markInIf", []code.Param{code.P("x1", code.THandle(flow.TMark))}, body...)
}

// TestFlowExecPrintsMarkInIfBody checks that the mark of a framed function that is set in the body of
// an If prints its parts that a Reset reads, and the Reset, in generated TypeScript.
func TestFlowExecPrintsMarkInIfBody(t *testing.T) {
	t.Parallel()
	for name, resetInIf := range map[string]bool{"reset in the If": true, "reset after the If": false} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			g := &Generator{}
			text, decls, _ := g.printTSFlowExec(markInIfFunc(t, resetInIf))
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
	f := markInIfFunc(t, true)
	// The body of the function is its block of failure; the If is its first statement.
	then := flow.FuncOf(f).Body.Stmts[0].(*code.Block).Stmts[0].(*code.If).Then
	then.Stmts = then.Stmts[:len(then.Stmts)-1] // without the Reset
	text, decls, _ := (&Generator{}).printTSFlowExec(f)
	if strings.Contains(text, "x1") || strings.Contains(strings.Join(decls, ""), "x1") {
		t.Errorf("an unread mark prints:\n%s%q", text, decls)
	}
}
