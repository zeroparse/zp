package execlower

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
	"zeroparse.dev/zp/internal/ir/core"
)

// flowModule returns a module at the Flow stage whose function matches a literal and advances.
func flowModule() *code.Module {
	m := flow.NewModule("t")
	ok := "ok"
	m.Funcs = append(m.Funcs, &code.Func{Name: "f", Locals: []code.Param{code.P(ok, code.TBool)}, Body: &code.Block{Label: "fail", Stmts: []code.Stmt{
		&code.Do{Call: &code.Call{Callee: flow.OpStep}},
		&code.Set{Dst: []code.Place{{}, {Name: ok}}, Values: []code.Expr{&code.Call{Callee: flow.OpMatchLit, Imm: &core.Lit{Text: "a"}}}},
		&code.If{Cond: &code.Unary{Op: "!", X: code.L(ok)}, Then: &code.Block{Stmts: []code.Stmt{&code.Exit{Label: "fail"}}}},
		&code.Do{Call: &code.Call{Callee: flow.OpAdvance, Args: []code.Expr{code.Int(1)}}},
	}}})
	return m
}

// TestLowerConvertsEveryFlowCall checks that the conversion leaves no call of a Flow operation: each
// becomes the primitive of the Exec stage that stands for it, with the same arguments and immediate.
func TestLowerConvertsEveryFlowCall(t *testing.T) {
	m := flowModule()
	if err := code.Verify(m); err != nil {
		t.Fatal(err)
	}
	x, err := Lower(m)
	if err != nil {
		t.Fatal(err)
	}
	if x.Stage != Stage {
		t.Errorf("stage = %q, want %q", x.Stage, Stage)
	}
	var calls []string
	code.Walk(x.Funcs[0].Body, nil, func(e code.Expr) {
		if c, ok := e.(*code.Call); ok {
			calls = append(calls, c.Callee)
			if FlowOp(c.Callee) == "" {
				t.Errorf("call of %s, which is not a primitive", c.Callee)
			}
			if o := x.Op(c.Callee); o == nil || o.Kind != code.OpPrimitive || o.Dialect != Dialect {
				t.Errorf("operation %s is %+v, want a primitive of the Exec dialect", c.Callee, o)
			}
		}
	})
	if got, want := strings.Join(calls, " "), "runtime.step runtime.matchLit runtime.advance"; got != want {
		t.Errorf("calls = %s, want %s", got, want)
	}
	for _, o := range x.Ops {
		if o.Dialect == flow.Dialect {
			t.Errorf("the Exec module declares the Flow operation %s", o.Name)
		}
	}
}

// TestLowerPostcondition is the negative control of the conversion's postcondition: a call of a Flow
// operation left at the Exec stage, declared there, fails verification.
func TestLowerPostcondition(t *testing.T) {
	x, err := Lower(flowModule())
	if err != nil {
		t.Fatal(err)
	}
	f := x.Funcs[0]
	f.Body.Stmts = append(f.Body.Stmts, &code.Do{Call: &code.Call{Callee: flow.OpStep}})
	x.AddOp(flow.Ops()[0])
	err = code.Verify(x)
	if err == nil || !strings.Contains(err.Error(), "not legal at the exec stage") {
		t.Errorf("a Flow call left at the Exec stage verifies: %v", err)
	}
	if _, err := Lower(x); err == nil {
		t.Error("Lower accepts a module that is not at the Flow stage")
	}
}
