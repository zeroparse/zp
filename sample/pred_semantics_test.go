package sample_test

import (
	"testing"

	"zeroparse.dev/zp/sample"
)

// predicateSemantics are grammars whose predicates the generator must decide as the parser does. Each
// predicate reads a part of the value that the generator tracks: the elements and captures of lists,
// the captures of a rule's node, the value of a hidden rule, positions in code points, captures made
// inside a lookahead, variables, type aliases and the value of an action, which it does not know.
var predicateSemantics = []struct {
	name, src string
	// unknown is set where the predicate reads what the generator cannot tell: the parser decides, so
	// some candidates may be rejected.
	unknown bool
}{
	{"elements and captures of a list", `
rule main = i:ind {foldl(0, $i.$segs, (a, s) => a + len(s.$sp)) >= 1 && len($i.$segs) == 2} "x" $$
rule ind = segs:(sp:" "* "\t")*`, false},
	{"elements of a list of strings", `
rule main = xs:("a" / "bc")* {foldl(0, $xs, (n, x) => n + len(text(x))) == 4} $$`, false},
	{"captures of a rule's node", `
rule main = r:pair {$r.$a.End - $r.$a.Start == $r.$b.End - $r.$b.Start && $r.$a.End > $r.$a.Start} $$
rule pair = a:"x"* "-" b:"y"*`, false},
	{"value of a hidden rule", `
rule main = x:_h {len($x) == 2} $$
rule _h = c:("a" / "b")*`, false},
	{"positions in code points", `
rule main = s:("é" / "a")* {$s.End - $s.Start == 2 && len(text($s)) == 2} $$`, false},
	{"a capture inside a lookahead", `
rule main = &(x:"ab"*) {len($x) == 2} ("ab" / "a" / "c")* $$`, false},
	{"variables", `
var n int
rule main = {n := 0} ("a" {n := n + 1} / "b")* {n == 2} $$`, false},
	{"a capture that did not match", `
rule main = x:"a"? y:"b"? ("c" / "d")* {$x == nil && $y != nil && len(text($x)) == 0} $$`, false},
	{"a type alias", `
type S ~ node[string]
rule main = x:("a" / "b") {text($x.(S)) == "a"} ("c" / "d")* $$`, false},
	{"a rule declared with node[string]", `
rule main = x:a {text($x.(node[string])) == "a"} ("c" / "d")* $$
rule a node[string] = "a" / "b"`, false},
	{"a rule declared with _", `
rule main = x:a {text($x.(node[string])) == "a"} ("c" / "d")* $$
rule a _ = "a" / "b"`, false},
	{"a rule declared with a join", `
rule main = x:a {text($x.(node[string])) == "a"} ("c" / "d")* $$
rule a (node[string] | int) = "a" / "b"`, false},
	{"a rule declared with a named node type", `
type Num = node[string]
rule main = x:a {text($x.(Num)) == "a"} ("c" / "d")* $$
rule a Num = "a" / "b"`, true},
	{"a rule declared with an alias of a named node type", `
type Num = node[string]
type A ~ Num
rule main = x:a {text($x.(Num)) == "a"} ("c" / "d")* $$
rule a A = "a" / "b"`, true},
	{"the value of an action", `
rule main = x:num {$x == 3} $$
rule num int = [0-9] => 3`, true},
}

// TestPredicateSemantics checks that the generator decides predicates as the parser does: it finds
// inputs (a predicate that it takes as failing where it holds would leave none), and where it can tell
// the value, the parser rejects none of its candidates (an undecided or wrongly passed predicate would
// let some through).
func TestPredicateSemantics(t *testing.T) {
	for _, c := range predicateSemantics {
		t.Run(c.name, func(t *testing.T) {
			p := compile(t, c.src)
			g, err := sample.New(p, sample.WithSeed(1))
			if err != nil {
				t.Fatal(err)
			}
			inputs, err := g.Generate(3)
			if err != nil || len(inputs) != 3 {
				t.Fatalf("%q, %v (stats %+v)", inputs, err, g.Stats())
			}
			checkParse(t, p, inputs)
			if s := g.Stats(); !c.unknown && s.Rejected != 0 {
				t.Errorf("the parser rejected %d of %d candidates: the generator did not decide the predicate (inputs %q)", s.Rejected, s.Candidates, inputs)
			}
		})
	}
}
