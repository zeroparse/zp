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

// growNullableCases are grammars with a growing unit of a leader whose first step can go on at
// its start through a path that matches without consuming input. without is the class with
// growNullableOn off.
var growNullableCases = []struct {
	name, src string
	class     ComplexityClass
	without   ComplexityClass
}{
	// On x^n ; the path "" !";" matches at each x without consuming it, so in a later step that
	// fails after its read of the seed the later alternative "x" F consumes the x and starts a
	// growth at the next position, which scans the x again: the cost grows as n^2. The lookahead
	// examines ;, which the later alternative cannot begin with, but the path does not begin
	// with it.
	{"path that ends with a negative lookahead", "@start\nrule main = E $$\n" +
		"rule E = ((E [x]* / \"\") !\";\" / \"x\" F) \"x\"\nrule F = E \",\"\n", ClassUnknown, ClassLinear},
	{"path with a negative lookahead and an empty literal", "@start\nrule main = E $$\n" +
		"rule E = ((E [x]* / \"\") !\";\" \"\" / \"x\" F) \"x\"\nrule F = E \",\"\n", ClassUnknown, ClassLinear},
	// Controls: the path consumes ; before the later alternative could consume anything, or the
	// unit cannot fail, so the later alternative is never evaluated.
	{"path that consumes", "@start\nrule main = E $$\n" +
		"rule E = ((E [x]* / \"\") \";\" / \"x\" F) \"x\"\nrule F = E \",\"\n", ClassLinear, ClassLinear},
	{"unit that cannot fail", "@start\nrule main = E $$\n" +
		"rule E = ((E [x]* / \"\") / \"x\" F) \"x\"\nrule F = E \",\"\n", ClassLinear, ClassLinear},
}

// TestGrowNullable checks that a path on which the first step of a growing unit of a leader goes
// on at its start and that can match without consuming input counts as beginning with any
// character, whatever its lookaheads examine. Without the rule (the mutant) the witnesses are
// classified linear.
func TestGrowNullable(t *testing.T) {
	defer func() { growNullableOn = true }()
	for _, c := range growNullableCases {
		growNullableOn = true
		got := classifySource(t, c.src)
		if got.Class != c.class {
			t.Errorf("%s: class %v, want %v\nreasons %v", c.name, got.Class, c.class, got.Reasons)
		}
		growNullableOn = false
		if got := classifySource(t, c.src).Class; got != c.without {
			t.Errorf("%s without the rule: class %v, want %v", c.name, got, c.without)
		}
	}
}
