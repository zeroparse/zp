package engine

import (
	"testing"

	"zeroparse.dev/zp/internal/gentest"
	"zeroparse.dev/zp/internal/limits"
)

// TestMemoLookaheadStateEvaluations checks that calls that alternate between lookahead states
// evaluate a rule at most three times per variant, whether or not its result has recovered errors:
// an entry computed outside a lookahead answers the calls in both states.
func TestMemoLookaheadStateEvaluations(t *testing.T) {
	t.Parallel()
	for _, c := range gentest.MemoLookaheadCases() {
		prog := mustCompile(t, c.Src)
		for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
			for _, lim := range []limits.Limits{{}, {Compute: 1 << 40}} {
				for _, recognize := range []bool{false, true} {
					var pr Profile
					// The parses with recovered errors end with a syntax error.
					_, _ = prog.ParseWith("s", c.Input, ParseOptions{Backend: b, Limits: lim, Recognize: recognize, Trace: pr.Trace})
					evals := -1
					for _, r := range pr.Rules {
						if r.Rule == "a" {
							evals = r.Evals
						}
					}
					want := c.Evals
					if recognize {
						want = c.Recognized
					}
					if evals != want {
						t.Errorf("%s %v limits %v recognize %v: a evaluated %d times, want %d", c.Name, b, lim.Compute != 0, recognize, evals, want)
					}
				}
			}
		}
	}
}
