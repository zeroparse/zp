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

// hasCaptureOfRepeat reports a capture of a repetition in e, the shape that Core IR projects in the Node table.
func hasCaptureOfRepeat(e grammar.Expr) (capture, repeated bool) {
	compile.WalkExpr(e, func(e grammar.Expr) {
		if c, ok := e.(*grammar.Capture); ok {
			capture = true
			if _, ok := c.Expr.(*grammar.Repeat); ok {
				repeated = true
			}
		}
	})
	return
}

// TestFlowRecognizerPrintsCapturesFromFlow checks the route of the rules with captures: over the corpus and the
// ten grammars, in the ordinary and the budget-controlled forms, every rule of the Node and recognizer tables is printed from Flow IR (a rule
// it does not print fails the generation), and the rules with captures (and with captured repetitions) take the Flow route.
func TestFlowRecognizerPrintsCapturesFromFlow(t *testing.T) {
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
			gh.LeanRoute = func(table string, r *compile.Rule) {
				mu.Lock()
				defer mu.Unlock()
				if c, _ := hasCaptureOfRepeat(r.BodyExpr()); c {
					taken[table+" lean with captures"]++
				}
			}
			gh.FramedRoute = func(table string, r *compile.Rule) {
				mu.Lock()
				defer mu.Unlock()
				c, rep := hasCaptureOfRepeat(r.BodyExpr())
				if c {
					taken[table+" framed with captures"]++
				}
				if rep {
					taken[table+" framed with a captured repetition"]++
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
			for _, k := range []string{"recRules lean with captures", "recRules framed with captures", "recRules framed with a captured repetition", "rules framed with a captured repetition"} {
				if taken[k] == 0 {
					t.Errorf("no rule takes the Flow route as %q (%v)", k, taken)
				}
			}
		})
	}
}
