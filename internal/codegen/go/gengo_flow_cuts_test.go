package gogen_test

import (
	"fmt"
	"sync"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegen/go"
	"zeroparse.dev/zp/internal/codegentest"
	"zeroparse.dev/zp/internal/compile"
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
	cases := append(append(codegentest.Representative(codegentest.GenCorpus(t)), codegentest.RepresentativeGrammars(codegentest.GrammarCases(t))...), observeShapes...)
	for _, controls := range []bool{false, true} {
		t.Run(fmt.Sprint("controls=", controls), func(t *testing.T) {
			t.Parallel()
			taken := map[string]int{}
			var mu sync.Mutex
			gh := &gogen.Hooks{}
			codegentest.SetGenHooks(t, gh)
			gh.FramedRoute = func(table string, r *compile.Rule) {
				mu.Lock()
				defer mu.Unlock()
				if hasCut(r.BodyExpr()) {
					taken[table]++
				}
			}
			codegentest.EachParallel(t, cases, func(t *testing.T, c codegentest.Case) {
				g, err := parseTest(c.Src)
				if err != nil {
					return
				}
				if _, err := codegentest.Generate(t, g, codegen.GenOptions{Package: "x", Module: "example.com/x", Start: "main", Lib: codegen.LibImport, Recognize: true, LimitControls: controls}); err != nil {
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
