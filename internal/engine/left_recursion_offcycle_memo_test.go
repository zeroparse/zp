package engine

import (
	"testing"
)

// offCycleMemoCases are grammars where rule w, on no left-recursion cycle, is called at position 0
// while the leader e grows there, in two steps of its growth, and outside the growth. "Evaluations of
// Rules" in spec/runtime/parsing.md puts such a call in no context of growths: the memo answers and
// stores it as any call outside a growth. w calls a rule, and is called from several places, so a
// parse memoizes it; the first call of w at a position is evaluated without the memo and the second
// is evaluated and stored, so every later call there is answered by the memo, in either step of the
// growth or outside it. In "growth first", the result is stored in the first step of the growth and
// answers the second step and the call after the growth; a memo that kept it apart by the growing
// result, or did not store it inside a growth, evaluates w again. In "growth last", it is stored
// before the growth and answers both steps; a memo that kept it apart by the growing result
// evaluates w again in each step.
var offCycleMemoCases = []struct {
	name, src, input string
	evals, hits      int // of w
}{
	{"growth first", "@start\nrule main = e \"!\" / w \"?\"\nrule e = e \"+\" w / w \"#\" / w\nrule w = l+\nrule l = [a-z]", "a?", 2, 3},
	{"growth last", "@start\nrule main = w \"#\" / w \"%\" / e \"!\" / w \"?\"\nrule e = e \"+\" w / w \"#\" / w\nrule w = l+\nrule l = [a-z]", "a?", 2, 5},
}

// TestLeftRecursionOffCycleMemo checks on every backend that the calls of a rule on no cycle of the
// growths at their position are memoized as calls outside a growth (offCycleMemoCases).
func TestLeftRecursionOffCycleMemo(t *testing.T) {
	t.Parallel()
	for _, c := range offCycleMemoCases {
		prog := mustCompile(t, c.src)
		for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
			for _, rec := range []bool{false, true} {
				var pr Profile
				if _, err := prog.ParseWith("", c.input, ParseOptions{Backend: b, Recognize: rec, Trace: pr.Trace}); err != nil {
					t.Fatalf("%s %s recognize=%v: %v", c.name, b, rec, err)
				}
				var w *RuleProfile
				for _, r := range pr.Rules {
					if r.Rule == "w" {
						w = r
					}
				}
				if w == nil || w.Evals != c.evals || w.MemoHits != c.hits {
					t.Errorf("%s %s recognize=%v: w %+v, want %d evaluations and %d memo answers", c.name, b, rec, w, c.evals, c.hits)
				}
			}
		}
	}
}
