package complexity

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

// lateMain is the start rule of the late-part cases: it repeats l, and the pieces of input that l
// leaves, on input ("ab+{")^n "}".
const lateMain = "@start\nrule s = (l / \"+{\" / \"}\")* $$\n"

// TestLateParts checks the late parts of a growth: parts of the body of a leader that read no
// growing result and that a step after the first can reach where the first step did not, because
// a left call that reads the seed matched where it failed in the first step and what followed it
// failed. Such a part that fails after a scanning source drops the step, and the next growth can
// scan the same input again. reason is the start of the reason of a late part that is not
// committing, or "" for none; degree is the witnessed degree of the class unknown.
func TestLateParts(t *testing.T) {
	t.Parallel()
	const z = "rule z = \"b\" \"+\" \"{\" [ab+{]* \"}\" \"!\"\n"
	for _, tc := range []struct {
		name, src string
		class     ComplexityClass
		reason    string
		degree    int
	}{
		// The second step of each growth reads the seed ab, matches +, fails at { and goes on to
		// "a" z, which scans the rest of the input and fails at its end; the next growth, four
		// characters on, scans it again.
		{"after an inner choice", lateMain + "rule l = (l \"+\" / \"a\") \"b\" / \"a\" z\n" + z, ClassUnknown,
			"the late part of l at 3:30 in rule l, which the first step of its growth need not reach, contains the repetition [ab+{]* at 4:22 in rule z and is not committing against the seed of the growth of l", 0},
		// The same with the scan in a repetition of a rule that scans, in a rule that it calls,
		// in a repetition whose element calls a left-recursive rule (no run site), and in the
		// skip of a #recover that is not a reused skip.
		{"repetition of a scanning rule", lateMain + "rule l = (l \"+\" / \"a\") \"b\" / \"a\" z\nrule z = \"b\" \"+\" \"{\" c+ \"}\" \"!\"\nrule c = \"a\" [ab+{]*\n", ClassUnknown, "the late part of l", 0},
		{"scan in a called rule", lateMain + "rule l = (l \"+\" / \"a\") \"b\" / \"a\" z\nrule z = \"b\" \"+\" \"{\" c+ \"}\" \"!\"\nrule c = \"a\" d\nrule d = [ab+{]*\n", ClassUnknown, "the late part of l", 0},
		{"element with a left-recursive call", lateMain + "rule l = (l \"+\" / \"a\") \"b\" / \"a\" z\nrule z = \"b\" \"+\" \"{\" (c / x)* \"}\" \"!\"\nrule c = \"a\" / \"b\" / \"+\" / \"{\"\nrule x = x \"#\" / \"#\"\n", ClassUnknown, "the late part of l", 0},
		{"scan in a recovery skip", lateMain + "rule l = (l \"+\" / \"a\") \"b\" / \"a\" r\nrule r = (\"b\" \"+\" \"{\" \"!\") #recover(skip=(c / x)* \"}\") \"!\"\nrule c = \"a\" / \"b\" / \"+\" / \"{\"\nrule x = x \"#\" / \"#\"\n", ClassUnknown, "the late part of l", 0},
		// A late part that is not the last alternative: the alternative after it fails too.
		{"late part before the last alternative", lateMain + "rule l = (l \"+\" / \"a\") \"b\" / \"a\" z / \"q\"\n" + z, ClassUnknown, "the late part of l", 0},
		// A left call of another leader of the cycle can match otherwise in each step: in the
		// second step of l, m grows again from the seed of l, matches "ab+", "b" fails at "{",
		// and "a" z, which the first step never reached, scans the rest of the input. The units
		// of m and l reject the grammar as well.
		{"after a left call of another leader", lateMain + "rule l = l \"y\" / m \"b\" / \"a\" z\nrule m = m \"x\" / l \"+\" / \"a\"\n" + z, ClassUnknown,
			"the late part of l at 3:26 in rule l", 0},
		// A late part that gives up the scan inside an option, a rule that it calls, a repetition
		// or an optional choice, and then succeeds no farther than the seed, drops the step as a
		// failing one does.
		{"gives up a scan in an option", lateMain + "rule l = (l \"+\" / \"a\") \"b\" / \"a\" z?\nrule z = \"b\" \"+\" \"{\" (c / x)* \"}\" \"!\"\nrule c = \"a\" / \"b\" / \"+\" / \"{\"\nrule x = x \"#\" / \"#\"\n", ClassUnknown, "the late part of l", 0},
		{"gives up a scan in a called option", lateMain + "rule l = (l \"+\" / \"a\") \"b\" / \"a\" w\nrule w = z?\nrule z = \"b\" \"+\" \"{\" (c / x)* \"}\" \"!\"\nrule c = \"a\" / \"b\" / \"+\" / \"{\"\nrule x = x \"#\" / \"#\"\n", ClassUnknown, "the late part of l", 0},
		{"gives up a scan in a repetition", lateMain + "rule l = (l \"+\" / \"a\") \"b\" / \"a\" z*\nrule z = \"b\" \"+\" \"{\" (c / x)* \"}\" \"!\"\nrule c = \"a\" / \"b\" / \"+\" / \"{\"\nrule x = x \"#\" / \"#\"\n", ClassUnknown, "the late part of l", 0},
		{"gives up a scan in an optional choice", lateMain + "rule l = (l \"+\" / \"a\") \"b\" / \"a\" (z / \"q\")?\nrule z = \"b\" \"+\" \"{\" (c / x)* \"}\" \"!\"\nrule c = \"a\" / \"b\" / \"+\" / \"{\"\nrule x = x \"#\" / \"#\"\n", ClassUnknown, "the late part of l", 0},
		// A late part that succeeds keeps the step, and its lookahead, which scans, is checked as
		// the unit it is; the late part adds no degree to it.
		{"kept step with a scanning lookahead", lateMain + "rule l = (l \"+\" / \"a\") \"b\" / \"a\" z\nrule z = \"b\" \"+\" \"{\" !([ab+{]* \"}\" \"!\")\n", ClassUnknown, "the late part of l", 2},
		// Controls. A first alternative that the step evaluates again succeeds again, so the
		// later one is never reached; the inner choice is followed by nothing that fails; the
		// late part begins otherwise than the seed; nothing fails after the scan.
		{"earlier alternative matches again", lateMain + "rule l = l \"+\" \"b\" / \"a\" \"b\" / \"a\" z\n" + z, ClassLinear, "", 0},
		{"option after an alternative that matches again", lateMain + "rule l = l \"+\" \"b\" / \"a\" \"b\" / \"a\" z?\nrule z = \"b\" \"+\" \"{\" (c / x)* \"}\" \"!\"\nrule c = \"a\" / \"b\" / \"+\" / \"{\"\nrule x = x \"#\" / \"#\"\n", ClassLinear, "", 0},
		{"nothing fails after the inner choice", lateMain + "rule l = (l \"+\" \"b\" / \"a\" \"b\") / \"a\" z\n" + z, ClassLinear, "", 0},
		{"late part committing against the seed", lateMain + "rule l = (l \"+\" / \"a\") \"b\" / \"q\" z\n" + z, ClassLinear, "", 0},
		// A skip of single characters is a reused skip, whose records keep the parse linear.
		{"reused skip", lateMain + "rule l = (l \"+\" / \"a\") \"b\" / \"a\" r\nrule r = (\"b\" \"+\" \"{\" \"!\") #recover(skip=[ab+{]* \"}\") \"!\"\n", ClassLinear, "", 0},
		// The part and the seed begin alike with "a"; past it, the part fails on the seed followed
		// by what follows l at "!", within the bound of the strict analysis, as an alternative that
		// the rewrite left-factors would.
		{"late part that parts from the seed after their shared start", "@start\nrule s = l \";\" $$\nrule l = (l \"+\" / \"a\") \"b\" / \"a\" z\nrule z = \"b\" \"+\" \"!\" [ab+{]* \"}\" \"!\"\n", ClassLinear, "", 0},
		{"late part that cannot fail after its scan", lateMain + "rule l = (l \"+\" / \"a\") \"b\" / \"a\" z\nrule z = \"b\" \"+\" \"{\" [ab+{]*\n", ClassLinear, "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := classifySource(t, tc.src)
			if c.Class != tc.class {
				t.Fatalf("class %v, want %v\ncauses %v\nreasons %v", c.Class, tc.class, c.Causes, c.Reasons)
			}
			if tc.degree != 0 && c.WitnessedDegree != tc.degree {
				t.Errorf("witnessed degree %d, want %d\ncauses %v", c.WitnessedDegree, tc.degree, c.Causes)
			}
			found := false
			for _, r := range c.Reasons {
				if strings.HasPrefix(r.Message, "the late part") {
					found = true
					if tc.reason == "" || !strings.HasPrefix(r.Message, tc.reason) {
						t.Errorf("reason %q, want %q", r.Message, tc.reason)
					}
				}
			}
			if tc.reason != "" && !found {
				t.Errorf("no late-part reason, want %q\nreasons %v", tc.reason, c.Reasons)
			}
		})
	}
}
