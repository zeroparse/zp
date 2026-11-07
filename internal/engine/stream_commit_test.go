//go:build !zp_budget_controls_reference

package engine

import (
	"context"
	"errors"
	"fmt"
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
		p := &core.Parser{Memo: core.NewMemoTable(), Recovered: make([]core.RecEntry, 1, capacity)}
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
	// Without frame scratch the release charges nothing, so the same commit runs to its end.
	p := &core.Parser{Input: core.Input{Unit: core.Bytes, Eof: true, Bs: []byte(strings.Repeat("a", 256)), BaseLine: 1, BaseCol: 1, Lines: []int{0},
		Control: core.NewBudgetControl(nil, limits.Limits{Compute: 128})}, Memo: core.NewMemoTable(), FrameSlab: make([]core.Frame, 2)}
	p.Frame = &p.FrameSlab[1]
	got := catchStreamPanic(func() { p.Commit(130) })
	if !core.StreamFrameScratch {
		if got != nil || p.Base != 129 || p.Control.Used != 127 {
			t.Fatalf("interruption=%v base=%d used=%d", got, p.Base, p.Control.Used)
		}
		return
	}
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
	p := &core.Parser{Input: core.Input{Base: 1023, Control: core.NewBudgetControl(nil, limits.Limits{Compute: 1})}, Memo: memo, Recovered: []core.RecEntry{core.ErrEntry(first)}}
	p.DiagnosticTail.Limit = 1
	p.Diagnosed = 1
	got := catchStreamPanic(func() { p.Commit(1024) })
	if _, ok := got.(core.Fatal); !ok || len(p.Recovered) != 1 || p.Recovered[0] != core.ErrEntry(first) || p.Diagnosed != 1 || memo.Base != 0 {
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

// TestStreamCommitChargesItsDeclaredWork measures the operations whose work depends on more than the
// size of their argument, at the bounds that they declare in internal/ir/exec/ops.
func TestStreamCommitChargesItsDeclaredWork(t *testing.T) {
	t.Parallel()
	budget := func() *core.BudgetControl { return core.NewBudgetControl(nil, limits.Limits{Compute: 1 << 20}) }
	t.Run("the frames are scanned and cleared", func(t *testing.T) {
		// The frame in use lies outside the scratch, so the scan finds nothing and the clear visits
		// every frame again: two units for each of the four.
		p := &core.Parser{Input: core.Input{Control: budget()}, Memo: core.NewMemoTable(), FrameSlab: make([]core.Frame, 4), Frame: &core.Frame{}}
		p.Commit(1)
		want := uint64(8)
		if !core.StreamFrameScratch {
			want = 0
		}
		if p.Control.Used != want {
			t.Fatalf("used=%d, want %d", p.Control.Used, want)
		}
	})
	t.Run("the whole input is scanned for lines", func(t *testing.T) {
		// The input is complete from its first byte, so the line table covers all 3000 bytes, past the
		// 1501 that go; the compaction copies the 1499 that stay.
		p := &core.Parser{Input: core.Input{Unit: core.Bytes, Eof: true, Bs: []byte(strings.Repeat("a", 3000)), BaseLine: 1, BaseCol: 1, Control: budget()}, Memo: core.NewMemoTable()}
		p.Commit(1502)
		if p.Base != 1501 || p.Control.Used != 3000+1499 {
			t.Fatalf("base=%d used=%d", p.Base, p.Control.Used)
		}
	})
	t.Run("the scratch of the errors is cleared", func(t *testing.T) {
		p := &core.Parser{Input: core.Input{Control: budget()}, Memo: core.NewMemoTable(), Recovered: make([]core.RecEntry, 1, 16)}
		p.DiagnosticTail.Limit = 1
		p.Commit(1)
		if p.Control.Used != 16 {
			t.Fatalf("used=%d", p.Control.Used)
		}
	})
}

// TestStreamCancelledContextDeliversNoDiagnostic holds the emission to its first checkpoint: a
// context that is already cancelled stops it before the diagnostic callback runs, as well as before
// the element.
func TestStreamCancelledContextDeliversNoDiagnostic(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	diagnostics, elements := 0, 0
	p := &core.Parser{Input: core.Input{Control: core.NewBudgetControl(ctx, limits.Limits{})}, Memo: core.NewMemoTable(), Recovered: []core.RecEntry{core.ErrEntry(&core.SyntaxError{Pos: 1})},
		Diagnostic: func(*core.SyntaxError) error { diagnostics++; return nil },
		Emit:       func(*value.Node) error { elements++; return nil }}
	got := catchStreamPanic(func() { p.EmitStream(nil, 0) })
	if f, ok := got.(core.Fatal); !ok || f.Err != context.Canceled || diagnostics != 0 || elements != 0 {
		t.Fatalf("panic=%v diagnostics=%d elements=%d", got, diagnostics, elements)
	}
}

// TestStreamDiagnosticCallbackPanicIsTheCallers keeps a panic of the diagnostic callback the caller's,
// as for the callback of the elements: the parse does not report it as a fault of its own.
func TestStreamDiagnosticCallbackPanicIsTheCallers(t *testing.T) {
	t.Parallel()
	p := mustCompile(t, `rule main = item+ #stream $$
rule item = "ab" #recover(skip=[^a]+)`)
	for _, backend := range []Backend{Closure, Bytecode, BytecodeIterative} {
		for _, runtimeValue := range []bool{false, true} {
			t.Run(fmt.Sprintf("%v/runtime=%t", backend, runtimeValue), func(t *testing.T) {
				var token any = &struct{ phase string }{"diagnostic"}
				if runtimeValue {
					token = &streamRuntimePanicToken{"diagnostic"}
				}
				var err error
				got := catchStreamPanic(func() {
					err = p.ParseStreamWith("main", strings.NewReader("abX!"), func(*value.Node) error { return nil },
						ParseOptions{Backend: backend, DiagnosticCallback: func(*SyntaxError) error { panic(token) }})
				})
				if got != token || err != nil {
					t.Fatalf("panic=%#v err=%v", got, err)
				}
			})
		}
	}
}

// TestStreamDiagnosticCallbackEndsTheEmission holds what follows the diagnostic callback: a
// cancellation in its last call or an error it returns stops the emission before the element, and
// the parse returns it as an error, since the callback is over and the end of the parse is not the
// caller's panic.
func TestStreamDiagnosticCallbackEndsTheEmission(t *testing.T) {
	t.Parallel()
	p := mustCompile(t, `rule main = item+ #stream $$
rule item = ("a" #recover(skip="x")) ("b" #recover(skip="y"))`)
	stop := errors.New("stop")
	for _, backend := range []Backend{Closure, Bytecode, BytecodeIterative} {
		for _, c := range []struct {
			name  string
			calls int
			want  error
		}{{"cancel", 2, context.Canceled}, {"error", 1, stop}} {
			t.Run(backend.String()+"/"+c.name, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				diagnostics, elements := 0, 0
				var err error
				got := catchStreamPanic(func() {
					err = p.ParseStreamWith("main", strings.NewReader("xyab"), func(*value.Node) error { elements++; return nil },
						ParseOptions{Backend: backend, Context: ctx, DiagnosticCallback: func(*SyntaxError) error {
							diagnostics++
							if c.want == stop {
								return stop
							}
							if diagnostics == c.calls {
								cancel()
							}
							return nil
						}})
				})
				if got != nil || !errors.Is(err, c.want) || diagnostics != c.calls || elements != 0 {
					t.Fatalf("panic=%v err=%v diagnostics=%d elements=%d", got, err, diagnostics, elements)
				}
			})
		}
	}
}

// TestStreamDiagnosticCallbackMarkEndsWithIt keeps the mark of a caller's callback to the callback:
// the interruptions that follow it reach the caller the same with the mark left on, so only a fault
// of the parser after the callback would show it, as the caller's panic.
func TestStreamDiagnosticCallbackMarkEndsWithIt(t *testing.T) {
	t.Parallel()
	for _, want := range []error{nil, errors.New("stop")} {
		p := &core.Parser{Recovered: []core.RecEntry{core.ErrEntry(&core.SyntaxError{Pos: 1})}}
		p.Diagnostic = func(*core.SyntaxError) error {
			if !p.EmitCall {
				t.Error("the callback runs without the mark")
			}
			return want
		}
		if err := p.ReportRecovered(); err != want || p.EmitCall {
			t.Fatalf("err=%v mark=%t, want err %v without the mark", err, p.EmitCall, want)
		}
	}
}
