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

// storePolicyRun parses case c in the way of a test: an ordinary parse or recognition, or a stream,
// on backend b, and returns the body evaluations of the case's rule.
func storePolicyRun(t *testing.T, prog *Program, c codegentest.StorePolicyCase, b Backend, recognize bool) int {
	t.Helper()
	var pr core.Profile
	o := ParseOptions{Backend: b, Recognize: recognize, Profiles: []*core.Profile{&pr}}
	var err error
	if c.Stream {
		err = prog.ParseStreamWith(c.Start, strings.NewReader(c.Input), func(*core.Node) error { return nil }, o)
	} else {
		_, err = prog.ParseWith(c.Start, c.Input, o)
	}
	if err != nil {
		t.Fatalf("%s %v recognize %v: %v", c.Name, b, recognize, err)
	}
	return profileEvals(&pr, c.Rule)
}

// TestMemoStorePolicy checks the store policy of "Evaluations of Rules" in spec/runtime/parsing.md
// on the closure and bytecode backends: of the evaluations of a memoized rule at a position outside
// every context of growths, only the first may leave no result in the memo. So for the rule and for
// its twin, whatever the lookahead state, the values of the variables that it reads and the binding
// level of each call, the evaluations of the rule are at least those that a memo storing every first
// call leaves (every call that a stored result answers is no evaluation) and at most those that a
// memo deferring every first call leaves. The cases cover the eager switch on and off, the sparse seen
// table and a stream.
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
				if got := storePolicyRun(t, prog, c, b, recognize); got < stored || got > deferred {
					t.Errorf("%s %v recognize %v: %s evaluated %d times, want %d to %d", c.Name, b, recognize, c.Rule, got, stored, deferred)
				}
			}
		}
	}
}

// TestMemoStorePolicyReferenceCounts pins what the reference implementation does within the policy,
// which the policy does not require and TestMemoStorePolicy does not: an ordinary parse defers
// the first call of a rule at a position until repeated calls exceed a sixteenth of its calls (the
// adaptive switch of FirstCall), so it evaluates the rule exactly as often as the model of
// codegentest.StorePolicyCase says; a stream memoizes every first call. A change of that choice,
// for example one that stores every first call, is allowed by the policy and updates this test.
func TestMemoStorePolicyReferenceCounts(t *testing.T) {
	t.Parallel()
	for _, c := range codegentest.StorePolicyCases() {
		prog := mustCompile(t, c.Src)
		for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
			for _, recognize := range []bool{false, true} {
				if c.Stream && recognize {
					continue
				}
				deferred, stored := c.StorePolicyEvals(!recognize)
				want := deferred
				if c.Stream {
					want = stored // a stream stores every first call
				}
				if got := storePolicyRun(t, prog, c, b, recognize); got != want {
					t.Errorf("%s %v recognize %v: %s evaluated %d times, the reference implementation %d", c.Name, b, recognize, c.Rule, got, want)
				}
			}
		}
	}
}

// TestMemoStorePolicyDocument checks the store policy for the Documents of the engine: the first
// parse of a Document evaluates a rule at least as often as a memo storing every first call and at
// most as often as one deferring each, as TestMemoStorePolicy says of ordinary parses. (The
// reference Document memoizes every call, the first of a rule at a position as well, and
// TestMemoStorePolicyReferenceCounts pins that.) The parse after an edit reuses what the edit leaves
// valid, which the policy leaves out, so the edit check is only a sanity bound: it evaluates the rule
// at most as often as the first parse did.
func TestMemoStorePolicyDocument(t *testing.T) {
	t.Parallel()
	for _, c := range codegentest.StorePolicyCases() {
		if c.Stream {
			continue
		}
		prog := mustCompile(t, c.Src)
		deferred, stored := c.StorePolicyEvals(true)
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
			first := profileEvals(&pr, c.Rule)
			if first < stored || first > deferred {
				t.Errorf("%s %v: Document evaluated %s %d times, want %d to %d", c.Name, b, c.Rule, first, stored, deferred)
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
			if got := profileEvals(&pr2, c.Rule); got > first {
				t.Errorf("%s %v: the parse after an edit evaluated %s %d times, more than the first parse (%d)", c.Name, b, c.Rule, got, first)
			}
		}
	}
}

// TestMemoStorePolicyDocumentReferenceCounts pins that the reference Document memoizes every call, the
// first of a rule at a position as well, so that its first parse evaluates a rule exactly as often as
// a memo storing every first call. The policy does not require it.
func TestMemoStorePolicyDocumentReferenceCounts(t *testing.T) {
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
			if err == nil {
				_, err = d.ParseWith(o)
			}
			if err != nil {
				t.Errorf("%s %v: %v", c.Name, b, err)
				continue
			}
			if got := profileEvals(&pr, c.Rule); got != stored {
				t.Errorf("%s %v: Document evaluated %s %d times, the reference Document %d (every call stores)", c.Name, b, c.Rule, got, stored)
			}
		}
	}
}
