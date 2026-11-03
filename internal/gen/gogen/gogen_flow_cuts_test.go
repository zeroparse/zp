package gogen_test

import (
	"fmt"
	"sync"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gen/gogen"
	"zeroparse.dev/zp/internal/gentest"
)

// hasCut reports a cut in e.
func hasCut(e grammar.Expr) (found bool) {
	compile.WalkExpr(e, func(e grammar.Expr) {
		if _, ok := e.(*grammar.Cut); ok {
			found = true
		}
	})
	return
}

// TestFlowCutsPrintFromFlow checks the route of the rules with cuts: over the corpus and the ten grammars, in
// the ordinary and the budget-controlled forms, a rule with a cut takes
// the Flow route in the Node and recognizer tables (a rule Flow IR does not print fails the generation).
func TestFlowCutsPrintFromFlow(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("generates the corpus")
	}
	cases := append(append(gentest.Representative(gentest.GenCorpus(t)), gentest.RepresentativeGrammars(gentest.GrammarCases(t))...), observeShapes...)
	for _, controls := range []bool{false, true} {
		t.Run(fmt.Sprint("controls=", controls), func(t *testing.T) {
			t.Parallel()
			taken := map[string]int{}
			var mu sync.Mutex
			gh := &gogen.Hooks{}
			gentest.SetGenHooks(t, gh)
			gh.FramedRoute = func(table string, r *compile.Rule) {
				mu.Lock()
				defer mu.Unlock()
				if hasCut(r.BodyExpr()) {
					taken[table]++
				}
			}
			gentest.EachParallel(t, cases, func(t *testing.T, c gentest.Case) {
				g, err := parseTest(c.Src)
				if err != nil {
					return
				}
				if _, err := gentest.Generate(t, g, gen.GenOptions{Package: "x", Module: "example.com/x", Start: "main", Runtime: gen.RuntimeImport, Recognize: true, LimitControls: controls}); err != nil {
					t.Fatalf("%s: %v", c.Name, err)
				}
			})
			for _, table := range []string{"rules", "recRules"} {
				if taken[table] == 0 {
					t.Errorf("no rule with a cut takes the Flow route in the %s table (%v)", table, taken)
				}
			}
		})
	}
}
