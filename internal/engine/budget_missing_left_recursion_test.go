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
		{Closure, false, 10, false}:         {248, 81016},
		{Closure, true, 10, false}:          {214, 46160},
		{Closure, false, 0, true}:           {245, 81820},
		{Bytecode, false, 0, true}:          {295, 90316},
		{BytecodeIterative, false, 0, true}: {294, 91612},
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

// TestBudgetCallEndChargesErrorsAfterExpectations pins where a limit interrupts the end of a
// memoized call: the call keeps its expectations (Keep) before it copies its recovered errors into
// its entry, so a limit that the copy reaches leaves the expectations kept and no entry stored.
// The total does not depend on the order; the point of the interruption does.
func TestBudgetCallEndChargesErrorsAfterExpectations(t *testing.T) {
	t.Parallel()
	end := func(limit uint64) (err error, kept, stored bool) {
		p := &core.Parser{Memo: core.NewMemoTable(), MemoAll: true, Expectations: true}
		r := &core.Rule{}
		r.ID, r.Memo = 1, true
		st := &core.CallState{}
		p.CallBegin(r, 0, st)
		p.Expect(0, 7)
		p.Recovered = append(p.Recovered, core.ErrEntry(&core.SyntaxError{}), core.ErrEntry(&core.SyntaxError{}), core.ErrEntry(&core.SyntaxError{}))
		p.Control = core.NewBudgetControl(nil, limits.Limits{Compute: limit})
		err = catchFatal(t, func() { p.CallEnd(r, st, nil, true) })
		_, stored = p.Memo.Get(core.MemoKey{Rule: r.ID})
		return err, len(p.LastKept) == 1, stored
	}
	keptOnly := 0
	for limit := uint64(1); ; limit++ {
		err, kept, stored := end(limit)
		if err == nil {
			if !kept || !stored {
				t.Fatalf("the call ended under a limit of %d without keeping or storing its result", limit)
			}
			break
		}
		if !errors.Is(err, limits.ErrLimit) {
			t.Fatalf("limit %d: %v", limit, err)
		}
		if kept && !stored {
			keptOnly++
		}
		if stored && !kept {
			t.Fatalf("limit %d: the entry was stored before the expectations were kept", limit)
		}
	}
	// Each of the three errors that the call copies is charged after Keep and before the store, and
	// so is a limit that interrupts the call between them.
	if keptOnly < 3 {
		t.Fatalf("%d limits interrupt the call after it keeps its expectations and before it stores its entry, want at least 3", keptOnly)
	}
}
