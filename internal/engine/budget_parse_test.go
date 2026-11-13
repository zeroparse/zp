//go:build !zp_budget_controls_reference

package engine

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/engine/state"
)

func TestBudgetParsePreCancelledAndValidation(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, `rule main = "ok" $$`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
		calls := 0
		opts := ParseOptions{Backend: b, Context: ctx, Trace: func(TraceEvent) { calls++ }}
		n, err := prog.ParseWith("main", "ok", opts)
		if n != nil || !errors.Is(err, context.Canceled) || calls != 0 {
			t.Fatalf("%v: node=%v err=%v calls=%d", b, n, err, calls)
		}
		_, err = prog.ParseWith("undefined", "ok", opts)
		if err == nil || errors.Is(err, context.Canceled) {
			t.Fatalf("%v: rule validation error=%v", b, err)
		}
	}
}

func TestBudgetParseBudgetAndPoolRetry(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, `rule main = [a-z]+ #atomic $$`)
	for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
		for _, u := range []core.PositionUnit{core.Bytes, core.CodePoints} {
			for _, lim := range []core.Limits{{Compute: 1}, {Compute: 1}} {
				n, err := prog.ParseWith("main", strings.Repeat("a", 10000), ParseOptions{Backend: b, Unit: u, Limits: lim})
				if n != nil || !errors.Is(err, core.ErrLimit) {
					t.Fatalf("%v/%v/%v: node=%v err=%v", b, u, lim, n, err)
				}
				n, err = prog.ParseWith("main", "abc", ParseOptions{Backend: b, Unit: u})
				if n == nil || err != nil {
					t.Fatalf("%v/%v retry: node=%v err=%v", b, u, n, err)
				}
			}
		}
	}
}

func TestBudgetParseCancelledFromTraceAndPanicCleanup(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, `rule main = "abc" $$`)
	for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
		ctx, cancel := context.WithCancel(context.Background())
		var observed *state.Parser
		n, err := prog.ParseWith("main", "abc", ParseOptions{Backend: b, Context: ctx, Trace: func(e TraceEvent) { observed = state.TraceParser(e); cancel() }})
		if n != nil || !errors.Is(err, context.Canceled) || observed == nil || observed.Scratch != nil || observed.Control != nil {
			t.Fatalf("%v: cancellation/cleanup node=%v err=%v parser=%v", b, n, err, observed)
		}
		token := &struct{ label string }{"caller panic"}
		var got any
		func() {
			defer func() { got = recover() }()
			_, _ = prog.ParseWith("main", "abc", ParseOptions{Backend: b, Trace: func(e TraceEvent) { observed = state.TraceParser(e); panic(token) }})
		}()
		if got != token || observed.Scratch != nil || observed.Control != nil {
			t.Fatalf("%v: panic=%v scratch=%v control=%v", b, got, observed.Scratch, observed.Control)
		}
	}
}

func TestBudgetParseLongLiteralAndScanner(t *testing.T) {
	t.Parallel()
	literal := strings.Repeat("a", 10000)
	for _, src := range []string{`rule main = "` + literal + `" $$`, `rule main = [a-z]+ #atomic $$`} {
		prog := mustCompile(t, src)
		for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
			ctx, cancel := context.WithCancel(context.Background())
			calls := 0
			n, err := prog.ParseWith("main", literal, ParseOptions{Backend: b, Unit: core.Bytes, Context: ctx, Trace: func(e TraceEvent) {
				if e.Kind == TraceEnter {
					calls++
					state.TraceParser(e).Control.Ctx = newPollingContext(2)
				}
			}})
			cancel()
			if n != nil || !errors.Is(err, context.Canceled) || calls == 0 {
				t.Fatalf("%v scanner: node=%v err=%v calls=%d", b, n, err, calls)
			}
		}
	}
}

func TestBudgetParseRecognitionAndStatefulAdmission(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, `rule main = "abc" $$`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
		n, err := prog.ParseWith("main", "abc", ParseOptions{Backend: b, Recognize: true, Context: ctx})
		if n != nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("%v recognition: node=%v err=%v", b, n, err)
		}
	}
	if d, err := prog.NewDocumentWith("main", "abc", ParseOptions{Context: ctx}); d != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("document ignored construction cancellation: document=%v error=%v", d, err)
	}
	stream := mustCompile(t, `rule main = "abc"* #stream $$`)
	if err := stream.ParseStreamWith("main", strings.NewReader("abc"), nil, ParseOptions{Context: ctx}); err != context.Canceled {
		t.Fatalf("stream ignored admission cancellation: %v", err)
	}
}

func TestBudgetParseFinalPublicationAndRecovery(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, `rule main = "abc" $$`)
	for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
		ctx, cancel := context.WithCancel(context.Background())
		n, err := prog.ParseWith("main", "abc", ParseOptions{Backend: b, Context: ctx, Trace: func(e TraceEvent) {
			if e.Kind == TraceExit {
				cancel()
			}
		}})
		if n != nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("%v publish: node=%v err=%v", b, n, err)
		}
	}
	recovery := mustCompile(t, `rule main = item+ $$

rule item = "a" ";" #recover(skip=[^;]* ";")`)
	n, err := recovery.ParseWith("main", strings.Repeat("x;", 1000), ParseOptions{Limits: core.Limits{Compute: 5}})
	if n != nil || !errors.Is(err, core.ErrLimit) {
		t.Fatalf("recovery swallowed interruption: node=%v err=%v", n, err)
	}
}

func TestBudgetParseTraceAccessorInterruption(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, `rule main = "abc" $$`)
	for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
		var observed *state.Parser
		n, err := prog.ParseWith("main", "abc", ParseOptions{Backend: b, Limits: core.Limits{Compute: 100}, Trace: func(e TraceEvent) {
			observed = state.TraceParser(e)
			state.TraceParser(e).Control.Limits.Compute = state.TraceParser(e).Control.Used
			e.LineCol(1) // Building the first line table is engine execution inside caller code.
		}})
		if n != nil || !errors.Is(err, core.ErrLimit) || observed == nil || observed.Scratch != nil {
			t.Fatalf("%v accessor interruption: node=%v err=%v parser=%v", b, n, err, observed)
		}
	}
}

func TestBudgetParseLeftRecursionAndIndependentNestedCall(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, `rule main = expr $$

rule expr = expr "+" "a" / "a"`)
	for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
		ctx, cancel := context.WithCancel(context.Background())
		entries := 0
		n, err := prog.ParseWith("main", "a+a+a+a", ParseOptions{Backend: b, Context: ctx, Trace: func(e TraceEvent) {
			if e.Kind == TraceEnter && e.Rule == "expr" {
				entries++
				if entries == 3 {
					cancel()
				}
			}
		}})
		if n != nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("%v LR: node=%v err=%v", b, n, err)
		}
		n, err = prog.ParseWith("main", "a+a+a+a", ParseOptions{Backend: b, Limits: core.Limits{Compute: 10000}})
		if n == nil || err != nil {
			t.Fatalf("%v LR retry: node=%v err=%v", b, n, err)
		}
		nested := false
		n, err = prog.ParseWith("main", "a", ParseOptions{Backend: b, Limits: core.Limits{Compute: 10000}, Trace: func(e TraceEvent) {
			if nested {
				return
			}
			nested = true
			_, inner := prog.ParseWith("main", "a", ParseOptions{Backend: b, Context: ctx})
			if !errors.Is(inner, context.Canceled) {
				t.Errorf("%v inner: %v", b, inner)
			}
		}})
		if n == nil || err != nil {
			t.Fatalf("%v independent outer: node=%v err=%v", b, n, err)
		}
	}
}

// Controlled preparation has its own decoder; ordinary conformance tests alone
// do not enter it. Compare full values/spans and error fields, not formatting.
func TestBudgetParseControlledUnicodeParity(t *testing.T) {
	t.Parallel()
	inputs := []string{"é💡x", "é💡\n字", "\xff", "\xc3\xff", "\xf0\x9f", "�\xff", ""}
	for _, src := range []string{`rule main = .* $$`, `rule main = "é💡" "x" $$`} {
		prog := mustCompile(t, src)
		for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
			for _, u := range []core.PositionUnit{core.Bytes, core.CodePoints} {
				for _, recognize := range []bool{false, true} {
					for _, input := range inputs {
						opts := ParseOptions{Backend: b, Unit: u, Recognize: recognize}
						baseline, baseErr := prog.ParseWith("main", input, opts)
						opts.Limits = core.Limits{Compute: 100000}
						got, err := prog.ParseWith("main", input, opts)
						if !reflect.DeepEqual(got, baseline) || !reflect.DeepEqual(err, baseErr) {
							t.Fatalf("%v/%v/recognize=%v input=%q: controlled=%#v/%#v baseline=%#v/%#v", b, u, recognize, input, got, err, baseline, baseErr)
						}
					}
				}
			}
		}
	}
}
