package complexity

import (
	"os"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/desugar"
	"zeroparse.dev/zp/internal/grammaranalysis"
	"zeroparse.dev/zp/internal/syntax"
)

// TestRunSitesInClassification checks that the classifier's marking of run sites is off by default,
// and that with it on a repetition whose scan a continuation restarts is not a scanning source,
// whether its element builds a value, captures, reads variables or recovers.
func TestRunSitesInClassification(t *testing.T) {
	defer func() { runSitesOn = false }()
	for _, c := range []struct {
		name, src string
		off, on   ComplexityClass
	}{
		{"memoized scan restarted by a repetition", "rule main = (a \"y\" / \"q\" a / .)* $$\nrule a = x+\nrule x = \"x\"\n", ClassPoly, ClassLinear},
		{"scan restarted through a memoized recursion", "rule main = t a? $$\nrule t = a t \"!\" / . t / \"\"\nrule a = x+\nrule x = \"x\"\n", ClassUnknown, ClassLinear},
		{"element with a capture", "rule main = (a \"y\" / \"q\" a / .)* $$\nrule a = (c:x)+ => 1\nrule x = \"x\"\n", ClassPoly, ClassLinear},
		{"element with #recover", "rule main = (a \"y\" / \"q\" a / .)* $$\nrule a = (x #recover(skip=[^y]*))+\nrule x = \"x\"\n", ClassPoly, ClassLinear},
		{"element with #error", "rule main = (a \"y\" / \"q\" a / .)* $$\nrule a = (x @error(message=\"m\"))+\nrule x = \"x\"\n", ClassPoly, ClassPoly},
	} {
		classOf := func() ComplexityClass { return classifySource(t, c.src).Class }
		runSitesOn = false
		if got := classOf(); got != c.off {
			t.Errorf("%s: off: class %v, want %v", c.name, got, c.off)
		}
		runSitesOn = true
		if got := classOf(); got != c.on {
			t.Errorf("%s: on: class %v, want %v", c.name, got, c.on)
		}
	}
}

// TestRunSitesMatchClassifier checks that the repetitions the engine marks as run sites (and the
// scans and skip-run sites it leaves aside) are those that the classifier counts as run sites,
// in the bundled grammars: the engine keeps records for every repetition the classifier relies on.
func TestRunSitesMatchClassifier(t *testing.T) {
	defer func() { runSitesOn = false }()
	for _, name := range []string{
		"testdata/grammars/cel/cel.zp", "testdata/grammars/cue/cue.zp", "testdata/grammars/duckdb/duckdb.zp",
		"testdata/grammars/golang/golang.zp", "testdata/grammars/python/python.zp", "testdata/grammars/typescript/typescript.zp",
		"testdata/grammars/xml/xml.zp", "testdata/grammars/yaml/yaml.zp", "examples/minilang/minilang.zp", "examples/outline/outline.zp",
	} {
		src, err := os.ReadFile("../../" + name)
		if err != nil {
			t.Fatal(err)
		}
		g, err := syntax.Parse(string(src))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		runSitesOn = true
		k, _, err := prepareClassifier(g, "", nil)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		// The run sites are those of the grammar, which the classifier reads through its view.
		rules := k.rules
		if k.base != nil {
			rules = k.base
		}
		classifier := 0
		for _, r := range rules {
			grammaranalysis.WalkExpr(r.Expr, func(x grammar.Expr) {
				if rep, ok := x.(*grammar.Repeat); ok && !compile.IsScan(rep) && k.runSite(rep) {
					classifier++
				}
			})
		}
		runSitesOn = false
		prog, err := compile.Check(g, compile.Options{}, nil, desugar.InlineNames{}, nil)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		t.Logf("%s: %d run sites, %d skip-run sites", name, prog.NumRunSites, prog.NumSkipSites)
		if got := prog.NumRunSites; got != classifier {
			t.Errorf("%s: the engine marks %d run sites, the classifier %d", name, got, classifier)
		}
	}
}
