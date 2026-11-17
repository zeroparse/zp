package gogen_test

import (
	"math"
	"testing"

	"zeroparse.dev/zp/internal/testing/codegentest"
	"zeroparse.dev/zp/internal/testing/complexitytest"
)

// TestRegionBoundsGenerated measures the controls of the region bound in generated parsers, which
// keep the runs of scan sites but no records of run sites: a control whose order is linear spends a
// budget linear in the input there, as the positive control and the one whose growth restarts a
// scan site do, and the control that only the records of run sites make linear a quadratic budget.
func TestRegionBoundsGenerated(t *testing.T) {
	t.Parallel()
	goBin := skipRunsGoBin(t)
	codegentest.NeedsBudgetControls(t)
	var controls []complexitytest.Control
	for _, c := range complexitytest.Controls() {
		if c.Family == "region bounds" {
			controls = append(controls, c)
		}
	}
	if len(controls) != 3 {
		t.Fatalf("%d region bound controls, want 3", len(controls))
	}
	sizes := []int{200, 400, 800}
	spends, _ := skipRunSpends(t, goBin, controls, sizes, nil, "")
	t.Logf("spends at %v: %v", sizes, spends)
	for i, row := range spends {
		lo, hi := math.Inf(1), math.Inf(-1)
		for j := 1; j < len(row); j++ {
			s := math.Log2(float64(row[j]) / float64(row[j-1]))
			lo, hi = min(lo, s), max(hi, s)
		}
		// The control that is not closed is linear in the engine only, through the records of
		// run sites.
		quadratic := controls[i].Order != complexitytest.Linear || controls[i].Name == "Comments that are not closed after tokens"
		switch {
		case !quadratic && hi > 1.2:
			t.Errorf("%s: spends %v grow faster than linearly", controls[i].Name, row)
		case quadratic && lo < 1.7:
			t.Errorf("%s: spends %v are not quadratic", controls[i].Name, row)
		}
	}
}
