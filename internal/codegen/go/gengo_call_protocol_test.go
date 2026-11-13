package gogen_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

// TestGeneratedCallProtocolMatchesEngine compares the results and errors of node and typed parsers
// whose rules reuse memo entries in lookaheads, attributes, recovery and growth with the engine's.
func TestGeneratedCallProtocolMatchesEngine(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds generated code")
	}
	codegentest.PinOrdinary(t)
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	codegentest.GeneratedParsersCorpus(t, goBin, codegentest.CallProtocolCases(), codegen.GenOptions{})
	codegentest.GeneratedTypesCorpus(t, goBin, codegentest.CallProtocolCases(), codegen.GenOptions{})
}

// TestCallProtocolBudget checks the budget totals, polls and results of the call protocol cases with
// budget controls against testdata/budget.golden: a composition that loses a charge, a growth step or a
// restore of the parser state changes a total or a result.
func TestCallProtocolBudget(t *testing.T) {
	t.Parallel()
	codegentest.NeedsBudgetControls(t)
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	cases := codegentest.CallProtocolCases()
	got, problems := budgetTotals(t, "call protocol", cases, "main", nil)
	for _, p := range problems {
		t.Error(p)
	}
	budgetGoldenFile.Check(t, "call protocol/", got)
}
