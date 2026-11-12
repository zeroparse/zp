package compile

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/desugar"
	"zeroparse.dev/zp/internal/syntax"
)

// TestLinearKeepsSites checks that @linear, which only adds a check, leaves the sites of a program
// as they are without it: a pure scan rule declared @linear stays a scan rule, so the repetition in
// it, which no iteration reaches, is still a run site.
func TestLinearKeepsSites(t *testing.T) {
	const src = "@start\nrule main = r \"a\" / r \"b\"\n%srule r = (\"xy\")*\n"
	sites := func(decl string) (run, scan int) {
		t.Helper()
		g, err := syntax.Parse(strings.Replace(src, "%s", decl, 1))
		if err != nil {
			t.Fatal(err)
		}
		prog, err := CompileProgram(g, Options{}, nil, desugar.InlineNames{}, nil, func(*Compiler) ExprBuilder[struct{}] { return noExprs{} })
		if err != nil {
			t.Fatal(err)
		}
		return prog.NumRunSites, prog.NumScanSites
	}
	run, scan := sites("")
	if run != 1 {
		t.Fatalf("without @linear: %d run sites, want 1 (the repetition of the scan rule r)", run)
	}
	if gotRun, gotScan := sites("@linear\n"); gotRun != run || gotScan != scan {
		t.Errorf("with @linear: %d run sites and %d scan sites, want %d and %d as without it", gotRun, gotScan, run, scan)
	}
}
