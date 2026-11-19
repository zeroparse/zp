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
	want := []pass.Step{{Name: Name, Required: true, In: "flow", Out: "exec", Dump: "execlower"}}
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

// lowered returns a lowered function with the variable ok.
func lowered(t *testing.T) *Lowered {
	t.Helper()
	m := flowModule()
	m.Funcs[0].Attrs = &flow.Attrs{Vars: []string{"ok"}}
	l, err := LowerFunc(m)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// TestLoweredCopies checks that a copy of a lowered function holds the copy of its function, that
// a module the passes made is taken back only if it keeps the function and its variables, and that
// neither changes the lowered function.
func TestLoweredCopies(t *testing.T) {
	l := lowered(t)
	c := l.Clone()
	if c.Module == l.Module || c.Func == l.Func || !slices.Contains(c.Module.Funcs, c.Func) || c.Func.Name != "f" {
		t.Fatalf("copy %+v", c)
	}
	if err := IR.Verify(c); err != nil {
		t.Fatal(err)
	}
	m := l.Module.Clone()
	w, err := l.WithModule(m)
	if err != nil || w.Func != m.Funcs[0] || w.Module != m || l.Module == m {
		t.Fatalf("WithModule = %+v, %v", w, err)
	}
	removed := l.Module.Clone()
	removed.Funcs = nil
	if _, err := l.WithModule(removed); err == nil || !strings.Contains(err.Error(), "replaced or removed the function f") {
		t.Errorf("a module without the function: %v", err)
	}
	dropped := l.Module.Clone()
	dropped.Funcs[0].Locals = nil
	if _, err := l.WithModule(dropped); err == nil || !strings.Contains(err.Error(), "removed the local ok") {
		t.Errorf("a function without the variable: %v", err)
	}
	c.Module.Funcs = append([]*code.Func{{Name: "g", Body: &code.Block{}}}, c.Module.Funcs[1:]...)
	if err := IR.Verify(c); err == nil || !strings.Contains(err.Error(), "replaced or removed the function f") {
		t.Errorf("the verifier of a module without the function: %v", err)
	}
	if err := IR.Verify(l); err != nil || len(l.Func.Locals) == 0 {
		t.Errorf("changing the copies changed the function: %v", err)
	}
}
