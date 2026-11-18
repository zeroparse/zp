package tsgen

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
	"zeroparse.dev/zp/internal/middleend/flowlower"
)

// TestFlowExecPrintsStepRunAsUnits checks that charges in a row, which the printer joins into one
// statement, are charged as that many one-unit reservations: a run whose units do not all fit fails at
// the unit that does not, as the charges in a row would, and reports one unit requested.
func TestFlowExecPrintsStepRunAsUnits(t *testing.T) {
	t.Parallel()
	f := &flow.Func{Name: "steps", Fail: &flow.Label{Name: "fail"}, Frame: true, Body: []flow.Stmt{&flow.Step{}, &flow.Step{}, &flow.Step{}}}
	text, _, _ := (&Generator{}).printTSFlowExec(emitted(t, f))
	if !strings.Contains(text, "p.spendUnits(3);") || strings.Contains(text, "p.spend(3)") {
		t.Errorf("three charges in a row do not print as a run of three units:\n%s", text)
	}
	if !strings.Contains(readLib(t, tsLibFileName), "spendUnits(n: number): void { this.control?.chargeUnits(n); }") {
		t.Error("the library's spendUnits does not charge a run of units")
	}
}

// emitted returns the lowered function f at the Flow stage of Code IR.
func emitted(t *testing.T, f *flow.Func) *code.Module {
	t.Helper()
	m, _, err := flowlower.Emit(f)
	if err != nil {
		t.Fatal(err)
	}
	return m
}
