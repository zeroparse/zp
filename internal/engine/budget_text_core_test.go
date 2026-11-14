//go:build !zp_budget_controls_reference

package engine

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/engine/state"
)

func TestBudgetTextFallbackBudgetCore(t *testing.T) {
	t.Parallel()
	for _, u := range []core.PositionUnit{core.Bytes, core.CodePoints} {
		in := state.Input{Unit: u, Bs: make([]byte, 8192), In: make([]rune, 256), Control: state.NewBudgetControl(nil, core.Limits{Compute: 1})}
		end := len(in.Bs)
		if u == core.CodePoints {
			end = len(in.In)
		}
		if err := catchFatal(t, func() { _ = in.Text(0, end) }); !errors.Is(err, core.ErrLimit) {
			t.Fatalf("%v materialization error=%v, want compute limit", u, err)
		}
	}
}

func TestBudgetTextSizingOverflow(t *testing.T) {
	t.Parallel()
	maxInt := int(^uint(0) >> 1)
	if got := state.CheckedTextSize(maxInt-3, 3); got != maxInt {
		t.Fatalf("exact maximum=%d", got)
	}
	for _, args := range [][2]int{{maxInt, 1}, {maxInt - 2, 3}, {-1, 1}, {0, -1}} {
		if err := catchFatal(t, func() { state.CheckedTextSize(args[0], args[1]) }); err == nil {
			t.Fatalf("sizing %v did not fail", args)
		}
	}
}

func TestBudgetTextBatchPredicateAndTrace(t *testing.T) {
	t.Parallel()
	source := strings.Repeat("é😀\xff", 80)
	for _, traceText := range []bool{false, true} {
		grammar := `rule main = words:piece {text($words) != ""} $$

rule piece = .*`
		if traceText {
			grammar = `rule main = piece $$

rule piece = .*`
		}
		prog := mustCompile(t, grammar)
		for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
			for _, u := range []core.PositionUnit{core.Bytes, core.CodePoints} {
				retained, err := prog.ParseWith("main", strings.Repeat("oldλ\xff", 90), ParseOptions{Backend: b, Unit: u, Limits: core.Limits{Compute: 1 << 50}})
				if err != nil || retained == nil {
					t.Fatalf("retained parse %v/%v: %v", b, u, err)
				}
				before, _ := json.Marshal(retained)
				for _, recognize := range []bool{false, true} {
					for _, cancel := range []bool{false, true} {
						if cancel && u == core.Bytes {
							continue
						}
						var observed *state.Parser
						armed := false
						n, err := prog.ParseWith("main", source, ParseOptions{Backend: b, Unit: u, Recognize: recognize, Limits: core.Limits{Compute: 1 << 50}, Trace: func(e core.TraceEvent) {
							observed = state.TraceParser(e)
							if e.Kind != core.TraceExit || e.Rule != "piece" || !e.Matched {
								return
							}
							armed = true
							if cancel {
								state.TraceParser(e).Control.SinceCheck = 0
								polls := 2
								if traceText {
									polls = 1
								}
								state.TraceParser(e).Control.Ctx = newPollingContext(polls)
							} else if u == core.CodePoints {
								// A source substring of Bytes charges no input, so only
								// code points see the tightened budget.
								state.TraceParser(e).Control.Limits.Compute = state.TraceParser(e).Control.Used + 16
							}
							if traceText {
								_ = e.Text()
							}
						}})
						if !armed || observed == nil || observed.Scratch != nil || observed.Control != nil {
							t.Fatalf("%v/%v trace=%t recognition=%t cancel=%t: trace/cleanup failed", b, u, traceText, recognize, cancel)
						}
						// A parse that builds the text of each matched character decodes the
						// input at the first of them, before the hook arms: every parse that
						// builds nodes, and a recognition inside the capture of the predicate,
						// whose value it needs. Later texts are substrings of the decoded input
						// that charge nothing, so only cancellation, which the rest of the parse
						// still polls, interrupts such a parse; the text of the trace hook in a
						// recognition is the first text and is interrupted while decoding.
						decodedEarly := !cancel && (!recognize || !traceText)
						if u == core.CodePoints && !decodedEarly {
							target := core.ErrLimit
							if cancel {
								target = context.Canceled
							}
							if n != nil || !errors.Is(err, target) {
								t.Fatalf("%v/%v trace=%t recognition=%t cancel=%t: n=%v err=%v want %v", b, u, traceText, recognize, cancel, n, err, target)
							}
						} else if err != nil || (!recognize && n == nil) {
							t.Fatalf("source substring %v: n=%v err=%v", b, n, err)
						}
						n, err = prog.ParseWith("main", source, ParseOptions{Backend: b, Unit: u, Recognize: recognize})
						if err != nil || (!recognize && n == nil) || (recognize && n != nil) {
							t.Fatalf("%v/%v retry: n=%v err=%v", b, u, n, err)
						}
						after, _ := json.Marshal(retained)
						if string(after) != string(before) {
							t.Fatalf("%v/%v retained tree changed", b, u)
						}
					}
				}
			}
		}
	}
}

func TestBudgetTextStepBudgetAndLazyOffsets(t *testing.T) {
	t.Parallel()
	for _, u := range []core.PositionUnit{core.Bytes, core.CodePoints} {
		in := state.Input{Unit: u, Bs: make([]byte, 4097), In: make([]rune, 129), Control: state.NewBudgetControl(nil, core.Limits{Compute: 128})}
		end := len(in.In)
		if u == core.Bytes {
			end = len(in.Bs)
			in.Control.Limits.Compute = 1
		}
		err := catchFatal(t, func() { _ = in.Text(0, end) })
		var limit *core.LimitError
		if !errors.As(err, &limit) || limit.Kind != core.LimitCompute {
			t.Fatalf("%v step error=%v", u, err)
		}
	}
	source := "aé😀z"
	in := state.Input{Unit: core.CodePoints, Src: source, SrcOK: true, In: []rune(source), Control: state.NewBudgetControl(nil, core.Limits{Compute: 1})}
	if err := catchFatal(t, func() { _ = in.Text(1, 3) }); !errors.Is(err, core.ErrLimit) || in.Offs != nil {
		t.Fatalf("lazy offsets error=%v offsets=%v", err, in.Offs)
	}
	in.Control = state.NewBudgetControl(nil, core.Limits{Compute: 100})
	if got := in.Text(1, 3); got != "é😀" {
		t.Fatalf("offset retry=%q", got)
	}
}

func TestBudgetTextNodeRoutesParity(t *testing.T) {
	t.Parallel()
	input := strings.Repeat("é\x00😀\xff", 40)
	for _, src := range []string{`rule main = .* #atomic $$`, `type Token = { Text string }

rule main Token = t:(.* $$) #atomic => Token{Text: text($t)}`, `rule main = item $$

rule item = "!" #recover(skip=.*)`} {
		prog := mustCompile(t, src)
		for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
			for _, u := range []core.PositionUnit{core.Bytes, core.CodePoints} {
				for _, recognize := range []bool{false, true} {
					opts := ParseOptions{Backend: b, Unit: u, Recognize: recognize}
					want, wantErr := prog.ParseWith("main", input, opts)
					opts.Limits = core.Limits{Compute: 1 << 50}
					got, gotErr := prog.ParseWith("main", input, opts)
					if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(gotErr, wantErr) {
						t.Fatalf("%v/%v recognition=%t node/error differs for %s", b, u, recognize, src)
					}
				}
			}
		}
	}
}
