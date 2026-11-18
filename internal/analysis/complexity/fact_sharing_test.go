package complexity

import (
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/frontend"
	"zeroparse.dev/zp/internal/frontend/desugar"
	"zeroparse.dev/zp/internal/frontend/parser"
	"zeroparse.dev/zp/internal/middleend/corepass"
	"zeroparse.dev/zp/internal/pass"
)

// factComputations returns how many times f computes the rule facts of a module. The count is
// program-wide, so the test that calls it does not run in parallel with others.
func factComputations(f func()) int {
	before := pass.Computations(corepass.FactsKey.Name())
	f()
	return pass.Computations(corepass.FactsKey.Name()) - before
}

// TestTypedClassifierSharesFacts measures the rule-fact computations of the classification with
// value types: it reads the facts that the check took, through the manager of the check's
// module, where checking and then taking the facts of the same rules again computes them twice.
func TestTypedClassifierSharesFacts(t *testing.T) {
	g, err := parser.Parse("@start\nrule s = xs:x* {len($xs) < 3} \"!\"\nrule x = \"a\" / \"b\" x\n")
	if err != nil {
		t.Fatal(err)
	}
	separate := factComputations(func() {
		prog, err := frontend.Check(g, compile.Options{}, nil, desugar.InlineNames{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := classifierFacts(prog.Grammar, prog.Grammar.Rules(), nil); err != nil {
			t.Fatal(err)
		}
	})
	shared := factComputations(func() {
		if k, _ := prepareTypedClassifier(g, "", nil); k == nil {
			t.Fatal("no typed classifier")
		}
	})
	t.Logf("rule-fact computations: %d checking and taking the facts again, %d shared", separate, shared)
	if separate != 2 || shared != 1 {
		t.Errorf("computations %d separate and %d shared, want 2 and 1", separate, shared)
	}
}
