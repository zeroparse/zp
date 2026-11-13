package gogen_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/gentest"
)

// TestGeneratedCallProtocolMatchesEngine compares the results and errors of node and typed parsers
// whose rules reuse memo entries in lookaheads, attributes, recovery and growth with the engine's.
func TestGeneratedCallProtocolMatchesEngine(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds generated code")
	}
	gentest.PinOrdinary(t)
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	gentest.GeneratedParsersCorpus(t, goBin, gentest.CallProtocolCases(), codegen.GenOptions{})
	gentest.GeneratedTypesCorpus(t, goBin, gentest.CallProtocolCases(), codegen.GenOptions{})
}

// TestCallProtocolBudget checks the budget totals, polls and results of the call protocol cases with
// budget controls against testdata/budget.golden: a composition that loses a charge, a growth step or a
// restore of the parser state changes a total or a result.
func TestCallProtocolBudget(t *testing.T) {
	t.Parallel()
	gentest.NeedsBudgetControls(t)
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	cases := gentest.CallProtocolCases()
	got, problems := budgetTotals(t, "call protocol", cases, "main", nil)
	for _, p := range problems {
		t.Error(p)
	}
	budgetGoldenFile.Check(t, "call protocol/", got)
}
