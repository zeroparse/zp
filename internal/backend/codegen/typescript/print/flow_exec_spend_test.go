package print

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
)

// TestFlowExecPrintsStepRunAsUnits checks that charges in a row, which the printer joins into one
// statement, are charged as that many one-unit reservations: a run whose units do not all fit fails at
// the unit that does not, as the charges in a row would, and reports one unit requested.
func TestFlowExecPrintsStepRunAsUnits(t *testing.T) {
	t.Parallel()
	f := framed(t, "steps", nil, &code.Do{Call: &code.Call{Callee: flow.OpStep}}, &code.Do{Call: &code.Call{Callee: flow.OpStep}}, &code.Do{Call: &code.Call{Callee: flow.OpStep}})
	text, _, _ := (&Generator{}).printTSFlowExec(f)
	if !strings.Contains(text, "p.spendUnits(3);") || strings.Contains(text, "p.spend(3)") {
		t.Errorf("three charges in a row do not print as a run of three units:\n%s", text)
	}
	if !strings.Contains(readLib(t, tsLibFileName), "spendUnits(n: number): void { this.control?.chargeUnits(n); }") {
		t.Error("the library's spendUnits does not charge a run of units")
	}
}

// framed returns a module at the Flow stage of Code IR of one framed function name with the
// variables vars and the body, as the lowering writes it: the body is the block of failure, which ends
// by returning the null value and true, and the function returns the null value and false after it.
func framed(t *testing.T, name string, vars []code.Param, body ...code.Stmt) *code.Module {
	t.Helper()
	value := code.THandle(flow.TValue)
	at := &flow.Attrs{Frame: true, Fail: "fail"}
	for _, v := range vars {
		at.Vars = append(at.Vars, v.Name)
	}
	fail := &code.Block{Label: at.Fail, Stmts: append(body, &code.Return{Values: []code.Expr{code.Null(value), code.BoolC(true)}})}
	m := flow.NewModule("flow " + name)
	m.Funcs = []*code.Func{{Name: name, Locals: vars, Results: []code.Type{value, code.TBool}, Attrs: at,
		Body: &code.Block{Stmts: []code.Stmt{fail, &code.Return{Values: []code.Expr{code.Null(value), code.BoolC(false)}}}}}}
	if err := flow.VerifyModule(m); err != nil {
		t.Fatal(err)
	}
	return m
}
