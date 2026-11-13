//go:build !zp_budget_controls_reference

package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/codegentest"
	"zeroparse.dev/zp/internal/engine/state"
	"zeroparse.dev/zp/internal/value"
)

func budgetStreamVariants(t *testing.T, run func(*testing.T, ParseOptions)) {
	t.Helper()
	for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
		for _, u := range []core.PositionUnit{core.Bytes, core.CodePoints} {
			t.Run(fmt.Sprintf("%s/%s", b, u), func(t *testing.T) { run(t, ParseOptions{Backend: b, Unit: u}) })
		}
	}
}

type budgetStreamReader struct {
	chunks    []string
	err       error
	cancel    func()
	cancelAt  int
	dataError bool
	reads     int
	closed    bool
}

func (r *budgetStreamReader) Read(dst []byte) (int, error) {
	r.reads++
	if r.cancel != nil && r.reads == r.cancelAt {
		r.cancel()
	}
	if len(r.chunks) > 0 {
		chunk := r.chunks[0]
		n := copy(dst, chunk)
		if n < len(chunk) {
			r.chunks[0] = chunk[n:]
		} else {
			r.chunks = r.chunks[1:]
		}
		if r.dataError && len(r.chunks) == 0 {
			return n, r.err
		}
		return n, nil
	}
	if r.err != nil {
		return 0, r.err
	}
	return 0, io.EOF
}
func (r *budgetStreamReader) Close() error { r.closed = true; return nil }

func TestBudgetStreamAdmission(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, codegentest.Records)
	budgetStreamVariants(t, func(t *testing.T, o ParseOptions) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		o.Context = ctx
		r := &budgetStreamReader{chunks: []string{"#records\na=1\n"}}
		callbacks := 0
		o.Trace = func(TraceEvent) { callbacks++ }
		err := prog.ParseStreamWith("main", r, func(*core.Node) error { callbacks++; return nil }, o)
		if err != context.Canceled || r.reads != 0 || callbacks != 0 || r.closed {
			t.Fatalf("error=%v reads=%d callbacks=%d closed=%t", err, r.reads, callbacks, r.closed)
		}
		for _, phase := range []string{"Done", "Err"} {
			t.Run(phase, func(t *testing.T) {
				token := &budgetContextPanicToken{phase: phase}
				if phase == "Done" {
					o.Context = &budgetContextDonePanic{Context: context.Background(), token: token}
				} else {
					c := newBudgetContextErrPanic(token, 1)
					c.armed = true
					o.Context = c
				}
				got := budgetContextRecover(func() { _ = prog.ParseStreamWith("main", r, nil, o) })
				if got != token || r.reads != 0 {
					t.Fatalf("panic=%v reads=%d", got, r.reads)
				}
				// Fixed errors precede caller Context methods and reader wrapping.
				if got := budgetContextRecover(func() { err = prog.ParseStreamWith("missing", r, nil, o) }); got != nil || err == nil {
					t.Fatalf("invalid start panic=%v error=%v", got, err)
				}
			})
		}
	})
}

func TestBudgetStreamUnlimitedParity(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, codegentest.Records)
	budgetStreamVariants(t, func(t *testing.T, base ParseOptions) {
		for _, text := range []string{"#records\na=1\nbc=22\n", "#records\na=1\nbad=x\n"} {
			for _, withError := range []bool{false, true} {
				stop := errors.New("data and error")
				var results [2][]string
				var errs [2]error
				for i := range 2 {
					o := base
					if i == 1 {
						ctx, cancel := context.WithCancel(context.Background())
						defer cancel()
						o.Context = ctx
					}
					r := &budgetStreamReader{chunks: []string{text}, dataError: withError}
					if withError {
						r.err = stop
					}
					errs[i] = prog.ParseStreamWith("main", r, func(n *core.Node) error { results[i] = append(results[i], codegentest.ResultJSON(n, nil)); return nil }, o)
				}
				if !reflect.DeepEqual(results[0], results[1]) || fmt.Sprint(errs[0]) != fmt.Sprint(errs[1]) {
					t.Fatalf("controlled results/errors differ: %v %v; %v %v", results[0], errs[0], results[1], errs[1])
				}
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		base.Context = ctx
		if err := prog.ParseStreamWith("main", strings.NewReader("#records\na=1\n"), nil, base); err != nil {
			t.Fatal(err)
		}
	})
}

func TestBudgetStreamReaderErrorPrecedence(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, codegentest.Records)
	budgetStreamVariants(t, func(t *testing.T, base ParseOptions) {
		for _, dataError := range []bool{false, true} {
			t.Run(fmt.Sprintf("data=%t", dataError), func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				o := base
				o.Context = ctx
				stop := errors.New("reader error wins")
				r := &budgetStreamReader{chunks: []string{"#records\na=1\n"}, err: stop, cancel: cancel, cancelAt: 2}
				if dataError {
					r.chunks = []string{"#records\na=1\n", "bc=22\n"}
					r.dataError = true
				}
				count := 0
				err := prog.ParseStreamWith("main", r, func(*core.Node) error { count++; return nil }, o)
				if err != stop || count != 1 || r.reads != 2 || r.closed {
					t.Fatalf("error=%v emits=%d reads=%d closed=%t", err, count, r.reads, r.closed)
				}
			})
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		base.Context = ctx
		r := &budgetStreamReader{chunks: []string{"#records\na=1\n"}, cancel: cancel, cancelAt: 2}
		if err := prog.ParseStreamWith("main", r, func(*core.Node) error { return nil }, base); err != context.Canceled {
			t.Fatalf("EOF error=%v", err)
		}
	})
}

func TestBudgetStreamEmitPrecedenceAndOwnership(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, codegentest.Records)
	budgetStreamVariants(t, func(t *testing.T, base ParseOptions) {
		for _, fail := range []bool{false, true} {
			ctx, cancel := context.WithCancel(context.Background())
			o := base
			o.Context = ctx
			stop := errors.New("emit error wins")
			var node *core.Node
			var snapshot string
			count := 0
			err := prog.ParseStreamWith("main", strings.NewReader("#records\na=1\nbc=22\n"), func(n *core.Node) error {
				count++
				node = n
				snapshot = codegentest.ResultJSON(n, nil)
				cancel()
				if fail {
					return stop
				}
				return nil
			}, o)
			cancel()
			want := error(context.Canceled)
			if fail {
				want = stop
			}
			if err != want || count != 1 || node == nil || codegentest.ResultJSON(node, nil) != snapshot {
				t.Fatalf("error=%v emits=%d snapshot changed=%t", err, count, codegentest.ResultJSON(node, nil) != snapshot)
			}
			if err := prog.ParseStreamWith("main", strings.NewReader("#records\nxy=3\n"), func(*core.Node) error { return nil }, base); err != nil || codegentest.ResultJSON(node, nil) != snapshot {
				t.Fatalf("fresh replay error=%v", err)
			}
		}
	})
}

type budgetStreamGatedReader struct {
	entered chan struct{}
	release chan struct{}
	reads   int
	closed  bool
}

func (r *budgetStreamGatedReader) Read(dst []byte) (int, error) {
	r.reads++
	if r.reads == 1 {
		close(r.entered)
		<-r.release
		return copy(dst, "#records\na=1\n"), nil
	}
	return 0, io.EOF
}
func (r *budgetStreamGatedReader) Close() error { r.closed = true; return nil }

func TestBudgetStreamBlockedReaderCooperation(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, codegentest.Records)
	budgetStreamVariants(t, func(t *testing.T, o ParseOptions) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		o.Context = ctx
		r := &budgetStreamGatedReader{entered: make(chan struct{}), release: make(chan struct{})}
		var release sync.Once
		defer release.Do(func() { close(r.release) })
		result := make(chan error, 1)
		emits := 0
		go func() { result <- prog.ParseStreamWith("main", r, func(*core.Node) error { emits++; return nil }, o) }()
		select {
		case <-r.entered:
		case err := <-result:
			t.Fatalf("returned before Read: %v", err)
		}
		cancel()
		release.Do(func() { close(r.release) })
		if err := <-result; err != context.Canceled || r.reads != 1 || emits != 0 || r.closed {
			t.Fatalf("error=%v reads=%d emits=%d closed=%t", err, r.reads, emits, r.closed)
		}
	})
}

func TestBudgetStreamLimitsAndCallerPanics(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, codegentest.Records)
	budgetStreamVariants(t, func(t *testing.T, base ParseOptions) {
		for _, kind := range []core.LimitKind{core.LimitCompute, core.LimitCompute} {
			o := base
			if kind == core.LimitCompute {
				o.Limits.Compute = 16
			} else {
				o.Limits.Compute = 1
			}
			count := 0
			var saved TraceEvent
			o.Trace = func(e TraceEvent) {
				if state.TraceParser(saved) == nil {
					saved = e
				}
			}
			err := prog.ParseStreamWith("main", strings.NewReader("#records\n"+strings.Repeat("a", 20000)+"=1\n"), func(*core.Node) error { count++; return nil }, o)
			var we *core.LimitError
			if !errors.As(err, &we) || we.Kind != kind || count != 0 {
				t.Fatalf("error=%v emits=%d", err, count)
			}
			assertTraceParserDetached(t, state.TraceParser(saved), false)
		}
		for _, phase := range []string{"reader", "emit", "trace", "context"} {
			t.Run(phase, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				o := base
				o.Context = ctx
				token := &streamRuntimePanicToken{phase: phase}
				r := &streamCallbackReader{text: "#records\na=1\n", panicValue: token}
				if phase != "reader" {
					r.panicValue = nil
				}
				var saved TraceEvent
				o.Trace = func(e TraceEvent) {
					if state.TraceParser(saved) == nil {
						saved = e
					}
					if e.Kind == TraceEnter && e.Rule == "record" {
						if phase == "trace" {
							panic(token)
						}
						if phase == "context" {
							c := newBudgetContextErrPanic(&budgetContextPanicToken{phase: phase}, 1)
							c.armed = true
							state.TraceParser(e).Control.Ctx = c
							token = nil
						}
					}
				}
				got := catchStreamPanic(func() {
					_ = prog.ParseStreamWith("main", r, func(*core.Node) error {
						if phase == "emit" {
							panic(token)
						}
						return nil
					}, o)
				})
				if phase == "context" {
					if _, ok := got.(*budgetContextPanicToken); !ok {
						t.Fatalf("context panic=%v", got)
					}
				} else if got != token {
					t.Fatalf("panic=%v want %v", got, token)
				}
				assertTraceParserDetached(t, state.TraceParser(saved), false)
				if r.closed {
					t.Fatal("reader closed")
				}
			})
		}
	})
}

func TestBudgetStreamCompactionChunks(t *testing.T) {
	t.Parallel()
	for _, shift := range []int{1, 127, 128, 129, 512} {
		source := make([]int, 1024)
		for i := range source {
			source[i] = i
		}
		want := append([]int(nil), source...)
		n := copy(want, want[shift:])
		c := state.NewBudgetControl(nil, core.Limits{Compute: uint64(n)})
		if got := state.CopyStreamPrefix(c, source, source[shift:]); got != n || !reflect.DeepEqual(source, want) || c.Used != uint64(n) {
			t.Fatalf("shift=%d result differs", shift)
		}
	}
	v := make([]int, 258)
	for i := range v {
		v[i] = i
	}
	c := state.NewBudgetControl(nil, core.Limits{Compute: 128})
	got := catchStreamPanic(func() { state.CopyStreamPrefix(c, v, v[1:]) })
	f, ok := got.(state.Fatal)
	if !ok || !errors.Is(f.Err, core.ErrLimit) || c.Used != 128 || v[128] != 128 {
		t.Fatalf("copy interruption=%v steps=%d", got, c.Used)
	}
}

func TestBudgetStreamCommitReservations(t *testing.T) {
	t.Parallel()
	// The frame scan and suffix clear are separately counted; neither touches
	// a Node already delivered to the caller.
	node := value.Ptr(value.Data{Text: "delivered"})
	p := &state.Parser{Input: state.Input{Control: state.NewBudgetControl(nil, core.Limits{Compute: 1})}, FrameSlab: make([]state.Frame, 258)}
	p.Frame = &p.FrameSlab[0]
	p.FrameSlab[1].Vals = []*core.Node{node}
	got := catchStreamPanic(func() { p.ResetStreamFrames() })
	if _, ok := got.(state.Fatal); !ok || len(p.FrameSlab) != 258 || p.FrameSlab[1].Vals[0] != node || value.D(node).Text != "delivered" {
		t.Fatalf("frame interruption=%v", got)
	}
	p.Control = state.NewBudgetControl(nil, core.Limits{Compute: 258})
	p.ResetStreamFrames()
	if len(p.FrameSlab) != 1 || p.Control.Used != 258 || value.D(node).Text != "delivered" {
		t.Fatal("frame retry changed ownership or count")
	}

	for _, all := range []bool{false, true} {
		t.Run(fmt.Sprintf("all=%t", all), func(t *testing.T) {
			memo := state.NewMemoTable()
			memo.Slots = make([]*state.MemoEntry, 260)
			a, b, growing := memo.Alloc(), memo.Alloc(), memo.Alloc()
			a.Node, b.Node, growing.Node = node, node, node
			growing.Growing = true
			a.Next = b
			memo.Slots[0] = a
			memo.Slots[1] = growing
			p := &state.Parser{Input: state.Input{Control: state.NewBudgetControl(nil, core.Limits{Compute: 2})}, Memo: memo}
			pos := 129
			if all {
				pos = 260
			}
			got := catchStreamPanic(func() { p.PruneStreamMemo(pos) })
			if _, ok := got.(state.Fatal); !ok || a.Node != nil || b.Node != node || memo.Base != 0 || growing.Node != node {
				t.Fatalf("memo partial prune=%v", got)
			}
			// An interrupted stream has no continuation guarantee: check a new
			// table for complete prune parity, instead of retrying partial links.
			fresh := state.NewMemoTable()
			fresh.Slots = make([]*state.MemoEntry, 260)
			seed := fresh.Alloc()
			seed.Growing = true
			seed.Node = node
			fresh.Slots[1] = seed
			keep := fresh.Alloc()
			keep.Node = node
			fresh.Slots[200] = keep
			p.Memo = fresh
			p.Control = state.NewBudgetControl(nil, core.Limits{Compute: 2000})
			p.PruneStreamMemo(pos)
			if fresh.Base != pos || seed.Node != node || value.D(node).Text != "delivered" || len(fresh.Slots) != max(260-pos, 0) {
				t.Fatal("prune ownership/base differs")
			}
			if !all && fresh.Slots[200-pos] != keep {
				t.Fatal("overlap compaction lost live entry")
			}
		})
	}
}

func TestBudgetStreamCommitAfterDelivery(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, codegentest.Records)
	budgetStreamVariants(t, func(t *testing.T, o ParseOptions) {
		o.Limits.Compute = 1 << 20
		var saved TraceEvent
		o.Trace = func(e TraceEvent) {
			if state.TraceParser(saved) == nil {
				saved = e
			}
		}
		var node *core.Node
		var before string
		count := 0
		var controller *state.BudgetControl
		last := 0
		err := prog.ParseStreamWith("main", strings.NewReader("#records\na=1\nbc=22\n"), func(n *core.Node) error {
			count++
			node = n
			before = codegentest.ResultJSON(n, nil)
			p := state.TraceParser(saved)
			controller = p.Control
			// Force the normal inactive-capacity clear at the next commit.
			p.Trail = make([]state.Undo, 0, 129)
			p.Memo.Base = p.Pos - 1024
			// The charges of the commit, in order and as requested: the line starts it scans
			// and the input it compacts, then, with the frame scratch, the frames it scans for
			// the one in use and those after it that it clears, then, with the undo cleanup, the
			// trail capacity it clears. The reference builds of the stream mechanisms lack one
			// of the last two, so the commit ends at another charge there.
			var charges []int
			charge := func(n, chunk int) {
				for ; n > 0; n -= chunk {
					charges = append(charges, min(n, chunk))
				}
			}
			copyCount := 0
			if removed := p.Pos - 1 - p.Base; removed > 0 && 2*removed >= len(p.In)+len(p.Bs) {
				copyCount = len(p.In) + len(p.Bs) - removed
			}
			lineSteps := 0
			if copyCount > 0 {
				if p.Eof && p.Base == 0 {
					if p.Lines == nil {
						lineSteps = p.Loaded()
					}
				} else {
					lineSteps = min(p.Pos-1, p.Loaded()) - p.Base
				}
			}
			charge(lineSteps, 1)
			charge(copyCount, 128)
			if state.StreamFrameScratch {
				keep := 0
				for i := range p.FrameSlab {
					charge(1, 1)
					if &p.FrameSlab[i] == p.Frame {
						keep = i + 1
						break
					}
				}
				charge(len(p.FrameSlab)-keep, 128)
			}
			if state.StreamUndoCleanup {
				charge(cap(p.Trail), 128)
			}
			if len(charges) == 0 {
				t.Fatal("the commit charges nothing")
			}
			// The limit lets every charge but the last through, so the commit is interrupted at
			// its end, after it has released what the element held.
			last = charges[len(charges)-1]
			total := 0
			for _, n := range charges {
				total += n
			}
			controller.Limits.Compute = controller.Used + uint64(total-last)
			return nil
		}, o)
		var we *core.LimitError
		if !errors.As(err, &we) || we.Kind != core.LimitCompute || we.Requested != uint64(last) || we.Used != controller.Limits.Compute || count != 1 {
			t.Fatalf("normal commit interruption error=%v emits=%d", err, count)
		}
		assertTraceParserDetached(t, state.TraceParser(saved), false)
		if node == nil || codegentest.ResultJSON(node, nil) != before {
			t.Fatal("commit changed emitted Node")
		}
		o.Limits = core.Limits{}
		o.Trace = nil
		replay := 0
		if err := prog.ParseStreamWith("main", strings.NewReader("#records\na=1\nbc=22\n"), func(*core.Node) error { replay++; return nil }, o); err != nil || replay != 2 || codegentest.ResultJSON(node, nil) != before {
			t.Fatalf("fresh replay error=%v emits=%d", err, replay)
		}
	})
}

func TestBudgetStreamEmitBoundary(t *testing.T) {
	t.Parallel()
	for _, phase := range []string{"before", "after", "error"} {
		t.Run(phase, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			p := &state.Parser{Input: state.Input{Control: state.NewBudgetControl(ctx, core.Limits{})}}
			stop := errors.New("emit")
			calls := 0
			p.Emit = func(*core.Node) error {
				calls++
				cancel()
				if phase == "error" {
					return stop
				}
				return nil
			}
			if phase == "before" {
				cancel()
			}
			got := catchStreamPanic(func() { p.EmitStream(nil, 0) })
			f, ok := got.(state.Fatal)
			if phase == "error" {
				if !ok || f.Err != stop || calls != 1 || p.EmitCall {
					t.Fatalf("panic=%v calls=%d caller flag=%t", got, calls, p.EmitCall)
				}
				return
			}
			wantCalls := 1
			if phase == "before" {
				wantCalls = 0
			}
			if !ok || f.Err != context.Canceled || calls != wantCalls || p.EmitCall {
				t.Fatalf("panic=%v calls=%d caller flag=%t", got, calls, p.EmitCall)
			}
		})
	}
}

func TestBudgetStreamReaderPanicInsideTrace(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, `rule main = "z"* #stream $$`)
	budgetStreamVariants(t, func(t *testing.T, o ParseOptions) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		o.Context = ctx
		token := &streamRuntimePanicToken{phase: "trace reader"}
		r := &streamCallbackReader{panicValue: token}
		var saved TraceEvent
		o.Trace = func(e TraceEvent) {
			if e.Kind == TraceEnter && e.Rule == "main" {
				saved = e
				e.LineCol(1)
			}
		}
		got := catchStreamPanic(func() { _ = prog.ParseStreamWith("main", r, nil, o) })
		if got != token || r.closed {
			t.Fatalf("panic=%v closed=%t", got, r.closed)
		}
		assertTraceParserDetached(t, state.TraceParser(saved), false)
	})
}

func TestBudgetStreamVMFaultAttribution(t *testing.T) {
	t.Parallel()
	for _, b := range []Backend{Bytecode, BytecodeIterative} {
		t.Run(b.String(), func(t *testing.T) {
			prog := mustCompile(t, `rule main = "z"* #stream $$`)
			if _, err := prog.rule(b, "main", state.Inlined, nil); err != nil {
				t.Fatal(err)
			}
			restore := corruptVMLiteral(t, prog)
			defer restore()
			var saved TraceEvent
			err := prog.ParseStreamWith("main", strings.NewReader("z"), func(*core.Node) error { t.Fatal("emitted from corrupt VM"); return nil }, ParseOptions{Backend: b, Limits: core.Limits{Compute: 1 << 20}, Trace: func(e TraceEvent) {
				if state.TraceParser(saved) == nil {
					saved = e
				}
			}})
			if err == nil || !strings.HasPrefix(err.Error(), "invalid bytecode:") {
				t.Fatalf("VM error=%v", err)
			}
			assertTraceParserDetached(t, state.TraceParser(saved), false)
			restore()
			if err := prog.ParseStreamWith("main", strings.NewReader("z"), nil, ParseOptions{Backend: b}); err != nil {
				t.Fatalf("restored VM: %v", err)
			}
		})
	}
}

func TestBudgetStreamConcurrentCounters(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, codegentest.Records)
	budgetStreamVariants(t, func(t *testing.T, base ParseOptions) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		type result struct {
			control *state.BudgetControl
			err     error
			count   int
		}
		start := make(chan struct{})
		results := make(chan result, 2)
		for i := range 2 {
			go func() {
				<-start
				o := base
				o.Limits = core.Limits{Compute: 100000}
				if i == 0 {
					o.Context = ctx
				}
				var run result
				o.Trace = func(e TraceEvent) {
					if e.Kind == TraceEnter && e.Rule == "main" {
						run.control = state.TraceParser(e).Control
						if run.control.Used > 1 { // at most the entry of the rule
							panic("stream inherited counters")
						}
					}
				}
				run.err = prog.ParseStreamWith("main", strings.NewReader("#records\na=1\nbc=22\n"), func(*core.Node) error {
					run.count++
					if i == 0 {
						cancel()
					}
					return nil
				}, o)
				results <- run
			}()
		}
		close(start)
		a, b := <-results, <-results
		if a.control == nil || b.control == nil || a.control == b.control || a.control.Used == 0 || b.control.Used == 0 {
			t.Fatal("concurrent invocation counters shared or empty")
		}
		if a.err == nil {
			a, b = b, a
		}
		if a.err != context.Canceled || a.count != 1 || b.err != nil || b.count != 2 {
			t.Fatalf("concurrent results: %v/%d, %v/%d", a.err, a.count, b.err, b.count)
		}
	})
}
