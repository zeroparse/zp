package complexity_test

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/engine"
	"zeroparse.dev/zp/internal/analysis/complexity"
	"zeroparse.dev/zp/internal/engine/state"
	"zeroparse.dev/zp/internal/testing/complexitytest"
	"zeroparse.dev/zp/internal/testing/starttest"
)

// copyScaleHead is the grammar of TestCopyStopsScale without W, whose first alternative
// continuation matches its lookahead in each case only through a copy.
const copyScaleHead = "@start\nrule main = E \"c\" L \"z\"\nrule E = \"(\" W / [b]+\nrule L = \"(\" L / \"b\"\n"

// TestCopyStopsScale measures the grammars whose continuation passes a copy that a negative
// lookahead's decision relies on, and holds the classifier to the measurement: a grammar whose
// cost grows faster than linearly is not classified linear. The cost is the least compute limit
// under which the parse ends, which is deterministic. On (^n bc (^n bq the first alternative of
// W goes on at each of the n levels of the nesting and copies the text of L, about n characters,
// so the cost grows as n^2. Without the stop at the copy (the mutant) each grammar is classified
// linear, and the test fails. The reference build has no compute limit, so it checks the classes
// alone.
func TestCopyStopsScale(t *testing.T) {
	defer complexity.SetCopyStops(true)()
	input := func(n int) string { return strings.Repeat("(", n) + "bc" + strings.Repeat("(", n) + "bq" }
	for _, c := range []struct{ name, w, rules string }{
		{"predicate", `!(L "c" L "z") E "c" x:L {len(text($x)) >= 0} "z" / L`, ""},
		{"predicate in a called rule", `!(L "c" L "z") E "c" M "z" / L`, "rule M = x:L {len(text($x)) >= 0}\n"},
	} {
		src := copyScaleHead + "rule W = " + c.w + "\n" + c.rules
		g, err := engine.ParseGrammar(src)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		g = starttest.MarkAll(g)
		p, err := engine.Compile(g, "main")
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		backends := complexitytest.Backends
		if !state.BudgetControlsEnabled {
			backends = nil
		}
		for _, b := range backends {
			s := complexitytest.Series{Sizes: []int{125, 250, 500}}
			for _, n := range s.Sizes {
				cost, err := complexitytest.StepsLenient(p, input(n), b, complexitytest.Cap)
				if err != nil {
					t.Fatalf("%s: %s: size %d: %v", c.name, complexitytest.BackendName(b), n, err)
				}
				if cost.ParseErr == nil {
					t.Fatalf("%s: %s: size %d parses, want the syntax error at q", c.name, complexitytest.BackendName(b), n)
				}
				s.Costs = append(s.Costs, cost)
			}
			if got := complexitytest.Classify(s); got != complexitytest.Poly(2) {
				t.Errorf("%s: %s: costs %v (slopes %.2f) measured %v, want %v", c.name, complexitytest.BackendName(b), s.Costs, s.Slopes(), got, complexitytest.Poly(2))
			}
		}
		for _, stops := range []bool{true, false} {
			restore := complexity.SetCopyStops(stops)
			got, err := complexity.ClassifyComplexityControlled(g, "main", nil)
			restore()
			if err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
			linear := got.Class == complexity.ClassLinear || got.Class == complexity.ClassLinearEnv
			switch {
			case stops && linear:
				t.Errorf("%s: classified %v, measured %v", c.name, got.Class, complexitytest.Poly(2))
			case !stops && !linear:
				// The mutant must claim linear, or the grammar would not witness the stop.
				t.Errorf("%s without the stop: classified %v, want linear", c.name, got.Class)
			}
		}
	}
}
