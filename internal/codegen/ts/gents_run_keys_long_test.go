//go:build zp_long

package tsgen_test

import (
	"math"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/testing/codegentest"
)

// TestRunSitesTSKeyedByValues checks (R2) and (R3) of the linear-cost proof in generated TypeScript
// parsers, on the grammars of the Go test (runKeySources in the Go generator's tests): walks under
// equal values of the key share records and a key is looked up in constant steps, so the spends
// grow linearly; keyed by the environment as an object, or looked up by stepping through every
// record of the site, they grow at least quadratically.
func TestRunSitesTSKeyedByValues(t *testing.T) {
	t.Parallel()
	node := codegentest.RequireNode(t)
	if testing.Short() {
		t.Skip("runs generated TypeScript")
	}
	sources := []string{
		"var m int\n@start\nrule main = (a / [xy!?])* $$\nrule a = { m := 1 } { m := 0 } w \"!\" / { m := 1 } { m := 0 } w \"?\"\nrule w = (\"x\" { m == 0 } / \"yy\")*\n",
		"var k int\n@start\nrule main = ({ k := k + 1 } (a / [xy]))* $$\nrule a = w \"!\" / w \"?\"\nrule w = (\"x\" { k > 0 })*\n",
	}
	sizes := []int{100, 200, 400}
	var inputs [][]string
	for _, unit := range []string{"x", "xy"} {
		var row []string
		for _, n := range sizes {
			row = append(row, strings.Repeat(unit, n))
		}
		inputs = append(inputs, row)
	}
	slopes := func(row []uint64) (lo, hi float64) {
		lo, hi = math.Inf(1), math.Inf(-1)
		for i := 1; i < len(row); i++ {
			s := math.Log2(float64(row[i]) / float64(row[i-1]))
			lo, hi = min(lo, s), max(hi, s)
		}
		return lo, hi
	}
	hot := [][2]string{{"const RUN_HOT_SPAN = 64;", "const RUN_HOT_SPAN = 1;"}}
	spends, _ := tsSpends(t, node, sources, inputs, []string{}, hot, "parseWithOptions")
	for i, row := range spends {
		if _, hi := slopes(row); hi > 1.2 {
			t.Errorf("%q: spends %v grow faster than linearly", sources[i], row)
		}
	}
	for i, mutation := range []string{"key by object", "scan keys"} {
		got, _ := tsSpends(t, node, sources[i:i+1], inputs[i:i+1], []string{}, append(hot, tsRunSiteMutations[mutation]...), "parseWithOptions")
		if lo, _ := slopes(got[0]); lo < 1.7 {
			t.Errorf("%s: spends %v are not quadratic", mutation, got[0])
		}
	}
}
