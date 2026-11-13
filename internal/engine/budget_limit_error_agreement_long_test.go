//go:build zp_long && !zp_budget_controls_reference

package engine

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gentest"
)

// TestLimitErrorAgreementMutants checks that TestLimitErrorAgreement notices a backend that reports a
// run of units as one reservation: generated Go whose Take reserves its units at once, and generated
// TypeScript whose run of units is charged as one reservation.
func TestLimitErrorAgreementMutants(t *testing.T) {
	t.Parallel()
	gentest.NeedsBudgetControls(t)
	cases := map[string]limitAgreementCase{}
	for _, c := range limitAgreementCases {
		cases[c.name] = c
	}
	t.Run("atomic Take", func(t *testing.T) {
		t.Parallel()
		gentest.NeedsBudgetControls(t)
		const unit = "func (c *BudgetControl) takeSlow(n uint64) {\n\tfor ; n > 0; n-- {\n\t\tc.chargeStep()\n\t}\n}"
		runtime := gentest.LibCopyWith(t, func(name, src string) string {
			if name != "budget.go" {
				return src
			}
			if !strings.Contains(src, unit) {
				t.Fatal("the unit reservations of Take are not in budget.go")
			}
			return strings.Replace(src, unit, "func (c *BudgetControl) takeSlow(n uint64) {\n\tc.chargeSlow(n)\n}", 1)
		}, nil)
		c := cases["literal"]
		want := engineLimitReports(t, c, Closure)
		node, typed := generatedGoLimitReports(t, c, runtime)
		if len(limitDisagreements(want, node, c.window)) == 0 || len(limitDisagreements(want, typed, c.window)) == 0 {
			t.Error("generated Go with an atomic Take agrees with the engine")
		}
	})
	t.Run("TypeScript run as one reservation", func(t *testing.T) {
		t.Parallel()
		gentest.NeedsBudgetControls(t)
		const units = "spendUnits(n: number): void { this.control?.chargeUnits(n); }"
		c := cases["operators"]
		want := engineLimitReports(t, c, Closure)
		ts := generatedTSLimitReports(t, c, func(b string) string {
			if !strings.Contains(b, units) {
				t.Fatal("the run of units is not in the TypeScript runtime")
			}
			return strings.Replace(b, units, "spendUnits(n: number): void { this.control?.charge(n); }", 1)
		})
		if len(limitDisagreements(want, ts, c.tsWindow)) == 0 {
			t.Error("generated TypeScript that charges a run of units at once agrees with the engine")
		}
	})
}
