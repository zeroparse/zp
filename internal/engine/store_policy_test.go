package engine

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/codegentest"
)

// profileEvals returns the body evaluations of the rule that a profile counted, or -1.
func profileEvals(pr *core.Profile, rule string) int {
	for _, r := range pr.Snapshot().Rules {
		if r.Rule == rule {
			return r.Evals
		}
	}
	return -1
}

// TestMemoStorePolicy checks the store policy of "Evaluations of Rules" in spec/runtime/parsing.md
// on the closure and bytecode backends: of the evaluations of a memoized rule at a position outside
// every context of growths, only the first may leave no result in the memo, so the rule is evaluated
// as often as the policy allows, whatever the lookahead state, the values of the variables that it
// reads and the binding level of each call, for the rule and for its twin, with the eager switch
// on and off, with the sparse seen table, and in a stream.
func TestMemoStorePolicy(t *testing.T) {
	t.Parallel()
	for _, c := range codegentest.StorePolicyCases() {
		prog := mustCompile(t, c.Src)
		for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
			for _, recognize := range []bool{false, true} {
				if c.Stream && recognize {
					continue // recognition is not supported for stream parsing
				}
				deferred, stored := c.StorePolicyEvals(!recognize)
				var pr core.Profile
				o := ParseOptions{Backend: b, Recognize: recognize, Profiles: []*core.Profile{&pr}}
				var err error
				if c.Stream {
					err = prog.ParseStreamWith(c.Start, strings.NewReader(c.Input), func(*core.Node) error { return nil }, o)
				} else {
					_, err = prog.ParseWith(c.Start, c.Input, o)
				}
				if err != nil {
					t.Errorf("%s %v recognize %v: %v", c.Name, b, recognize, err)
					continue
				}
				got := profileEvals(&pr, c.Rule)
				switch {
				case c.Stream:
					// A stream stores every first call, which the policy allows: no evaluation
					// beyond those that a stored result leaves, and none fewer.
					if got < stored || got > deferred {
						t.Errorf("%s %v: %s evaluated %d times, want %d to %d", c.Name, b, c.Rule, got, stored, deferred)
					}
				case got != deferred:
					t.Errorf("%s %v recognize %v: %s evaluated %d times, want %d (%d if every first call stored)", c.Name, b, recognize, c.Rule, got, deferred, stored)
				}
			}
		}
	}
}

// TestMemoStorePolicyDocument checks the store policy for the Documents of the engine. A Document
// parse memoizes every call, the first call of a rule at a position as well (it never defers: FirstCall
// does nothing without DeferMemo, which only the ordinary parses set), so of the evaluations of a
// memoized rule at a position every one stores, which the policy allows, and the first parse of a
// Document evaluates the rule as often as the policy allows when the first call stores. A parse after
// an edit reuses the results that the edit leaves valid, so it evaluates the rule at most as often as
// that first parse did: reuse only answers calls, it never makes a call that stored evaluate again.
func TestMemoStorePolicyDocument(t *testing.T) {
	t.Parallel()
	for _, c := range codegentest.StorePolicyCases() {
		if c.Stream {
			continue
		}
		prog := mustCompile(t, c.Src)
		_, stored := c.StorePolicyEvals(true)
		for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
			var pr core.Profile
			o := ParseOptions{Backend: b, Profiles: []*core.Profile{&pr}}
			d, err := prog.NewDocumentWith(c.Start, c.Input, o)
			if err != nil {
				t.Errorf("%s %v: %v", c.Name, b, err)
				continue
			}
			if _, err := d.ParseWith(o); err != nil {
				t.Errorf("%s %v: %v", c.Name, b, err)
				continue
			}
			if got := profileEvals(&pr, c.Rule); got != stored {
				t.Errorf("%s %v: Document evaluated %s %d times, want %d (every call stores)", c.Name, b, c.Rule, got, stored)
			}
			// Replace the last unit of the input by itself and parse again.
			var pr2 core.Profile
			o.Profiles = []*core.Profile{&pr2}
			end := len([]rune(c.Input))
			if err := d.Edit(end-1, end, string([]rune(c.Input)[end-1:])); err != nil {
				t.Errorf("%s %v: %v", c.Name, b, err)
				continue
			}
			if _, err := d.ParseWith(o); err != nil {
				t.Errorf("%s %v after the edit: %v", c.Name, b, err)
				continue
			}
			if got := profileEvals(&pr2, c.Rule); got > stored {
				t.Errorf("%s %v: the parse after an edit evaluated %s %d times, more than the first parse (%d)", c.Name, b, c.Rule, got, stored)
			}
		}
	}
}
