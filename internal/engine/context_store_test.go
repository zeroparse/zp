package engine

import (
	"fmt"
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/engine/core"
	"zeroparse.dev/zp/internal/gentest"
	"zeroparse.dev/zp/internal/limits"
)

// contextStoreGrowth parses the shape at the two gentest.ContextStoreSizes on the backend and
// returns the evaluations of B at its busiest position and of all rules, at each size. The parses
// have no compute limit: a context that the store cannot index exceeds the default one.
func contextStoreGrowth(t *testing.T, prog *Program, s gentest.DependencyMemoShape, o ParseOptions) (b, all [2]int) {
	t.Helper()
	for i, m := range gentest.ContextStoreSizes {
		var pr Profile
		o.Trace = pr.Trace
		o.Limits = limits.Limits{Compute: limits.Unlimited}
		if _, err := prog.ParseWith("", s.Input(m), o); err != nil {
			t.Fatalf("%s %s m=%d: %v", s.Name, o.Backend, m, err)
		}
		all[i] = pr.Evals
		for _, r := range pr.Rules {
			if r.Rule == "B" {
				b[i] = r.MaxEvals
			}
		}
	}
	return b, all
}

// contextStoreBounded reports, for the shape on the backend, whether B is evaluated at its busiest
// position as often for the larger input as for the smaller, and the parse evaluates at most
// gentest.ContextStoreGrowth times the calls.
func contextStoreBounded(t *testing.T, prog *Program, s gentest.DependencyMemoShape, o ParseOptions) (string, bool) {
	b, all := contextStoreGrowth(t, prog, s, o)
	msg := fmt.Sprintf("%s %s: B evaluated at most %v times at a position, %v evaluations in all, at m=%v",
		s.Name, o.Backend, b, all, gentest.ContextStoreSizes)
	return msg, b[0] > 0 && b[0] == b[1] && all[1] <= all[0]*gentest.ContextStoreGrowth
}

// TestContextStoreBounds checks on every backend that a context with more dependencies or more
// leaders than the fixed-size keys of the context store hold still has its results reused: the
// evaluations of a rule at one position do not grow with the input.
func TestContextStoreBounds(t *testing.T) {
	t.Parallel()
	for _, s := range gentest.ContextStoreShapes {
		prog := mustCompile(t, "@start\nrule main = "+s.Rules, compile.Options{SourceInfo: true})
		for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
			if msg, ok := contextStoreBounded(t, prog, s, ParseOptions{Backend: b}); !ok {
				t.Error(msg)
			}
		}
	}
}

// TestContextStoreBoundsControls checks that TestContextStoreBounds detects a context store that
// drops what its fixed-size keys cannot hold.
func TestContextStoreBoundsControls(t *testing.T) {
	for _, c := range []struct {
		mutation int
		shape    string
	}{
		{core.CtxCapDeps, "seven dependencies"},
	} {
		var s gentest.DependencyMemoShape
		for _, x := range gentest.ContextStoreShapes {
			if x.Name == c.shape {
				s = x
			}
		}
		prog := mustCompile(t, "@start\nrule main = "+s.Rules, compile.Options{SourceInfo: true})
		func() {
			core.ContextMemoMutation = c.mutation
			defer func() { core.ContextMemoMutation = 0 }()
			for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
				if msg, ok := contextStoreBounded(t, prog, s, ParseOptions{Backend: b}); ok {
					t.Errorf("mutation %d not detected: %s", c.mutation, msg)
				}
			}
		}()
	}
}
