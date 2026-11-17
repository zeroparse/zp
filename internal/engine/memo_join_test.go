package engine

import (
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/engine/state"
	"zeroparse.dev/zp/internal/testing/codegentest"
)

// TestMemoJoinEvaluations checks that a store never narrows the calls that the entry of a key
// answers ("Evaluations of Rules" in spec/runtime/parsing.md): on every backend, with and without
// limit controls, the rule k of each family of codegentest.MemoJoinCases is evaluated as often in
// every run of the family, however many calls of k the run makes.
func TestMemoJoinEvaluations(t *testing.T) {
	t.Parallel()
	for _, c := range codegentest.MemoJoinCases() {
		for _, run := range c.Runs {
			prog := mustCompile(t, run.Src)
			for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
				for _, lim := range []core.Limits{{}, {Compute: 1 << 40}} {
					if lim.Compute != 0 && !state.BudgetControlsEnabled {
						continue // the reference build rejects a parse with a limit
					}
					for _, recognize := range []bool{false, true} {
						var pr core.Profile
						if _, err := prog.ParseWith("s", run.Input, ParseOptions{Backend: b, Limits: lim, Recognize: recognize, Profiles: []*core.Profile{&pr}}); err != nil {
							t.Fatalf("%s %q: %v", c.Name, run.Input, err)
						}
						evals := -1
						for _, r := range pr.Snapshot().Rules {
							if r.Rule == "k" {
								evals = r.Evals
							}
						}
						want := c.Evals
						if recognize {
							want = c.Recognized
						}
						if evals != want {
							t.Errorf("%s (%d units of input) %v limits %v recognize %v: k evaluated %d times, want %d", c.Name, len(run.Input), b, lim.Compute != 0, recognize, evals, want)
						}
					}
				}
			}
		}
	}
}
