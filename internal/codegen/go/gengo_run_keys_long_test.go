//go:build zp_long

package gogen_test

import (
	"math"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/testing/codegentest"
)

// runKeySources are grammars whose run site is keyed by the variables: walks under equal values
// built apart at every position (the engine's "equal environments built apart"), and a new key at
// every position, each with inputs of the sizes runKeySizes.
var runKeySources = []string{
	"var m int\n@start\nrule main = (a / [xy!?])* $$\nrule a = { m := 1 } { m := 0 } w \"!\" / { m := 1 } { m := 0 } w \"?\"\nrule w = (\"x\" { m == 0 } / \"yy\")*\n",
	"var k int\n@start\nrule main = ({ k := k + 1 } (a / [xy]))* $$\nrule a = w \"!\" / w \"?\"\nrule w = (\"x\" { k > 0 })*\n",
}

var runKeySizes = []int{100, 200, 400}

func runKeyInputs() [][]string {
	var inputs [][]string
	for _, unit := range []string{"x", "xy"} {
		var row []string
		for _, n := range runKeySizes {
			row = append(row, strings.Repeat(unit, n))
		}
		inputs = append(inputs, row)
	}
	return inputs
}

// TestRunSitesGeneratedKeyedByValues checks (R2) and (R3) of the linear-cost proof in generated Go
// parsers: walks under equal values of the key share records and a key is looked up in constant
// steps, so the spends grow linearly; keyed by the environment as an object, or looked up by
// stepping through every record of the site, they grow at least quadratically.
func TestRunSitesGeneratedKeyedByValues(t *testing.T) {
	t.Parallel()
	goBin := skipRunsGoBin(t)
	codegentest.NeedsBudgetControls(t)
	inputs := runKeyInputs()
	slope := func(row []uint64) (lo, hi float64) {
		lo, hi = math.Inf(1), math.Inf(-1)
		for i := 1; i < len(row); i++ {
			s := math.Log2(float64(row[i]) / float64(row[i-1]))
			lo, hi = min(lo, s), max(hi, s)
		}
		return lo, hi
	}
	spends, want := runSiteRuns(t, goBin, runKeySources, inputs, "", 1, true)
	for i, row := range spends {
		if _, hi := slope(row); hi > 1.2 {
			t.Errorf("%q: spends %v grow faster than linearly", runKeySources[i], row)
		}
	}
	for i, mutation := range []string{"key by object", "scan keys"} {
		got, results := runSiteRuns(t, goBin, runKeySources[i:i+1], inputs[i:i+1], mutation, 1, true)
		if lo, _ := slope(got[0]); lo < 1.7 {
			t.Errorf("%s: spends %v are not quadratic", mutation, got[0])
		}
		if r := want[i*len(runKeySizes) : (i+1)*len(runKeySizes)]; strings.Join(results, "\n") != strings.Join(r, "\n") {
			t.Errorf("%s: results differ", mutation)
		}
	}
}
