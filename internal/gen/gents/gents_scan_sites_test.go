package gents_test

import (
	"math"
	"os/exec"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
)

// TestScanSitesTSMatchEngine checks that generated TypeScript parsers, whose repetitions of one
// character record and reuse their walks at scan sites, give the engine's results on the scan-site
// grammars.
func TestScanSitesTSMatchEngine(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("runs generated TypeScript")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found")
	}
	gentest.TSParsersCorpus(t, node, gentest.ScanSiteCases(40), gen.GenOptions{})
}

// TestScanSitesTSLinear checks that grammars which restart a walk of a scan site at one position
// once for each level of a nest spend a budget linear in the input in generated TypeScript
// recognizers, that walks that never take a record make them quadratic and give the same results,
// and that records stored at the wrong start change the results.
func TestScanSitesTSLinear(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("runs generated TypeScript")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found")
	}
	sources := []string{
		"rule main = e $$\nrule e = \"(\" (e ws \"x\" / e) / \".\"\nrule ws = c:[ ]*",
		"rule main = e \" \" ws $$\nrule e = \"(\" e? (ws \")\")?\nrule ws = [ ]* #atomic",
		"rule main = e \" \" ws $$\nrule e = \"(\" e? (ws \")\")?\nrule ws = [ ]*",
	}
	sizes := []int{200, 400, 800}
	var inputs [][]string
	for i := range sources {
		var row []string
		for _, n := range sizes {
			if i == 0 {
				row = append(row, strings.Repeat("(", n)+"."+strings.Repeat(" ", n))
			} else {
				row = append(row, strings.Repeat("(", n)+strings.Repeat(" ", n))
			}
		}
		inputs = append(inputs, row)
	}
	// The results are those of the last source.
	results := []string{strings.Repeat("(", 30) + strings.Repeat(" ", 40), strings.Repeat("(", 30) + strings.Repeat(" ", 40) + ")"}
	slopes := func(row []uint64) (lo, hi float64) {
		lo, hi = math.Inf(1), math.Inf(-1)
		for i := 1; i < len(row); i++ {
			s := math.Log2(float64(row[i]) / float64(row[i-1]))
			lo, hi = min(lo, s), max(hi, s)
		}
		return lo, hi
	}
	spends, got := tsSkipRunSpends(t, node, sources, inputs, results, nil)
	t.Logf("spends at %v: %v", sizes, spends)
	for i, row := range spends {
		if _, hi := slopes(row); hi > 1.2 {
			t.Errorf("%q: spends %v grow faster than linearly", sources[i], row)
		}
	}
	spends, plain := tsSkipRunSpends(t, node, sources, inputs, results, [][2]string{{
		"    const rec = this.scanTables[site]?.get(start);\n",
		"    const rec = undefined as ScanRecord | undefined;\n",
	}})
	t.Logf("spends of walks that never reuse: %v", spends)
	for i, row := range spends {
		if lo, _ := slopes(row); lo < 1.7 {
			t.Errorf("%q, walks that never reuse: spends %v are not quadratic", sources[i], row)
		}
	}
	for i := range got {
		if got[i] != plain[i] {
			t.Errorf("input %q: records change the result\n got  %s\n want %s", results[i], got[i], plain[i])
		}
	}
	_, shifted := tsSkipRunSpends(t, node, sources, inputs, results, [][2]string{{"    t.set(start, rec);\n", "    t.set(start + 1, rec);\n"}})
	differ := false
	for i := range got {
		differ = differ || got[i] != shifted[i]
	}
	if !differ {
		t.Errorf("records stored at the wrong start change no result of %q", results)
	}
}
