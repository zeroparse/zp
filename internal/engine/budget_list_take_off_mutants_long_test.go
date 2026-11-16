//go:build zp_long && !zp_budget_controls_reference

package engine_test

import "testing"

// TestRepetitionListTakeOffMutants is the negative control of the repetition case of
// TestLimitErrorAgreement on the engine's backends: a closure backend whose Parser.Kids reserves the
// copy and the take-off of a repetition's elements apart, and bytecode backends that do so where a
// repetition ends with its list, no longer make the take-off one reservation of two units for each
// element, and the case fails.
func TestRepetitionListTakeOffMutants(t *testing.T) {
	t.Parallel()
	const tests = "TestLimitErrorAgreement/repetition"
	t.Run("closure", func(t *testing.T) {
		runPrintedMutants(t, "state/alloc.go", []printedMutant{{"Kids reserves the copy and the take-off apart",
			[]printedEdit{{"\tif n > 0 {\n\t\tp.SpendValue(2 * n)\n\t}\n", "\tif n > 0 {\n\t\tp.SpendValue(n)\n\t\tp.SpendValue(n)\n\t}\n"}},
			tests, []string{"."}}})
	})
	t.Run("vm", func(t *testing.T) {
		runPrintedMutants(t, "vm/vm.go", []printedMutant{{"a repetition's list reserves the copy and the take-off apart",
			[]printedEdit{{"\t\t\t\t\tif n > 0 {\n\t\t\t\t\t\tp.SpendValue(2 * n)\n\t\t\t\t\t}\n", "\t\t\t\t\tif n > 0 {\n\t\t\t\t\t\tp.SpendValue(n)\n\t\t\t\t\t\tp.SpendValue(n)\n\t\t\t\t\t}\n"}},
			tests, []string{"."}}})
	})
}
