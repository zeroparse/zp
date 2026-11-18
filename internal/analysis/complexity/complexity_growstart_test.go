package complexity

import "testing"

// growStartCases are grammars with a growing unit of a leader that goes on at its start in the
// first step of the growth, after its left call fails. without is the class with growStartOn off.
var growStartCases = []struct {
	name, src string
	class     ComplexityClass
	without   ComplexityClass
}{
	// On x^(n+1) ;^n the later alternative "x" F consumes the x at the position where E grows,
	// in a step after the unit's [x]* scanned the rest of the x, and starts a growth at the next
	// position that scans them again: the cost grows as n^2.
	{"later alternative after a shared literal", "@start\nrule main = E $$\n" +
		"rule E = (E [x]* / \"\") \"x\" / \"x\" F\nrule F = E \";\"\n", ClassUnknown, ClassLinear},
	// On (^n b x^m y^n ! the second step of the growth of G at each of the n positions scans the
	// x^m: the cost grows as n*m.
	{"later alternative after a shared call", "@start\nrule main = G \"!\"\n" +
		"rule G = (G [x]* / \"\") \"(\" L / \"(\" G \"y\" / \"b\" [x]*\nrule L = \"(\" L / \"b\"\n", ClassUnknown, ClassLinear},
	// Controls: the later alternatives cannot begin with what the first step matches, or there are
	// none.
	{"later alternative with other first characters", "@start\nrule main = E $$\n" +
		"rule E = (E [x]* / \"\") \"a\" / \"b\" F\nrule F = E \";\"\n", ClassLinear, ClassLinear},
	{"later alternative that is not a call", "@start\nrule main = E $$\n" +
		"rule E = (E [x]* / \"\") \"x\" / \"y\"\n", ClassLinear, ClassLinear},
	{"no later alternative", "@start\nrule main = E $$\nrule E = (E [x]* / \"\") \"x\"\n", ClassLinear, ClassLinear},
}

// TestGrowStart checks that a growing unit of a leader whose first step goes on at its start is
// committing at that start against the later alternatives of its choice, whether or not the paths
// that go on contain a scanning source and with no shared call or literal removed. Without the
// check (the mutant) the witnesses are classified linear.
func TestGrowStart(t *testing.T) {
	defer func() { growStartOn = true }()
	for _, c := range growStartCases {
		growStartOn = true
		got := classifySource(t, c.src)
		if got.Class != c.class {
			t.Errorf("%s: class %v, want %v\nreasons %v", c.name, got.Class, c.class, got.Reasons)
		}
		growStartOn = false
		if got := classifySource(t, c.src).Class; got != c.without {
			t.Errorf("%s without the check: class %v, want %v", c.name, got, c.without)
		}
	}
}
