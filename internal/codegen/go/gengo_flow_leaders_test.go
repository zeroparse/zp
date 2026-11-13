package gogen_test

import (
	"fmt"
	"sync"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegen/go"
	"zeroparse.dev/zp/internal/codegentest"
	"zeroparse.dev/zp/internal/compile"
)

// TestFlowLeadersPrintFromFlow checks that every leader takes the Flow route in the Node and
// recognizer tables, over the corpus and the ten grammars in both forms (a rule Flow IR does not
// print fails the generation).
func TestFlowLeadersPrintFromFlow(t *testing.T) {
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
				if r.Leader {
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
					t.Errorf("no leader takes the Flow route in the %s table (%v)", table, taken)
				}
			}
		})
	}
}
