package execlower

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
	"zeroparse.dev/zp/internal/pass"
)

// TestLoweringStep checks the conversion as a step of the pipeline: one required step from the Flow
// stage to the Exec stage that returns the function with its module, and whose postcondition, the
// verifier, names the step and the module when it rejects the result.
func TestLoweringStep(t *testing.T) {
	want := []pass.Step{{Name: Name, Required: true, In: "flow", Out: "exec", Dump: "exec"}}
	if got := Lowering.Steps(); !slices.Equal(got, want) {
		t.Errorf("steps = %+v, want %+v", got, want)
	}
	m := flowModule()
	fn := m.Funcs[0]
	fn.Attrs = &flow.Attrs{Vars: []string{"ok"}}
	l, err := Lowering.Run(&pass.Run{}, pass.Label{Table: "rules"}, m)
	if err != nil {
		t.Fatal(err)
	}
	if l.Func != fn || !slices.Equal(l.Vars, []string{"ok"}) || !slices.Contains(l.Module.Funcs, fn) || l.Module.Stage != Stage {
		t.Errorf("lowered = %+v", l)
	}
	// A call of an operation that the Flow module does not declare is not legal after the conversion.
	m = flowModule()
	m.Funcs[0].Attrs = &flow.Attrs{}
	m.Funcs[0].Body.Stmts = append(m.Funcs[0].Body.Stmts, &code.Do{Call: &code.Call{Callee: "flow.undeclared"}})
	_, err = Lowering.Run(&pass.Run{}, pass.Label{Table: "rules"}, m)
	var se *pass.StepError
	if !errors.As(err, &se) || se.Step.Name != Name || !strings.HasPrefix(err.Error(), "lower t: ") {
		t.Errorf("an invalid result: %v", err)
	}
}
