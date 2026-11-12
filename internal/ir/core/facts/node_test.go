package facts_test

import (
	"slices"
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	"zeroparse.dev/zp/internal/factstest"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/ir/core/build"
	"zeroparse.dev/zp/internal/ir/core/facts"
)

// TestNodeQueriesMatchRuleFacts checks the node queries on the body of every rule of every corpus
// grammar against the rule facts: LeftCalls is the rule's LeftCalls (LeftCallsIns with token
// insertion), and a rule off every left-recursion cycle succeeds exactly when its body does.
// The mutated control flips one expectation and must be reported, so that the comparison is
// known to be able to fail.
func TestNodeQueriesMatchRuleFacts(t *testing.T) {
	t.Parallel()
	for _, path := range factstest.Corpus {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			m, f := moduleFacts(t, path)
			for _, r := range m.Rules {
				if r == nil || r.Twin != nil {
					continue
				}
				if got, want := f.LeftCalls(r.Body, false), f.Rules[r.ID].LeftCalls; !slices.Equal(got, want) {
					t.Errorf("rule %s: LeftCalls %v, rule fact %v", r.Name, got, want)
				}
				if got, want := f.LeftCalls(r.Body, true), f.Rules[r.ID].LeftCallsIns; !slices.Equal(got, want) {
					t.Errorf("rule %s: LeftCalls with insertion %v, rule fact %v", r.Name, got, want)
				}
				if !f.Rules[r.ID].Cyclic() && f.Succeeds(r.Body) != f.Rules[r.ID].Succeeds {
					t.Errorf("rule %s: Succeeds %v, rule fact %v", r.Name, f.Succeeds(r.Body), f.Rules[r.ID].Succeeds)
				}
				if f.Succeeds(r.Body) && f.Fails(r.Body) {
					t.Errorf("rule %s: body both always succeeds and always fails", r.Name)
				}
			}
		})
	}
}

// TestNodeQueriesNegativeControl runs the same comparison on facts whose rule data is changed and
// requires a difference.
func TestNodeQueriesNegativeControl(t *testing.T) {
	t.Parallel()
	for _, path := range factstest.Corpus {
		m, f := moduleFacts(t, path)
		for _, r := range m.Rules {
			if r == nil || r.Twin != nil || len(f.Rules[r.ID].LeftCalls) == 0 {
				continue
			}
			mutated := slices.Clone(f.Rules[r.ID].LeftCalls)
			mutated[0]++
			if slices.Equal(f.LeftCalls(r.Body, false), mutated) {
				t.Fatalf("%s: the comparison cannot tell a changed LeftCalls from the real one", path)
			}
			return
		}
	}
	t.Fatal("no corpus rule has left calls")
}

func moduleFacts(t *testing.T, path string) (*core.Module, *facts.Facts) {
	t.Helper()
	g, err := compiletest.Parse(factstest.Read(t, path))
	if err != nil {
		t.Fatal(err)
	}
	prog, err := compiletest.Compile(g, compile.Options{})
	if err != nil {
		t.Fatal(err)
	}
	m, err := build.CoreFrom(prog, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range m.Rules {
		r.Policy = core.Policy{}
	}
	return m, facts.Compute(m, nil)
}
