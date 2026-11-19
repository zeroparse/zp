package flowlower

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/pass"
)

// TestLoweringStep checks the lowering as a step of the pipeline: one required step from the Core
// stage to the Flow stage, whose result is the function that Lower makes, and whose failure names it.
func TestLoweringStep(t *testing.T) {
	want := []pass.Step{{Name: Name, Required: true, In: "core", Out: "flow", Dump: "flowlower"}}
	if got := Lowering.Steps(); !slices.Equal(got, want) {
		t.Errorf("steps = %+v, want %+v", got, want)
	}
	m, r := valueFree(alts(&core.Lit{Text: "ab"}, &core.Lit{Text: "c"}))
	old, err := Lower(m, r, "r", Options{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := Lowering.Run(&pass.Run{}, pass.Label{Table: "rules"}, Unit{Module: m, Func: Func{Form: Flat, Rule: r, Name: "r"}})
	if err != nil {
		t.Fatal(err)
	}
	if a, b := code.FuncStringIn(got, got.Funcs[0]), code.FuncStringIn(old, old.Funcs[0]); a != b {
		t.Errorf("the step lowers\n%s\nwhere Lower lowers\n%s", a, b)
	}
	// A unit that the lowering refuses: a local function of a left-recursion leader.
	r.Policy.Leader = true
	_, err = Lowering.Run(&pass.Run{}, pass.Label{Table: "rules"}, Unit{Module: m, Func: Func{Form: Flat, Rule: r, Name: "r"}})
	var se *pass.StepError
	if !errors.As(err, &se) || se.Step.Name != Name || !strings.Contains(err.Error(), "is not lowered to a local function") {
		t.Errorf("a refused unit: %v", err)
	}
}
