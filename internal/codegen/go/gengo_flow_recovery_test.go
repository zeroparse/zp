package gogen_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegen/go"
	"zeroparse.dev/zp/internal/codegentest"
	"zeroparse.dev/zp/internal/compile"
)

// hasRecovery reports a recovery in e, read from the expression itself.
func hasRecovery(e grammar.Expr) (found bool) {
	compile.WalkExpr(e, func(e grammar.Expr) {
		switch e := e.(type) {
		case *grammar.Recovery:
			found = true
		case *grammar.Attributed:
			for _, a := range e.Attrs {
				found = found || a.Name == "recover"
			}
		}
	})
	return
}

// TestFlowRecoveryPrintsFromFlow checks the route of the rules with recovery: over the corpus and the ten
// grammars, in the ordinary and the budget-controlled forms, every rule, Operators Rule line and stream start rule of the
// Node and recognizer tables is printed from Flow IR (otherwise the generation fails), and the rules with a recovery (alone, with a cut and
// with a leader) take the Flow route.
func TestFlowRecoveryPrintsFromFlow(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("generates the corpus")
	}
	cases := append(append(append(codegentest.Representative(codegentest.GenCorpus(t)), codegentest.RepresentativeGrammars(codegentest.GrammarCases(t))...), observeShapes...), streamCorpus...)
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
				if !hasRecovery(r.BodyExpr()) {
					return
				}
				taken[table+" recovery"]++
				if hasCut(r.BodyExpr()) {
					taken[table+" recovery with a cut"]++
				}
				if r.Leader {
					taken[table+" recovery with a leader"]++
				}
			}
			gh.OperatorsRoute = func(table string, r *compile.Rule) {
				mu.Lock()
				defer mu.Unlock()
				taken[table+" operators line"]++
			}
			codegentest.EachParallel(t, cases, func(t *testing.T, c codegentest.Case) {
				g, err := parseTest(c.Src)
				if err != nil {
					return
				}
				for _, stream := range []bool{false, true} {
					if _, err := codegentest.Generate(t, g, codegen.GenOptions{Package: "x", Module: "example.com/x", Start: "main", Lib: codegen.LibImport, Recognize: true, Stream: stream, LimitControls: controls}); err != nil && (!stream || strings.Contains(err.Error(), "Flow IR does not print")) {
						t.Fatalf("%s: %v", c.Name, err)
					}
				}
			})
			for _, table := range []string{"rules", "recRules"} {
				for _, k := range []string{" recovery", " recovery with a cut", " recovery with a leader", " operators line"} {
					if taken[table+k] == 0 {
						t.Errorf("no %s%s takes the Flow route (%v)", table, k, taken)
					}
				}
			}
		})
	}
}
