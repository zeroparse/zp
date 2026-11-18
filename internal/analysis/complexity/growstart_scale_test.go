package complexity_test

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/analysis/complexity"
	"zeroparse.dev/zp/internal/runtime/state"
	"zeroparse.dev/zp/internal/testing/complexitytest"
	"zeroparse.dev/zp/internal/testing/enginetest"
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
	checkGrowScale(t, complexity.SetGrowStart, []growScaleCase{
		{"later alternative after a shared literal",
			"@start\nrule main = E $$\nrule E = (E [x]* / \"\") \"x\" / \"x\" F\nrule F = E \";\"\n",
			func(n int) string { return strings.Repeat("x", n+1) + strings.Repeat(";", n) }},
		{"later alternative after a shared call",
			"@start\nrule main = G \"!\"\nrule G = (G [x]* / \"\") \"(\" L / \"(\" G \"y\" / \"b\" [x]*\nrule L = \"(\" L / \"b\"\n",
			func(n int) string {
				return strings.Repeat("(", n) + "b" + strings.Repeat("x", n) + strings.Repeat("y", n) + "!"
			}},
	})
}

// TestGrowNullableScale measures the grammars whose growing unit goes on at its start in the
// first step through a path that matches without consuming input, before a negative lookahead
// that examines a character with which the later alternative cannot begin, as TestGrowStartScale
// does. On x^n ; the path matches at each x, and the later alternative consumes it in a later step
// and starts a growth at the next position, which scans the x again, so the cost grows as n^2.
// Without the rule (the mutant) each grammar is classified linear, and the test fails.
func TestGrowNullableScale(t *testing.T) {
	input := func(n int) string { return strings.Repeat("x", n) + ";" }
	checkGrowScale(t, complexity.SetGrowNullable, []growScaleCase{
		{"path that ends with a negative lookahead",
			"@start\nrule main = E $$\nrule E = ((E [x]* / \"\") !\";\" / \"x\" F) \"x\"\nrule F = E \",\"\n", input},
		{"path with a negative lookahead and an empty literal",
			"@start\nrule main = E $$\nrule E = ((E [x]* / \"\") !\";\" \"\" / \"x\" F) \"x\"\nrule F = E \",\"\n", input},
	})
}

type growScaleCase struct {
	name, src string
	input     func(n int) string
}

// checkGrowScale measures each case at three sizes on every backend and requires a cost that grows
// as n^2, and classifies it with the check that set turns on and off: on, it must not be linear;
// off (the mutant), it must be linear, or the grammar would not witness the check.
func checkGrowScale(t *testing.T, set func(bool) func(), cases []growScaleCase) {
	t.Helper()
	defer set(true)()
	for _, c := range cases {
		g, err := enginetest.ParseGrammar(c.src)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		g = starttest.MarkAll(g)
		p, err := enginetest.Compile(g, "main")
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
			restore := set(on)
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
