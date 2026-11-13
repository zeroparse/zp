package sample_test

import (
	"os"
	"slices"
	"testing"

	"zeroparse.dev/zp/engine"
	"zeroparse.dev/zp/sample"
)

// TestPythonCoverageFloor holds the coverage of the Python grammar, whose statements depend on
// predicates over the indentation that rules capture as lists of nodes with captures: the generator
// reaches statements and blocks only by deciding them. The floors are a little below the measured
// coverage (122 rules and 113 alternatives), so that changes to the search do not trip them.
func TestPythonCoverageFloor(t *testing.T) {
	if testing.Short() {
		t.Skip("generates 50 Python inputs")
	}
	src, err := os.ReadFile("../testdata/grammars/python/python.zp")
	if err != nil {
		t.Fatal(err)
	}
	p, err := engine.CompileSource(string(src), "main")
	if err != nil {
		t.Fatal(err)
	}
	g, err := sample.New(p, sample.WithSeed(1), sample.WithCoverage())
	if err != nil {
		t.Fatal(err)
	}
	inputs, err := g.Generate(50)
	if err != nil {
		t.Fatal(err)
	}
	checkParse(t, p, inputs)
	c := g.Coverage()
	t.Logf("rules %d/%d, alternatives %d/%d", c.RulesCovered, c.Rules, c.AlternativesCovered, c.Alternatives)
	if c.RulesCovered < 110 || c.AlternativesCovered < 100 {
		t.Errorf("rules %d/%d, alternatives %d/%d; want at least 110 rules and 100 alternatives", c.RulesCovered, c.Rules, c.AlternativesCovered, c.Alternatives)
	}
	for _, r := range []string{"first_stmts", "stmts", "stmt", "simple_stmts", "block"} {
		if slices.Contains(c.MissedRules, r) {
			t.Errorf("no input exercised rule %s", r)
		}
	}
}
