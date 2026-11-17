package tsgen_test

import (
	"math"
	"os/exec"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/testing/codegentest"
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
	codegentest.TSParsersCorpus(t, node, codegentest.ScanSiteCases(40), codegen.GenOptions{})
}

// TestScanSitesTSLinear checks that grammars which restart a walk of a scan site at one position
// once for each level of a nest spend a budget linear in the input in generated TypeScript
// recognizers, that walks that never take a run make them quadratic and give the same results,
// and that runs with the wrong end change the results.
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
		"      this.scanWalkTab = this.scanTabs[site] ?? null;\n",
		"      this.scanWalkTab = null;\n",
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
	_, shifted := tsSkipRunSpends(t, node, sources, inputs, results, [][2]string{{tsScanRunLiteral, strings.Replace(tsScanRunLiteral, "end: this.pos,", "end: this.pos + 1,", 1)}})
	differ := false
	for i := range got {
		differ = differ || got[i] != shifted[i]
	}
	if !differ {
		t.Errorf("runs with the wrong end change no result of %q", results)
	}
}

// tsScanRunLiteral is the line of the library that makes a run, which the negative controls change.
const tsScanRunLiteral = "    const r: ScanRun = { first: start, end: this.pos, examined: this.scanExaminedEnd(), count, rem: null, buf: null, next: 0 };\n"

// TestScanSitesTSIntervals checks that walks which begin at different positions of one run take the
// rest from the run that an earlier walk recorded in generated TypeScript: the budgets of recognizing
// the controls of the engine (codegentest.ScanIntervalControls) grow linearly, and grow quadratically
// with the same results when a run answers only the walks from its first position.
func TestScanSitesTSIntervals(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("runs generated TypeScript")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found")
	}
	var sources []string
	var inputs [][]string
	for _, c := range codegentest.ScanIntervalControls {
		sources = append(sources, c.Src)
		inputs = append(inputs, []string{c.Input(200), c.Input(400), c.Input(800)})
	}
	last := codegentest.ScanIntervalControls[len(codegentest.ScanIntervalControls)-1]
	results := []string{last.Input(17), last.Input(40), "y"}
	spends, got := tsSkipRunSpends(t, node, sources, inputs, results, nil)
	off, plain := tsSkipRunSpends(t, node, sources, inputs, results, [][2]string{{
		"    if (rb < 0 || (this.unit === Bytes && start > run.end - SCAN_REACH)) {\n",
		"    if (rb < 0 || start !== run.first) {\n",
	}})
	for i, c := range codegentest.ScanIntervalControls {
		t.Logf("%s: spends %v, intervals off %v", c.Name, spends[i], off[i])
		if r := float64(spends[i][2]) / float64(spends[i][1]); r > 2.2 {
			t.Errorf("%s: spends %v grow by %.2f, want linear", c.Name, spends[i], r)
		}
		if r := float64(off[i][2]) / float64(off[i][1]); r < 3.5 {
			t.Errorf("%s, runs that answer only their first position: spends %v grow by %.2f, want quadratic", c.Name, off[i], r)
		}
	}
	for i := range got {
		if got[i] != plain[i] {
			t.Errorf("input %q: runs change the result\n got  %s\n want %s", results[i], got[i], plain[i])
		}
	}
}
