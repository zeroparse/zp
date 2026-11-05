//go:build !zp_budget_controls_reference

package engine

import (
	"errors"
	"testing"

	"zeroparse.dev/zp/internal/engine/core"
	"zeroparse.dev/zp/internal/limits"
)

// leastLimit returns the least limit, as set by with, under which parse ends without reaching a
// limit: the units that the parse uses, since a parse stops at the first charge beyond its limit.
// Every parse that ends must end with the same error as the parse under the largest limit.
func leastLimit(t *testing.T, with func(n uint64) limits.Limits, parse func(limits.Limits) error) uint64 {
	t.Helper()
	lo, hi := uint64(1), uint64(1)<<24
	end := parse(with(hi))
	var syntax core.SyntaxErrors
	if end != nil && !errors.As(end, &syntax) {
		t.Fatalf("the parse under a limit of %d fails with %v, not a syntax error", hi, end)
	}
	for lo < hi {
		mid := lo + (hi-lo)/2
		err := parse(with(mid))
		if errors.Is(err, limits.ErrLimit) {
			lo = mid + 1
			continue
		}
		if (err == nil) != (end == nil) || err != nil && err.Error() != end.Error() {
			t.Fatalf("the parse ends with %v under a limit of %d and with %v under %d", err, mid, end, uint64(1)<<24)
		}
		hi = mid
	}
	return hi
}

// TestBudgetUnrecordedContextCalls pins the compute and memory of a parse whose left-recursive
// calls run in a context that records no dependencies, as with missing tokens or without the
// context memo: their results go to no table, so such a call allocates no entry and copies no
// recovered errors. It covers missing tokens in the Closure parse and recognition, and the parse
// without the context memo on every backend.
func TestBudgetUnrecordedContextCalls(t *testing.T) {
	t.Parallel()
	// The bytecode backends do not run grammars with missing tokens: only the Closure runs have h.
	const grammar = `rule e = f "+" "a" / f "-" "a" / "a" g
rule f = e ("x" / "y" / "z")?
rule g = ";" / "x" #recover(skip=[b]+)`
	missingProg, prog := mustCompile(t, grammar+"\nrule h = \"q\" #missing"), mustCompile(t, grammar)
	type run struct {
		backend   Backend
		recognize bool
		missing   int
		ctxOff    bool
	}
	type usage struct{ compute, memory uint64 }
	want := map[run]usage{
		{Closure, false, 10, false}:         {255, 83088},
		{Closure, true, 10, false}:          {221, 48232},
		{Closure, false, 0, true}:           {252, 83924},
		{Bytecode, false, 0, true}:          {302, 92420},
		{BytecodeIterative, false, 0, true}: {301, 93716},
	}
	for r, w := range want {
		p := prog
		if r.missing != 0 {
			p = missingProg
		}
		parse := func(l limits.Limits) error {
			_, err := p.ParseWith("e", "abb+a-a+a", ParseOptions{Backend: r.backend, Recognize: r.recognize, MissingTokens: r.missing, ContextMemoOff: r.ctxOff, Limits: l})
			return err
		}
		got := usage{
			compute: leastLimit(t, func(n uint64) limits.Limits { return limits.Limits{Compute: n} }, parse),
			memory:  leastLimit(t, func(n uint64) limits.Limits { return limits.Limits{Memory: n} }, parse),
		}
		if got != w {
			t.Errorf("%+v: used %+v, want %+v", r, got, w)
		}
	}
}
