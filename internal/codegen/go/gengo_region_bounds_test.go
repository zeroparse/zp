package gogen_test

import (
	"math"
	"testing"

	"zeroparse.dev/zp/internal/codegentest"
	"zeroparse.dev/zp/internal/complexitytest"
)

// TestRegionBoundsGenerated measures the controls of the region bound in generated parsers, which
// keep no records of run sites or scan sites: the positive control spends a budget linear in the
// input, and the negative ones a quadratic budget.
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
		switch {
		case controls[i].Positive && hi > 1.2:
			t.Errorf("%s: spends %v grow faster than linearly", controls[i].Name, row)
		case !controls[i].Positive && lo < 1.7:
			t.Errorf("%s: spends %v are not quadratic", controls[i].Name, row)
		}
	}
}
