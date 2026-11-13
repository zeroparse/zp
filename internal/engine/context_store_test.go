package engine

import (
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/codegentest"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/engine/state"
)

// contextStoreGrowth parses the shape at the two codegentest.ContextStoreSizes on the backend and
// returns the evaluations of B at its busiest position and of all rules, at each size. The parses
// have no compute limit: a context that the store cannot index exceeds the default one.
func contextStoreGrowth(t *testing.T, prog *Program, s codegentest.DependencyMemoShape, o ParseOptions) (b, all [2]int) {
	t.Helper()
	for i, m := range codegentest.ContextStoreSizes {
		var pr Profile
		o.Trace = pr.Trace
		o.Limits = core.Limits{Compute: core.Unlimited}
		// With missing-token insertion, the parse reports the insertion as an error.
		if r, err := prog.ParseWith("", s.Input(m), o); err != nil && (r == nil || o.MissingTokens == 0) {
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
// codegentest.ContextStoreGrowth times the calls.
func contextStoreBounded(t *testing.T, prog *Program, s codegentest.DependencyMemoShape, o ParseOptions) (string, bool) {
	b, all := contextStoreGrowth(t, prog, s, o)
	msg := fmt.Sprintf("%s %s: B evaluated at most %v times at a position, %v evaluations in all, at m=%v",
		s.Name, o.Backend, b, all, codegentest.ContextStoreSizes)
	return msg, b[0] > 0 && b[0] == b[1] && all[1] <= all[0]*codegentest.ContextStoreGrowth
}

// TestContextStoreBounds checks on every backend that a context with more dependencies or more
// leaders than the fixed-size keys of the context store hold still has its results reused: the
// evaluations of a rule at one position do not grow with the input.
func TestContextStoreBounds(t *testing.T) {
	t.Parallel()
	for _, s := range codegentest.ContextStoreShapes {
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
		{state.CtxCapDeps, "seven dependencies"},
		{state.CtxNarrowPresent, "70 leaders"},
	} {
		var s codegentest.DependencyMemoShape
		for _, x := range codegentest.ContextStoreShapes {
			if x.Name == c.shape {
				s = x
			}
		}
		prog := mustCompile(t, "@start\nrule main = "+s.Rules, compile.Options{SourceInfo: true})
		func() {
			state.ContextMemoMutation = c.mutation
			defer func() { state.ContextMemoMutation = 0 }()
			for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
				if msg, ok := contextStoreBounded(t, prog, s, ParseOptions{Backend: b}); ok {
					t.Errorf("mutation %d not detected: %s", c.mutation, msg)
				}
			}
		}()
	}
}

// missingChain is the shape of TestContextStoreMissingInsertion: a chain whose rules insert nothing,
// in a grammar where main inserts a missing ";" after it.
var missingChain = codegentest.DependencyMemoShape{Name: "missing-token insertion", Rules: codegentest.ContextChain(2, 1, true), Input: codegentest.ContextChainInput}

// TestContextStoreMissingInsertion checks that a parse with missing-token insertion keeps the
// results of rules that insert nothing in contexts: their evaluations at one position do not grow
// with the input, as without insertion.
func TestContextStoreMissingInsertion(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, "@start\nrule main = "+missingChain.Rules, compile.Options{SourceInfo: true})
	if msg, ok := contextStoreBounded(t, prog, missingChain, ParseOptions{Backend: Closure, MissingTokens: 1}); !ok {
		t.Error(msg)
	}
}

// TestContextStoreMissingInsertionControl checks that TestContextStoreMissingInsertion detects a
// parse with insertion that keeps no result in a context.
func TestContextStoreMissingInsertionControl(t *testing.T) {
	prog := mustCompile(t, "@start\nrule main = "+missingChain.Rules, compile.Options{SourceInfo: true})
	state.ContextMemoMutation = state.CtxStrictMissing
	defer func() { state.ContextMemoMutation = 0 }()
	if msg, ok := contextStoreBounded(t, prog, missingChain, ParseOptions{Backend: Closure, MissingTokens: 1}); ok {
		t.Errorf("CtxStrictMissing not detected: %s", msg)
	}
}

// TestContextStoreMissingRandom checks that keeping results in contexts in parses with
// missing-token insertion changes only the cost: random cycle grammars, some of whose literals may
// be inserted, give the same results, errors and expectations as without memos, where some
// rules of the cycles insert tokens and others read their seeds.
func TestContextStoreMissingRandom(t *testing.T) {
	t.Parallel()
	seeds := 40
	if codegentest.Long() {
		seeds = 400
	}
	for _, msg := range missingRandomMismatches(t, seeds) {
		t.Error(msg)
	}
}

// TestContextStoreMissingRandomControl checks that TestContextStoreMissingRandom detects rules
// that insert tokens keeping their results in contexts.
func TestContextStoreMissingRandomControl(t *testing.T) {
	state.ContextMemoMutation = state.CtxStoreInserts
	defer func() { state.ContextMemoMutation = 0 }()
	if len(missingRandomMismatches(t, 40)) == 0 {
		t.Error("CtxStoreInserts not detected")
	}
}

// missingRandomMismatches parses the inputs of seeds random cycle grammars with insertion, with and
// without memos, and returns the differences.
func missingRandomMismatches(t *testing.T, seeds int) []string {
	t.Helper()
	var out []string
	lit := regexp.MustCompile(`"[a-z]"`)
	checked := 0
	for s := int64(0); s < int64(seeds); s++ {
		c := codegentest.RandomCycleGrammar(s, false)
		rng := rand.New(rand.NewSource(s))
		src := lit.ReplaceAllStringFunc(c.Src, func(l string) string {
			if rng.Intn(3) == 0 {
				return l + " #missing"
			}
			return l
		})
		if !strings.Contains(src, "#missing") {
			continue
		}
		g, err := parseTest(src)
		if err != nil {
			continue
		}
		if _, err := Compile(g, compile.Options{}); err != nil {
			continue
		}
		checked++
		memo, ref := mustCompile(t, src), mustCompile(t, src, compile.Options{DisableMemo: true})
		for _, in := range c.Inputs {
			for _, recognize := range []bool{false, true} {
				o := ParseOptions{Backend: Closure, Recognize: recognize, MissingTokens: 3}
				got, err := memo.ParseWith("", in, o)
				o.ContextMemoOff = true
				want, wantErr := ref.ParseWith("", in, o)
				a, b := codegentest.ResultJSON(got, err), codegentest.ResultJSON(want, wantErr)
				// Only the compute and memory limits may end the two parses at different points: rules
				// that insert tokens keep no result, so both make the same insertion attempts.
				if a != b && !strings.Contains(a+b, "compute limit") && !strings.Contains(a+b, "memory limit") {
					out = append(out, fmt.Sprintf("seed %d %q recognize=%v:\n%s\n got %s\nwant %s", s, in, recognize, src, a, b))
				}
			}
		}
	}
	if checked < seeds/4 {
		t.Fatalf("only %d of %d grammars compiled with insertion", checked, seeds)
	}
	return out
}
