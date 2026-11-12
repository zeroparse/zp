package compile

import (
	"strings"
	"testing"
)

// deadWitness is a grammar whose first alternative of E never succeeds: the normal view reduces
// !"b" "b" to a part that no evaluation passes. Each step of the growth of E at a position
// evaluates that alternative again, and it scans W after the "a" before it fails, while E "x" reads
// the growing result. On a (ab)^k y x^k the steps scan W k times, so the cost grows as n^2.
const deadWitness = "rule s = E $$\n" +
	"rule E = \"a\" W !\"b\" \"b\" / E \"x\" / \"a\" W \"y\"\n" +
	"rule W = (\"a\" #cut \"b\")*\n"

// TestDeadUnitRejected checks that a unit that never succeeds is compared with its continuations
// on the paths that it evaluates before it fails, so that it decides none of them for want of
// paths, and that the reason names the path. Without dead paths (the negative control) the unit
// has no path, decides every continuation, and the grammar is accepted as linear.
func TestDeadUnitRejected(t *testing.T) {
	defer restoreExclusionDefaults()
	restoreExclusionDefaults()
	c := classifySource(t, deadWitness)
	if c.Class != ClassPoly {
		t.Fatalf("class %v, want %v", c.Class, ClassPoly)
	}
	const why = "the alternative does not part from it on a path on which it cannot succeed"
	if len(c.Causes) == 0 || !strings.Contains(c.Causes[0].Message, why) {
		t.Errorf("causes %+v do not say %q", c.Causes, why)
	}
	deadPathsOn = false
	if got := classifySource(t, deadWitness).Class; got != ClassLinear {
		t.Errorf("without dead paths: class %v, want %v (the negative control)", got, ClassLinear)
	}
}

// TestDeadUnitVariants checks grammars near deadWitness. A unit that can succeed ends the growth in
// its first step, and a unit that never succeeds outside a growth parts from a continuation that
// can go on at the point where it fails. The variant whose repetition has no cut stays rejected:
// the steps of the growth evaluate it again as they do the cut one, and the classifier rejects the
// unit that can succeed and fails at "c" in the same place.
func TestDeadUnitVariants(t *testing.T) {
	defer restoreExclusionDefaults()
	restoreExclusionDefaults()
	const cutW = "rule W = (\"a\" #cut \"b\")*\n"
	for _, c := range []struct {
		name, e, w string
		want       ComplexityClass
	}{
		{"can succeed", `"a" W / E "x"`, cutW, ClassLinear},
		{"no growth", `"a" W !"b" "b" / "a" W "y"`, cutW, ClassLinear},
		{"no cut", `"a" W !"b" "b" / E "x" / "a" W "y"`, "rule W = \"ab\"*\n", ClassPoly},
		{"no cut, can succeed", `"a" W "c" / E "x" / "a" W "y"`, "rule W = \"ab\"*\n", ClassPoly},
	} {
		src := "rule s = E $$\nrule E = " + c.e + "\n" + c.w
		if got := classifySource(t, src).Class; got != c.want {
			t.Errorf("%s: class %v, want %v", c.name, got, c.want)
		}
	}
}

// TestDeadNotDecisive checks the comparison of two paths that reach, after a symbol passed alike, a
// part that no evaluation passes: neither can go on there, so it is not decisive, while a dead
// part against a path that can go on is. Without dead paths (the negative control) the dead paths
// vanish and the pair counts as decided.
func TestDeadNotDecisive(t *testing.T) {
	defer restoreExclusionDefaults()
	restoreExclusionDefaults()
	const rules = "rule z1 = !\"b\" \"b\"\nrule z2 = !\"b\" \"b\"\nrule w = \"w\"\n"
	for _, c := range []struct {
		name, p, q string
		on, off    bool
		tag        string
	}{
		{"both dead", `"a" z1`, `"a" z2 "z"`, false, true, "dead"},
		{"one dead", `"a" z1`, `"a" "z"`, true, true, ""},
		{"both dead, direct", `"a" !"b" "b"`, `"a" !"b" "b" "z"`, false, true, "dead"},
		{"own copy", `"a" w !"b" "b"`, `"a" w !"b" "b"`, false, true, "dead"},
		{"one dead, direct", `"a" !"b" "b"`, `"a" "z"`, true, true, ""},
		{"one dead, direct, continuation side", `"a" "z"`, `"a" !"b" "b"`, true, true, ""},
	} {
		src := rules + "rule p = " + c.p + "\nrule q = " + c.q + "\n"
		deadPathsOn = true
		ok, tag := comparePaths(t, src, true, 3)
		if ok != c.on || tag != c.tag {
			t.Errorf("%s: decided %v (%q), want %v (%q)", c.name, ok, tag, c.on, c.tag)
		}
		deadPathsOn = false
		if ok, _ := comparePaths(t, src, true, 3); ok != c.off {
			t.Errorf("%s without dead paths: decided %v, want %v", c.name, ok, c.off)
		}
	}
}
