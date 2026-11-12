package compile

import (
	"strings"
	"testing"
)

// stepMain is the start rule of the step cases: it repeats S, and a letter other than b where S
// does not match.
const stepMain = "@start\nrule main = (S / [ac-z])* $$\n"

// TestStepUnits checks the unit of a step of a growth after the first. A step that fails after a
// scanning source in its tail is dropped, and the parser goes on at the end of the seed with what
// follows the calls that started the growth, which can examine the scanned input again. reason
// is the start of the reason of a step that is not committing, or "" for none.
func TestStepUnits(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, src string
		class     ComplexityClass
		reason    string
	}{
		{"source in a tail outside every unit", stepMain + `rule S = (S "a" / "b") T
rule T = "c" / "d" [a-z]* "!"
`, ClassUnknown, "a step of the growth of S at 3:10 in rule S can fail after the repetition [a-z]* at 4:20 in rule T and is not committing against what follows the call S at 2:14 in rule main"},
		{"tail through the end of another rule of the cycle", stepMain + `rule S = F T
rule F = S "a" / "b"
rule T = "c" / "d" [a-z]* "!"
`, ClassUnknown, "a step of the growth of S at 3:10 in rule S can fail after the repetition [a-z]* at 5:20 in rule T and is not committing against what follows the call S at 2:14 in rule main"},
		{"no source in the tail", stepMain + `rule S = (S "a" / "b") T
rule T = "c" / "d" "!"
`, ClassLinear, ""},
		{"nothing fails after the source", stepMain + `rule S = (S "a" / "b") T
rule T = "c" / "d" [a-z]* "!"?
`, ClassLinear, ""},
		{"continuation disjoint from the tail", `@start
rule main = (S ";" / [ac-z])* $$
rule S = (S "a" / "b") T
rule T = "c" / "d" [a-z]* "!"
`, ClassLinear, ""},
		{"bounded repetition", stepMain + `rule S = (S "a" / "b") T
rule T = "c" / "d" [a-z]#repeat(,3) "!"
`, ClassLinear, ""},
		// The tails R "!" and "+" "x" begin differently: each is committing against what
		// follows E in s, R "?", on its own, after the shared call of R, while the choice of both
		// shares no call with it.
		{"tails compared one at a time", `@start
rule s = E R "?" $$
rule E = E R "!" / E "+" "x" / "x"
rule R = [ ]*
`, ClassLinear, ""},
		{"expression", `@start
rule main = E $$
rule E = E "+" T / T
rule T = "(" E ")" / [0-9]+
`, ClassLinear, ""},
		// The growing unit S "a" T fails after the source too, which the growing-unit check
		// reports without a degree.
		{"failure inside the growing unit", stepMain + `rule S = S "a" T / "b" "c"
rule T = "c" / "d" [a-z]* "!"
`, ClassUnknown, "a step of the growth of S"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := classifySource(t, tc.src)
			if c.Class != tc.class {
				t.Fatalf("class %v, want %v\ncauses %v\nreasons %v", c.Class, tc.class, c.Causes, c.Reasons)
			}
			found := false
			for _, r := range c.Reasons {
				if strings.HasPrefix(r.Message, "a step of the growth") {
					found = true
					if tc.reason == "" || !strings.HasPrefix(r.Message, tc.reason) {
						t.Errorf("reason %q, want %q", r.Message, tc.reason)
					}
				}
			}
			if tc.reason != "" && !found {
				t.Errorf("no step reason, want %q\nreasons %v", tc.reason, c.Reasons)
			}
		})
	}
}
