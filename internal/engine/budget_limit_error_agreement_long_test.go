//go:build zp_long && !zp_budget_controls_reference

package engine

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/testing/codegentest"
)

// TestLimitErrorAgreementMutants checks that TestLimitErrorAgreement notices a backend that reports a
// run of units as one reservation: generated Go whose Take reserves its units at once, and generated
// TypeScript whose run of units is charged as one reservation; and one that splits the reservation of
// a list take-off: generated Go and TypeScript that reserve the copy and the take-off apart, and
// generated Go that reserves it chunk by chunk.
func TestLimitErrorAgreementMutants(t *testing.T) {
	t.Parallel()
	codegentest.NeedsBudgetControls(t)
	cases := map[string]limitAgreementCase{}
	for _, c := range limitAgreementCases {
		cases[c.name] = c
	}
	t.Run("atomic Take", func(t *testing.T) {
		t.Parallel()
		codegentest.NeedsBudgetControls(t)
		const unit = "func (c *BudgetControl) takeSlow(n uint64) {\n\tfor ; n > 0; n-- {\n\t\tc.chargeStep()\n\t}\n}"
		runtime := codegentest.LibCopyWith(t, func(name, src string) string {
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
	// A list take-off reserved as a copy and a take-off, or chunk by chunk, is not one reservation of two
	// units for each element.
	kids := "\tif n > 0 {\n\t\tp.Control.Charge(2 * uint64(n))\n\t}\n"
	for name, split := range map[string]string{
		"list take-off as two reservations": "\tif n > 0 {\n\t\tp.Control.Charge(uint64(n))\n\t\tp.Control.Charge(uint64(n))\n\t}\n",
		"list take-off by chunks":           "\tfor k := n; k > 0; k -= min(k, 128) {\n\t\tp.Control.Charge(2 * uint64(min(k, 128)))\n\t}\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			codegentest.NeedsBudgetControls(t)
			runtime := codegentest.LibCopyWith(t, func(file, src string) string {
				if file != "lib.go" && file != "typed.go" {
					return src
				}
				if strings.Count(src, kids) != 1 {
					t.Fatalf("the reservation of a list take-off is not in %s", file)
				}
				return strings.Replace(src, kids, split, 1)
			}, nil)
			c := cases["list"]
			if name == "list take-off by chunks" {
				c = cases["long list"]
			}
			node, typed := generatedGoLimitReports(t, c, runtime)
			for _, size := range c.lists {
				if listReservationMissing(node, size) && listReservationMissing(typed, size) {
					return
				}
			}
			t.Errorf("generated Go with the %s still makes each list take-off one reservation", name)
		})
	}
	t.Run("TypeScript list take-off as two reservations", func(t *testing.T) {
		t.Parallel()
		codegentest.NeedsBudgetControls(t)
		const kids = "if (n > 0) c.charge(2 * n);"
		c := cases["list"]
		ts := generatedTSLimitReports(t, c, func(b string) string {
			if !strings.Contains(b, kids) {
				t.Fatal("the reservation of a list take-off is not in the TypeScript runtime")
			}
			return strings.Replace(b, kids, "if (n > 0) { c.charge(n); c.charge(n); }", 1)
		})
		if !listReservationMissing(ts, c.lists[0]) {
			t.Error("generated TypeScript with a list take-off as two reservations still makes it one")
		}
	})
	t.Run("TypeScript run as one reservation", func(t *testing.T) {
		t.Parallel()
		codegentest.NeedsBudgetControls(t)
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
