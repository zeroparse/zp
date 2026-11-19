package engine

import (
	"testing"

	"zeroparse.dev/zp/internal/testing/codegentest"
	"zeroparse.dev/zp/pkg/bytecode"
)

// flowGrammar is a grammar of the tests of Flow bodies, with its inputs. want says that a Flow body of
// the grammar must run the operation the test is about.
type flowGrammar struct {
	name, src string
	inputs    []string
	want      bool
}

// checkFlowGrammars runs the grammars gs through both VM models with every rule that has a Flow body
// running it: the trees and the errors must be those of the match code and of the reference parser
// (the closure backend), and each grammar that wants it must run op from a Flow body. (Flow bodies
// do not yet charge the units of the match code for every operation, so the charges are not
// compared; PRED and BIND run the match code's evaluation, with its charges, and CUT and SETCUT
// charge nothing.)
func checkFlowGrammars(t *testing.T, gs []flowGrammar, op bytecode.FlowOp) {
	t.Helper()
	for _, g := range gs {
		ran := 0
		old, flows, n := flowProgramsOf(t, g.src, func(string) bool { return true }, func(_ string, f *bytecode.Flow) {
			for _, in := range f.Code {
				if in.Op == op {
					ran++
				}
			}
		})
		if g.want && ran == 0 {
			t.Errorf("%s: no Flow body runs %v", g.name, op)
		}
		if n == 0 {
			t.Logf("%s: no rule has a Flow body yet", g.name)
			continue
		}
		for _, in := range g.inputs {
			rt, re, _ := parseChargedWith(t, old, in, Closure)
			for _, b := range []Backend{Bytecode, BytecodeIterative} {
				ot, oe, _ := parseChargedWith(t, old, in, b)
				ft, fe, _ := parseChargedWith(t, flows, in, b)
				if ot != ft || oe != fe {
					t.Errorf("%s, %v, input %q:\n match code %s %s\n flow       %s %s", g.name, b, in, ot, oe, ft, fe)
				}
				if ft != rt || fe != re {
					t.Errorf("%s, %v, input %q:\n reference %s %s\n flow      %s %s", g.name, b, in, rt, re, ft, fe)
				}
			}
		}
	}
}

// predicateBlocks returns the cases of predicate blocks as grammars of checkFlowGrammars that need not
// run the operation.
func predicateBlocks() []flowGrammar {
	var gs []flowGrammar
	for _, c := range codegentest.PredicateBlockCases() {
		gs = append(gs, flowGrammar{c.Name, c.Block, c.Inputs, false})
	}
	return gs
}

// TestFlowBodyPredicates runs grammars with predicates from Flow bodies (checkFlowGrammars). The
// predicates read captures of the rule's frame and of a repetition element's frame, read none, hold,
// fail and fail to evaluate, so the Flow bodies run PRED in each frame the match code runs its PRED
// in. The cases of predicate blocks run too.
func TestFlowBodyPredicates(t *testing.T) {
	checkFlowGrammars(t, append([]flowGrammar{
		{"rule frame", `rule main = items:item+ $$ => $items
rule item = n:[0-9]+ {text($n) != "0"} ","?`, []string{"1,2", "0", "1,0", "12,30,4", "", ",", "1,,2"}, true},
		{"element frame", `rule main = xs:(a:[a-z] {text($a) != "q"})* $$ => $xs`, []string{"", "abc", "aqb", "q", "zz9"}, true},
		{"no captures", `rule main = w ("," w)* $$
rule w = [a-z]+ {true}`, []string{"a", "a,bc", "", "a,", "1"}, true},
		{"failed evaluation", `rule main = "a" {1 / 0 == 0} "b" $$ / "a" "c" $$`, []string{"ab", "ac", "a", ""}, true},
		{"false", `rule main = x:"a"+ {len(text($x)) < 3} "b" $$ => $x`, []string{"ab", "aab", "aaab", "b", ""}, true},
	}, predicateBlocks()...), bytecode.FlowPred)
}

// TestFlowBodyBindings runs grammars with bindings from Flow bodies (checkFlowGrammars). The bindings
// read captures of the rule's frame and of an element's frame, are undone by the backtracking of a
// choice, an option and a repetition, fail to evaluate, and are restored when the rule that bound
// them returns, so the Flow bodies run BIND where the match code runs its BIND. The cases of
// predicate blocks, most of which bind, run too.
func TestFlowBodyBindings(t *testing.T) {
	checkFlowGrammars(t, append([]flowGrammar{
		{"rule frame", `var n int
rule main = items:item+ $$ => $items
rule item = d:[0-9]+ {n := len(text($d))} {n < 3} ","?`, []string{"1,22", "333", "1,333", "", "4444,1"}, true},
		{"element frame", `var s string
rule main = xs:(a:[a-z] {s := text($a)} {s != "q"})* $$ => $xs`, []string{"", "abc", "aqb", "q"}, true},
		{"choice rollback", `var x int
rule main = {x := 0} ("a" {x := 1} "!" / "a" {x == 0} "?") $$`, []string{"a!", "a?", "a", "b"}, true},
		{"repetition rollback", `var k int
rule main = {k := 0} ("a" {k := k + 1} "b")* "a" {k == 1} $$`, []string{"aba", "a", "ababa", "abab", ""}, true},
		{"failed evaluation", `var x int
rule main = "a" {x := 1 / 0} "b" $$ / "a" "c" $$`, []string{"ab", "ac", ""}, true},
		{"restored by the rule", `var x int
rule main = {x := 1} inner {x == 1} $$
rule inner = "a" {x := 2} {x == 2}`, []string{"a", "", "aa"}, true},
	}, predicateBlocks()...), bytecode.FlowBind)
}

// TestFlowBodyCuts runs grammars with cuts from Flow bodies (checkFlowGrammars). A body with a cut
// reads the cut flag of its rule call on entry and publishes it on return (CUT, SETCUT); a choice
// cuts its own flag, so a cut after a partial match fails the choice, a cut of a called rule does not
// reach its caller, and the cut of an inner choice, of a repetition element or of a lookahead is
// scoped to it. (The rule call clears the flag on entry and restores it on return, so what a rule
// body publishes is not observable from the rule table.) The cases of predicate blocks run too.
func TestFlowBodyCuts(t *testing.T) {
	checkFlowGrammars(t, append([]flowGrammar{
		{"rule choice", `rule main = "a" #cut "b" $$ / "a" "c" $$ / "d" $$`, []string{"ab", "ac", "a", "d", ""}, true},
		{"cut after a partial match", `rule main = "a" "b" #cut "c" $$ / "a" "b" "d" $$ / "a" $$`, []string{"abc", "abd", "a", "ab", ""}, true},
		{"called rule", `rule main = item "x" $$ / "k" "z" $$
rule item = "k" #cut "y" / "k"`, []string{"kyx", "kx", "kz", "k", "ky"}, true},
		{"nested choice", `rule main = x:("a" #cut "b" / "a" "c") "!" $$ / "a" "c" "?" $$ => $x`, []string{"ab!", "ac!", "ac?", "a", ""}, false},
		{"element", `rule main = ("a" #cut "b" / "a" "c")* $$`, []string{"abab", "ac", "aa", ""}, false},
		{"lookahead", `rule main = !("a" #cut "b" / "a") "a" $$ / "ab" $$ / "q" $$`, []string{"ab", "a", "q", "ac"}, false},
	}, predicateBlocks()...), bytecode.FlowCut)
}
