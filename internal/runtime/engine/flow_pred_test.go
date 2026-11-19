package engine

import (
	"testing"

	"zeroparse.dev/zp/internal/testing/codegentest"
	"zeroparse.dev/zp/pkg/bytecode"
)

// TestFlowBodyPredicates runs grammars with predicates through both VM models with every rule that
// has a Flow body running it: the trees and the errors must be those of the match code and of the
// reference parser (the closure backend). (Flow bodies do not yet charge the units of the match code
// for every operation, so the charges are not compared; PRED runs the match code's evaluation, with
// its charges.) The predicates read
// captures of the rule's frame and of a repetition element's frame, read none, hold, fail and fail
// to evaluate, so the Flow bodies run PRED in each frame the match code runs its PRED in. The cases
// of predicate blocks whose rules have Flow bodies without bindings run too.
func TestFlowBodyPredicates(t *testing.T) {
	type grammar struct {
		name, src string
		inputs    []string
		// pred says that a Flow body of the grammar must run PRED.
		pred bool
	}
	gs := []grammar{
		{"rule frame", `rule main = items:item+ $$ => $items
rule item = n:[0-9]+ {text($n) != "0"} ","?`, []string{"1,2", "0", "1,0", "12,30,4", "", ",", "1,,2"}, true},
		{"element frame", `rule main = xs:(a:[a-z] {text($a) != "q"})* $$ => $xs`, []string{"", "abc", "aqb", "q", "zz9"}, true},
		{"no captures", `rule main = w ("," w)* $$
rule w = [a-z]+ {true}`, []string{"a", "a,bc", "", "a,", "1"}, true},
		{"failed evaluation", `rule main = "a" {1 / 0 == 0} "b" $$ / "a" "c" $$`, []string{"ab", "ac", "a", ""}, true},
		{"false", `rule main = x:"a"+ {len(text($x)) < 3} "b" $$ => $x`, []string{"ab", "aab", "aaab", "b", ""}, true},
	}
	for _, c := range codegentest.PredicateBlockCases() {
		gs = append(gs, grammar{c.Name, c.Block, c.Inputs, false})
	}
	preds := 0
	for _, g := range gs {
		n0 := preds
		old, flows, n := flowProgramsOf(t, g.src, func(string) bool { return true }, func(_ string, f *bytecode.Flow) {
			for _, in := range f.Code {
				if in.Op == bytecode.FlowPred {
					preds++
				}
			}
		})
		if g.pred && preds == n0 {
			t.Errorf("%s: no Flow body runs PRED", g.name)
		}
		if n == 0 {
			t.Logf("%s: no rule has a Flow body yet", g.name) // as for bindings, which have no instructions yet
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
