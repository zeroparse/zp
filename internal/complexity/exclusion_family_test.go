package complexity_test

import (
	"testing"
	"zeroparse.dev/zp/internal/complexity"

	"zeroparse.dev/zp/engine"
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/complexitytest"
	"zeroparse.dev/zp/internal/starttest"
)

// TestSharedContinuationsFlip checks that the classifier's use of the exclusion analysis changes
// the class of the grammars of complexitytest.SharedContinuationGrammars that TestLinearClaims
// measures: some are classified linear with the decided units and trivia rules that are on by
// default and are not without them. TestLinearClaims then holds those claims against
// measurement; where no grammar changed class, the differential test would give no evidence for
// the defaults. It takes the grammars that the default tier of TestLinearClaims measures, which the
// other settings of the test measure as well.
func TestSharedContinuationsFlip(t *testing.T) {
	linear := func(c complexity.ComplexityClass) bool {
		return c == complexity.ClassLinear || c == complexity.ClassLinearEnv
	}
	classify := func(g *grammar.Grammar) complexity.ComplexityClass {
		c, err := complexity.ClassifyComplexityControlled(g, "main", nil)
		if err != nil {
			t.Fatal(err)
		}
		return c.Class
	}
	count := complexitytest.SharedContinuationCount(complexitytest.DefaultDifferentialGrammars / 4)
	var compiled, flipped, linearOn int
	for i, src := range complexitytest.SharedContinuationGrammars(count) {
		g, err := engine.ParseGrammar(src)
		if err != nil {
			t.Fatalf("grammar %d does not parse: %v\n%s", i, err, src)
		}
		g = starttest.MarkAll(g)
		if _, err := engine.Compile(g, "main"); err != nil {
			continue
		}
		compiled++
		on := classify(g)
		restore := complexity.ExclusionHooks{}.Set()
		off := classify(g)
		restore()
		if linear(on) {
			linearOn++
		}
		if linear(on) && !linear(off) {
			flipped++
		}
	}
	t.Logf("%d grammars compile, %d classified linear, %d of them only with the analysis", compiled, linearOn, flipped)
	if compiled < count*9/10 {
		t.Errorf("%d of %d grammars compile", compiled, count)
	}
	if flipped == 0 {
		t.Error("no grammar changes class with decided units and trivia rules on: the differential test gives no evidence for them")
	}
}
