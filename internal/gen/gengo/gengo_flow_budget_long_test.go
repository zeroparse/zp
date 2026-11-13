//go:build zp_long

package gengo_test

import (
	"bytes"
	"testing"

	"zeroparse.dev/zp/internal/gentest"
)

// TestFlowNodeBudgetMutants checks that the invariants and the golden of TestFlowNodeBudgetInvariants notice a
// printer that loses the charge of one Step: with the first, and with the last, step charge of each parser
// removed, some report violates an invariant or differs from the golden.
func TestFlowNodeBudgetMutants(t *testing.T) {
	t.Parallel()
	gentest.NeedsBudgetControls(t)
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	cases := gentest.GenCorpus(t)
	golden := gentest.ReadGolden(t, budgetGolden)
	for _, m := range []struct {
		name   string
		mutate func([]byte) []byte
	}{{"drop the first step charge", dropFirstStep}, {"drop the last step charge", dropLastStep}} {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			mutated := 0
			got, problems := budgetTotals(t, "corpus", cases, "main", func(code []byte) []byte {
				out := m.mutate(code)
				if !bytes.Equal(out, code) {
					mutated++
				}
				return out
			})
			if mutated == 0 {
				t.Fatal("the mutant changes no generated parser")
			}
			differ := 0
			for k, v := range got {
				if w, ok := golden[k]; !ok || w != v {
					differ++
				}
			}
			if len(problems)+differ == 0 {
				t.Errorf("the mutant %q changes no report and breaks no invariant", m.name)
			}
			t.Logf("%d parsers mutated: %d reports differ from the golden, %d invariants violated", mutated, differ, len(problems))
		})
	}
}
