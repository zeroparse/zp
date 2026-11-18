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

// TestGrowStartScale measures the grammars whose growing unit goes on at its start in the first
// step and whose later alternative can begin with what it matches there, and holds the classifier
// to the measurement: a grammar whose cost grows faster than linearly is not classified linear.
// The cost is the least compute limit under which the parse ends, which is deterministic. In a
// later step the unit scans the x after the seed and fails, and the later alternative consumes
// the character at the position where the rule grows and starts a growth at the next position,
// which scans the x again, so the cost grows as n^2. Without the check (the mutant) each grammar
// is classified linear, and the test fails. The reference build has no compute limit, so it checks
// the classes alone.
func TestGrowStartScale(t *testing.T) {
	defer complexity.SetGrowStart(true)()
	for _, c := range []struct {
		name, src string
		input     func(n int) string
	}{
		{"later alternative after a shared literal",
			"@start\nrule main = E $$\nrule E = (E [x]* / \"\") \"x\" / \"x\" F\nrule F = E \";\"\n",
			func(n int) string { return strings.Repeat("x", n+1) + strings.Repeat(";", n) }},
		{"later alternative after a shared call",
			"@start\nrule main = G \"!\"\nrule G = (G [x]* / \"\") \"(\" L / \"(\" G \"y\" / \"b\" [x]*\nrule L = \"(\" L / \"b\"\n",
			func(n int) string {
				return strings.Repeat("(", n) + "b" + strings.Repeat("x", n) + strings.Repeat("y", n) + "!"
			}},
	} {
		g, err := engine.ParseGrammar(c.src)
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
				cost, err := complexitytest.StepsLenient(p, c.input(n), b, complexitytest.Cap)
				if err != nil {
					t.Fatalf("%s: %s: size %d: %v", c.name, complexitytest.BackendName(b), n, err)
				}
				s.Costs = append(s.Costs, cost)
			}
			t.Logf("%s: %s: costs %v (slopes %.2f)", c.name, complexitytest.BackendName(b), s.Costs, s.Slopes())
			if got := complexitytest.Classify(s); got != complexitytest.Poly(2) {
				t.Errorf("%s: %s: costs %v (slopes %.2f) measured %v, want %v", c.name, complexitytest.BackendName(b), s.Costs, s.Slopes(), got, complexitytest.Poly(2))
			}
		}
		for _, on := range []bool{true, false} {
			restore := complexity.SetGrowStart(on)
			got, err := complexity.ClassifyComplexityControlled(g, "main", nil)
			restore()
			if err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
			linear := got.Class == complexity.ClassLinear || got.Class == complexity.ClassLinearEnv
			switch {
			case on && linear:
				t.Errorf("%s: classified %v, measured %v", c.name, got.Class, complexitytest.Poly(2))
			case !on && !linear:
				// The mutant must claim linear, or the grammar would not witness the check.
				t.Errorf("%s without the check: classified %v, want linear", c.name, got.Class)
			}
		}
	}
}
