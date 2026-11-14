//go:build !zp_budget_controls_reference

package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/engine/state"
	"zeroparse.dev/zp/internal/value"
)

func actionBudgetParser(ctx context.Context, limits core.Limits, unit core.PositionUnit) *state.Parser {
	return &state.Parser{Input: state.Input{Unit: unit, Control: state.NewBudgetControl(ctx, limits)}}
}

func actionBudgetNodes(n int) []*core.Node {
	items := make([]*core.Node, n)
	for i := range items {
		items[i] = value.Ptr(value.Data{Start: int32(i), End: int32(i + 1), Terminal: true, Text: "x"})
	}
	return items
}

func TestBudgetActionStringHelperParity(t *testing.T) {
	t.Parallel()
	malformed := string([]byte{'a', 0xff, 0xc3, 0x80, 'z'})
	left := strings.Repeat("prefix-", 2048) + "a" + string([]byte{0xff})
	right := strings.Repeat("prefix-", 2048) + "b" + string([]byte{0x00})

	for _, unit := range []core.PositionUnit{core.Bytes, core.CodePoints} {
		p := actionBudgetParser(nil, core.Limits{Compute: 1 << 40}, unit)
		wantLen := len(malformed)
		if unit == core.CodePoints {
			wantLen = utf8.RuneCountInString(malformed)
		}
		if got := p.ValueTextLen(malformed); got != wantLen {
			t.Errorf("%v valueTextLen(%q) = %d, want %d", unit, malformed, got, wantLen)
		}
		if got, want := p.CompareText(left, right), strings.Compare(left, right); got != want {
			t.Errorf("%v compareText(long equal prefix) = %d, want %d", unit, got, want)
		}
		if got := p.CompareText(malformed, malformed); got != 0 {
			t.Errorf("%v compareText(malformed equal strings) = %d, want 0", unit, got)
		}
		joined, err := p.ValueBinary("+", malformed, string([]byte{0xfe, 'q'}))
		if err != nil {
			t.Fatalf("%v string addition: %v", unit, err)
		}
		if got, want := []byte(joined.(string)), append([]byte(malformed), 0xfe, 'q'); string(got) != string(want) {
			t.Errorf("%v string addition bytes = %v, want %v", unit, got, want)
		}
		if got, err := p.ValueBinary("<", left, right); err != nil || got != true {
			t.Errorf("%v lexical binary comparison = %v, %v; want true", unit, got, err)
		}
		if got, err := p.ValueBinary("+", 20, 22); err != nil || got != 42 {
			t.Errorf("%v integer addition = %v, %v; want 42", unit, got, err)
		}
		if p.Control.Used == 0 {
			t.Fatalf("scalar budget counter = %d, want the helpers to spend budget", p.Control.Used)
		}
	}
}

func TestBudgetActionStringHelpersChargeLongBudget(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("a", 8192)
	p := actionBudgetParser(nil, core.Limits{Compute: 256}, core.CodePoints)
	if err := catchFatal(t, func() { p.CompareText(long+"x", long+"y") }); !errors.Is(err, core.ErrLimit) {
		t.Fatalf("long equal-prefix comparison error = %v, want ErrLimit", err)
	}

	p = actionBudgetParser(nil, core.Limits{Compute: 128}, core.Bytes)
	if err := catchFatal(t, func() {
		_, _ = p.ValueBinary("+", long, long)
	}); !errors.Is(err, core.ErrLimit) {
		t.Fatalf("long string concatenation error = %v, want ErrLimit", err)
	}

	p = actionBudgetParser(nil, core.Limits{Compute: 128}, core.CodePoints)
	if err := catchFatal(t, func() { p.ValueTextLen(long) }); !errors.Is(err, core.ErrLimit) {
		t.Fatalf("long rune length error = %v, want ErrLimit", err)
	}
}

func TestBudgetActionCollectionLoopsChargeElements(t *testing.T) {
	t.Parallel()
	const size = 2048
	items := actionBudgetNodes(size)

	cases := []struct {
		name string
		run  func(*state.EvalCtx) error
	}{
		{"newList", func(c *state.EvalCtx) error { _ = c.NewList(items); return nil }},
		{"listNode", func(c *state.EvalCtx) error { _ = c.ListNode(items); return nil }},
		{"local copy", func(c *state.EvalCtx) error { _ = c.Keep(); return nil }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := newPollingContext(1)
			p := actionBudgetParser(ctx, core.Limits{}, core.CodePoints)
			var locals *state.Local
			if tc.name == "local copy" {
				for i := 0; i < size; i++ {
					locals = &state.Local{Name: fmt.Sprintf("v%d", i), Val: i, Next: locals}
				}
			}
			c := &state.EvalCtx{P: p, Locals: locals}
			var returned error
			fatalErr := catchFatal(t, func() { returned = tc.run(c) })
			if returned != nil {
				t.Fatalf("operation returned ordinary error: %v", returned)
			}
			if !errors.Is(fatalErr, context.Canceled) {
				t.Fatalf("operation panic error = %v, want context.Canceled", fatalErr)
			}
		})
	}
}

func TestBudgetActionBatchRoutesAndPoolRetry(t *testing.T) {
	t.Parallel()
	routes := []struct {
		name   string
		action string
	}{
		{"map", `map($parts, (x) => x)`},
		{"foldl", `foldl(nil, $parts, (acc, x) => acc)`},
		{"foldr", `foldr(nil, $parts, (acc, x) => acc)`},
		{"concat", `concat($parts)`},
	}
	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			prog := mustCompile(t, `rule main = parts:items => `+route.action+`
rule items = [a-z] #atomic*`)
			input := strings.Repeat("a", 1024)
			for _, backend := range []Backend{Closure, Bytecode, BytecodeIterative} {
				for _, unit := range []core.PositionUnit{core.Bytes, core.CodePoints} {
					t.Run(fmt.Sprintf("%v/%v", backend, unit), func(t *testing.T) {
						armed := false
						n, err := prog.ParseWith("main", input, ParseOptions{Backend: backend, Unit: unit, Trace: func(e TraceEvent) {
							if !armed && e.Kind == TraceExit && e.Rule == "items" {
								armed = true
								state.TraceParser(e).Control = state.NewBudgetControl(nil, core.Limits{Compute: 128})
							}
						}})
						if !armed || n != nil || !errors.Is(err, core.ErrLimit) {
							t.Fatalf("controlled parse: armed=%t node=%v err=%v", armed, n, err)
						}
						n, err = prog.ParseWith("main", input, ParseOptions{Backend: backend, Unit: unit})
						if err != nil {
							t.Fatalf("pooled retry: %v", err)
						}
					})
				}
			}
		})
	}
}

func TestBudgetActionBatchCancellationStopsPublication(t *testing.T) {
	t.Parallel()
	for _, action := range []string{`map($parts, (x) => x)`, `concat($parts)`} {
		t.Run(action, func(t *testing.T) { batchCancellationStopsPublication(t, action) })
	}
}

func batchCancellationStopsPublication(t *testing.T, action string) {
	t.Helper()
	prog := mustCompile(t, `rule main = parts:items => `+action+`

rule items = [a-z] #atomic*`)
	input := strings.Repeat("a", 1024)
	for _, backend := range []Backend{Closure, Bytecode, BytecodeIterative} {
		for _, unit := range []core.PositionUnit{core.Bytes, core.CodePoints} {
			t.Run(fmt.Sprintf("%v/%v", backend, unit), func(t *testing.T) {
				armed := false
				var attempt *state.BudgetControl
				n, err := prog.ParseWith("main", input, ParseOptions{Backend: backend, Unit: unit, Trace: func(e TraceEvent) {
					if !armed && e.Kind == TraceExit && e.Rule == "items" {
						armed = true
						attempt = state.NewBudgetControl(newPollingContext(2), core.Limits{})
						state.TraceParser(e).Control = attempt
					}
				}})
				if !armed || n != nil || !errors.Is(err, context.Canceled) || attempt.Used < 128 {
					t.Fatalf("controlled parse: armed=%t node=%v err=%v, want cancellation and no result", armed, n, err)
				}
				n, err = prog.ParseWith("main", "abc", ParseOptions{Backend: backend, Unit: unit})
				if err != nil || n == nil {
					t.Fatalf("pooled retry: node=%v err=%v", n, err)
				}
			})
		}
	}
}

func TestBudgetActionShortCircuitAndRecoveryPropagation(t *testing.T) {
	t.Parallel()
	shortCircuit := mustCompile(t, `rule main = words:items {true || text($words) == text($words)} $$

rule items = [a-z]* #atomic`)
	recovery := mustCompile(t, `rule main = (words:items {text($words) == text($words) + ""} "!") #recover(skip=[^;]* ";") $$

rule items = [a-z]* #atomic`)
	input := strings.Repeat("a", 1024) + "!"

	for _, backend := range []Backend{Closure, Bytecode, BytecodeIterative} {
		for _, unit := range []core.PositionUnit{core.Bytes, core.CodePoints} {
			t.Run(fmt.Sprintf("%v/%v", backend, unit), func(t *testing.T) {
				armContext := func(e TraceEvent, armed *bool) {
					if !*armed && e.Kind == TraceExit && e.Rule == "items" {
						*armed = true
						state.TraceParser(e).Control = state.NewBudgetControl(nil, core.Limits{Compute: 128})
					}
				}
				armed := false
				n, err := shortCircuit.ParseWith("main", strings.TrimSuffix(input, "!"), ParseOptions{Backend: backend, Unit: unit, Trace: func(e TraceEvent) { armContext(e, &armed) }})
				if !armed || err != nil || n == nil {
					t.Fatalf("short-circuit parse: armed=%t node=%v err=%v", armed, n, err)
				}

				armed = false
				n, err = recovery.ParseWith("main", input, ParseOptions{Backend: backend, Unit: unit, Trace: func(e TraceEvent) {
					if !armed && e.Kind == TraceExit && e.Rule == "items" {
						armed = true
						state.TraceParser(e).Control = state.NewBudgetControl(nil, core.Limits{Compute: 256})
					}
				}})
				if !armed || n != nil || !errors.Is(err, core.ErrLimit) {
					t.Fatalf("recovery swallowed action interruption: armed=%t node=%v err=%v", armed, n, err)
				}
				n, err = recovery.ParseWith("main", input, ParseOptions{Backend: backend, Unit: unit})
				if err != nil || n == nil {
					t.Fatalf("recovery retry: node=%v err=%v", n, err)
				}
			})
		}
	}
}
