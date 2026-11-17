package complexity

import "testing"

// copyWitness builds the grammars of the copies that a decision by a negative lookahead passes.
// The first alternative of W fails at !K wherever the rest of the continuation matches K: the
// later alternative L, after the end of W and of E, goes on with "c" L and "z" in the body of W.
// The continuation re-evaluates that rest, and a copy of the text of L in it costs as much input
// as L consumed. On (^n bc (^n bq, K fails at the end at every level of the nesting, so the
// first alternative goes on and makes the copy at each of the n levels, and the cost grows as
// n^2. tail is what follows "c" in the first alternative of W, and rules are added at the end.
func copyWitness(tail, rules string) string {
	return "@start\nrule main = E \"c\" L \"z\"\nrule E = \"(\" W / [b]+\n" +
		"rule W = !(L \"c\" L \"z\") E \"c\" " + tail + " / L\nrule L = \"(\" L / \"b\"\n" + rules
}

// copyCases are the grammars whose classes depend on where a path of the strict analysis ends at
// a copy. without is the class with the copy passed over (copyStopsOn off).
var copyCases = []struct {
	name, src string
	class     ComplexityClass
	without   ComplexityClass
}{
	{"predicate", copyWitness(`x:L {len(text($x)) >= 0} "z"`, ""), ClassUnknown, ClassLinear},
	{"predicate in a called rule", copyWitness(`M "z"`, "rule M = x:L {len(text($x)) >= 0}\n"), ClassUnknown, ClassLinear},
	{"binding", "var y string = \"\"\n" + copyWitness(`x:L {y := text($x) + "a"} "z"`, ""), ClassUnknown, ClassLinearEnv},
	{"inline action", copyWitness(`(x:L => text($x) + text($x)) "z"`, ""), ClassUnknown, ClassLinear},
	{"action of a called rule", copyWitness(`M "z"`, "rule M string = x:L => text($x) + text($x)\n"), ClassUnknown, ClassLinear},
	{"action of a rule that a later call reaches", copyWitness(`N`, "rule N = M \"z\"\nrule M string = x:L => text($x) + text($x)\n"), ClassUnknown, ClassLinear},
	// The continuation leaves the body of W, whose action copies its text, before it reaches
	// "c" L "z". The copy also holds the nested values of W, which the copy check finds.
	{"action at the end of the rule of the unit", "@start\nrule main = E \"c\" L \"z\"\nrule E = \"(\" W / [b]+\n" +
		"rule W string = x:(!(L \"c\" L \"z\") E \"c\" L \"z\" / L) => text($x) + text($x)\nrule L = \"(\" L / \"b\"\n", ClassUnknown, ClassPoly},
	// Controls: a predicate that compares with a literal, a binding of the text of a capture and an
	// action that returns a constant make no copy that the input can make large.
	{"literal comparison", copyWitness(`x:L {text($x) == "zz"} "z"`, ""), ClassLinear, ClassLinear},
	{"literal comparison in a called rule", copyWitness(`M "z"`, "rule M = x:L {text($x) == \"zz\"}\n"), ClassLinear, ClassLinear},
	{"binding of a text", "var y string = \"\"\n" + copyWitness(`x:L {y := text($x)} "z"`, ""), ClassLinearEnv, ClassLinearEnv},
	{"constant action", copyWitness(`M "z"`, "rule M string = L => \"q\"\n"), ClassLinear, ClassLinear},
	// The action of the start rule runs once, where the parse ends.
	{"action of the start rule", "@start\nrule main string = x:(E \"c\" L \"z\") => text($x) + text($x)\nrule E = \"(\" W / [b]+\n" +
		"rule W = !(L \"c\" L \"z\") E \"c\" L \"z\" / L\nrule L = \"(\" L / \"b\"\n", ClassLinear, ClassLinear},
}

// TestCopyStopsPath checks that a path of the strict analysis ends at a predicate, a binding or an
// action whose copying operation is a scanning source, instead of passing over it as over a
// predicate that copies nothing, so that a negative lookahead whose body the rest of the
// continuation matches through the copy does not decide the unit. Without the stop (the mutant)
// the witnesses are classified linear.
func TestCopyStopsPath(t *testing.T) {
	defer restoreExclusionDefaults()
	for _, c := range copyCases {
		restoreExclusionDefaults()
		got := classifySource(t, c.src)
		if got.Class != c.class {
			t.Errorf("%s: class %v, want %v\ncauses %v\nreasons %v", c.name, got.Class, c.class, got.Causes, got.Reasons)
		}
		copyStopsOn = false
		if got := classifySource(t, c.src).Class; got != c.without {
			t.Errorf("%s without the stop: class %v, want %v", c.name, got, c.without)
		}
	}
}
