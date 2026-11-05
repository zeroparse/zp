//go:build !zp_budget_controls_reference

package engine

import (
	"errors"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/engine/core"
	"zeroparse.dev/zp/internal/limits"
	"zeroparse.dev/zp/internal/value"
)

// TestStreamCommitFreezesDeliveredElements holds the delivery to its contract: an element is final
// once the callback sees it, so no later step of the parse may extend it in place.
func TestStreamCommitFreezesDeliveredElements(t *testing.T) {
	t.Parallel()
	p := mustCompile(t, `rule main = ("a" "b")* #stream $$`)
	for _, backend := range []Backend{Closure, Bytecode, BytecodeIterative} {
		for _, unit := range []PositionUnit{CodePoints, Bytes} {
			t.Run(backend.String()+"/"+unit.String(), func(t *testing.T) {
				elements := 0
				err := p.ParseStreamWith("main", strings.NewReader("ababab"), func(n *value.Node) error {
					elements++
					if n == nil || value.D(n).Kind != value.SeqKind || value.D(n).Fresh {
						t.Errorf("element %d is %+v", elements, n)
					}
					return nil
				}, ParseOptions{Backend: backend, Unit: unit})
				if err != nil || elements != 3 {
					t.Fatalf("elements=%d err=%v", elements, err)
				}
			})
		}
	}
}

// TestStreamCommitForgetsTheDiagnosedCount keeps every recovered error of an element that recovers
// more often than the one before it: a commit that settles the errors must also forget how many of
// them were reported, or the next report starts inside the new element's errors.
func TestStreamCommitForgetsTheDiagnosedCount(t *testing.T) {
	t.Parallel()
	p := mustCompile(t, `rule main = item+ #stream $$
rule item = ("a" #recover(skip="x")) ("b" #recover(skip="y"))`)
	for _, backend := range []Backend{Closure, Bytecode, BytecodeIterative} {
		for _, unit := range []PositionUnit{CodePoints, Bytes} {
			t.Run(backend.String()+"/"+unit.String(), func(t *testing.T) {
				var events []string
				err := p.ParseStreamWith("main", strings.NewReader("ayxy"), func(*value.Node) error {
					events = append(events, "element")
					return nil
				}, ParseOptions{Backend: backend, Unit: unit, DiagnosticLimit: 8, DiagnosticLimitSet: true, DiagnosticCallback: func(*SyntaxError) error {
					events = append(events, "diagnostic")
					return nil
				}})
				want := "diagnostic element diagnostic diagnostic element"
				if got := strings.Join(events, " "); got != want {
					t.Fatalf("events %q, want %q", got, want)
				}
				var errs SyntaxErrors
				if !errors.As(err, &errs) || len(errs) != 3 {
					t.Fatalf("err=%v", err)
				}
			})
		}
	}
}

// TestStreamCommitBoundsTheErrorScratch pins the size at which a commit stops keeping the scratch of
// the recovered errors: the largest one it reuses, and the next.
func TestStreamCommitBoundsTheErrorScratch(t *testing.T) {
	t.Parallel()
	for capacity, reused := range map[int]bool{16: true, 17: false} {
		p := &core.Parser{Memo: core.NewMemoTable(), Recovered: make([]*core.SyntaxError, 1, capacity)}
		p.DiagnosticTail.Limit = 1
		p.Commit(1)
		if (p.Recovered != nil) != reused || reused && cap(p.Recovered) != capacity {
			t.Errorf("capacity %d: recovered=%v cap=%d", capacity, p.Recovered, cap(p.Recovered))
		}
	}
}

// TestStreamCommitDiscardsBeforeItReleases pins the first two steps of a commit: when the budget
// runs out in the release, the input is already gone and the frames are not.
func TestStreamCommitDiscardsBeforeItReleases(t *testing.T) {
	t.Parallel()
	// The discard before position 129 copies 127 units, and the scan of the frames takes two more.
	p := &core.Parser{Input: core.Input{Unit: core.Bytes, Eof: true, Bs: []byte(strings.Repeat("a", 256)), BaseLine: 1, BaseCol: 1, Lines: []int{0},
		Control: core.NewBudgetControl(nil, limits.Limits{Compute: 128})}, Memo: core.NewMemoTable(), FrameSlab: make([]core.Frame, 2)}
	p.Frame = &p.FrameSlab[1]
	got := catchStreamPanic(func() { p.Commit(130) })
	if _, ok := got.(core.Fatal); !ok || p.Base != 129 || len(p.FrameSlab) != 2 || p.Control.Used != 128 {
		t.Fatalf("interruption=%v base=%d frames=%d used=%d", got, p.Base, len(p.FrameSlab), p.Control.Used)
	}
}

// TestStreamCommitPrunesBeforeItSettles pins the last two steps of a commit: when the budget runs
// out in the pruning, the recovered errors are not settled yet.
func TestStreamCommitPrunesBeforeItSettles(t *testing.T) {
	t.Parallel()
	first := &core.SyntaxError{Pos: 1}
	memo := core.NewMemoTable()
	memo.Slots = make([]*core.MemoEntry, 2)
	p := &core.Parser{Input: core.Input{Base: 1023, Control: core.NewBudgetControl(nil, limits.Limits{Compute: 1})}, Memo: memo, Recovered: []*core.SyntaxError{first}}
	p.DiagnosticTail.Limit = 1
	p.Diagnosed = 1
	got := catchStreamPanic(func() { p.Commit(1024) })
	if _, ok := got.(core.Fatal); !ok || len(p.Recovered) != 1 || p.Recovered[0] != first || p.Diagnosed != 1 || memo.Base != 0 {
		t.Fatalf("interruption=%v recovered=%v diagnosed=%d floor=%d", got, p.Recovered, p.Diagnosed, memo.Base)
	}
}

// A diagnostic limit needs no callback: each commit still moves the committed errors into the tail
// before it settles them, so that the terminal result counts them all.
func TestStreamCommitFeedsATailWithoutCallback(t *testing.T) {
	t.Parallel()
	p := mustCompile(t, `rule main = item+ #stream $$
rule item = "ab" #recover(skip=[^a]+)`)
	const count = 100
	input := strings.Repeat("abX!", count)
	for _, backend := range []Backend{Closure, Bytecode, BytecodeIterative} {
		err := p.ParseStreamWith("main", strings.NewReader(input), func(*value.Node) error { return nil },
			ParseOptions{Backend: backend, DiagnosticLimit: 2})
		var tail *TruncatedSyntaxErrors
		if !errors.As(err, &tail) || tail.Total != count || tail.Dropped != count-2 || len(tail.Errors) != 2 {
			t.Fatalf("%v: tail=%+v err=%v", backend, tail, err)
		}
		if got, want := tail.Errors[1].Pos, (count-1)*4+2; got != want {
			t.Fatalf("%v: last error at %d, want %d", backend, got, want)
		}
	}
}
