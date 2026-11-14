//go:build !zp_budget_controls_reference

package engine

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/codegentest"
	"zeroparse.dev/zp/internal/engine/state"
)

type setupPhaseContext struct {
	context.Context
	cancel    context.CancelFunc
	ready     func() bool
	triggered bool
}

func newSetupPhaseContext(ready func() bool) *setupPhaseContext {
	ctx, cancel := context.WithCancel(context.Background())
	return &setupPhaseContext{Context: ctx, cancel: cancel, ready: ready}
}

func (c *setupPhaseContext) Err() error {
	if !c.triggered && c.ready() {
		c.triggered = true
		c.cancel()
	}
	return c.Context.Err()
}

type setupCountingReader struct {
	r     io.Reader
	reads int
}

func (r *setupCountingReader) Read(p []byte) (int, error) {
	r.reads++
	return r.r.Read(p)
}

// setupProgram compiles src; with moduleOnly it returns a program built from the compiled module
// alone, without the grammar, which runs only on the bytecode backends.
func setupProgram(t *testing.T, src string, moduleOnly bool) *Program {
	t.Helper()
	p := mustCompile(t, src)
	if moduleOnly {
		return moduleProgram(p.Module())
	}
	return p
}

func setupBackendReady(p *Program, backend Backend) bool {
	switch backend {
	case Bytecode:
		return p.preparedVM(Bytecode) != nil
	case BytecodeIterative:
		return p.preparedVM(BytecodeIterative) != nil
	default:
		return true
	}
}

func assertSetupBackendState(t *testing.T, p *Program, backend Backend, ready bool) {
	t.Helper()
	if got := setupBackendReady(p, backend); got != ready {
		t.Fatalf("backend %v ready=%t, want %t", backend, got, ready)
	}
	if backend == Bytecode && p.preparedVM(BytecodeIterative) != nil || backend == BytecodeIterative && p.preparedVM(Bytecode) != nil {
		t.Fatalf("unused VM was prepared: vm=%p ivm=%p", p.preparedVM(Bytecode), p.preparedVM(BytecodeIterative))
	}
}

func setupDonePanicContext(token *budgetContextPanicToken) context.Context {
	return &budgetContextDonePanic{Context: context.Background(), token: token}
}

func TestBudgetSetupValidationPrecedesContextMethods(t *testing.T) {
	t.Parallel()
	ordinary := `rule main = "x" $$`
	stream := `rule main = "x"* #stream $$`
	for _, moduleOnly := range []bool{false, true} {
		for _, backend := range []Backend{Closure, Bytecode, BytecodeIterative, Backend(99)} {
			t.Run(setupProgramLabel(moduleOnly)+"/"+backend.String(), func(t *testing.T) {
				p := setupProgram(t, ordinary, moduleOnly)
				ctxToken := &budgetContextPanicToken{phase: "validation accessed Context.Done"}
				ctx := setupDonePanicContext(ctxToken)
				opts := ParseOptions{Backend: backend, Context: ctx}
				var returnedErr error
				got := budgetContextRecover(func() { _, returnedErr = p.ParseWith("missing", "x", opts) })
				if got != nil || returnedErr == nil {
					t.Fatalf("ParseWith missing rule panic=%v err=%v", got, returnedErr)
				}
				if backend == Backend(99) {
					got = budgetContextRecover(func() { _, returnedErr = p.ParseWith("main", "x", opts) })
					if got != nil || returnedErr == nil {
						t.Fatalf("ParseWith invalid backend panic=%v err=%v", got, returnedErr)
					}
				}
				got = budgetContextRecover(func() {
					_, returnedErr = p.NewDocumentWith("main", "x", ParseOptions{Backend: backend, Recognize: true, Context: ctx})
				})
				if got != nil || returnedErr == nil {
					t.Fatalf("NewDocumentWith recognition rejection panic=%v err=%v", got, returnedErr)
				}
				if moduleOnly {
					got = budgetContextRecover(func() {
						_, returnedErr = p.ParseWith("main", "x", ParseOptions{Backend: Closure, Context: ctx})
					})
					if got != nil || returnedErr == nil {
						t.Fatalf("AST-moduleOnly Closure panic=%v err=%v", got, returnedErr)
					}
				}

				got = budgetContextRecover(func() { _, returnedErr = p.NewDocumentWith("missing", "x", opts) })
				if got != nil || returnedErr == nil {
					t.Fatalf("NewDocumentWith missing rule panic=%v err=%v", got, returnedErr)
				}
				if backend == Backend(99) {
					got = budgetContextRecover(func() { _, returnedErr = p.NewDocumentWith("main", "x", opts) })
					if got != nil || returnedErr == nil {
						t.Fatalf("NewDocumentWith invalid backend panic=%v err=%v", got, returnedErr)
					}
				}

				streamProgram := setupProgram(t, stream, moduleOnly)
				got = budgetContextRecover(func() {
					returnedErr = streamProgram.ParseStreamWith("missing", strings.NewReader("x"), nil, opts)
				})
				if got != nil || returnedErr == nil {
					t.Fatalf("ParseStreamWith missing rule panic=%v err=%v", got, returnedErr)
				}
				if backend == Backend(99) {
					got = budgetContextRecover(func() {
						returnedErr = streamProgram.ParseStreamWith("main", strings.NewReader("x"), nil, opts)
					})
					if got != nil || returnedErr == nil {
						t.Fatalf("ParseStreamWith invalid backend panic=%v err=%v", got, returnedErr)
					}
				}
				got = budgetContextRecover(func() {
					returnedErr = streamProgram.ParseStreamWith("main", strings.NewReader("x"), nil, ParseOptions{Backend: backend, Recognize: true, Context: ctx})
				})
				if got != nil || returnedErr == nil {
					t.Fatalf("ParseStreamWith recognition rejection panic=%v err=%v", got, returnedErr)
				}
				plain := setupProgram(t, ordinary, moduleOnly)
				got = budgetContextRecover(func() {
					returnedErr = plain.ParseStreamWith("main", strings.NewReader("x"), nil, ParseOptions{Backend: backend, Context: ctx})
				})
				if got != nil || returnedErr == nil {
					t.Fatalf("ParseStreamWith non-stream rejection panic=%v err=%v", got, returnedErr)
				}
				if p.preparedVM(Bytecode) != nil || p.preparedVM(BytecodeIterative) != nil || streamProgram.preparedVM(Bytecode) != nil || streamProgram.preparedVM(BytecodeIterative) != nil || plain.preparedVM(Bytecode) != nil || plain.preparedVM(BytecodeIterative) != nil {
					t.Fatal("fixed rejection prepared an unused VM")
				}
			})
		}
	}

	// Recognition is unavailable without the grammar; reject before constructing a recognizer or
	// consulting the caller's Context.
	p := setupProgram(t, ordinary, true)
	token := &budgetContextPanicToken{phase: "module-only recognition touched Context"}
	ctx := setupDonePanicContext(token)
	var err error
	got := budgetContextRecover(func() {
		_, err = p.ParseWith("main", "x", ParseOptions{Backend: Bytecode, Recognize: true, Context: ctx})
	})
	if got != nil || err == nil || p.rec != nil || p.preparedVM(Bytecode) != nil || p.preparedVM(BytecodeIterative) != nil {
		t.Fatalf("module-only recognition panic=%v err=%v rec=%p vm=%p ivm=%p", got, err, p.rec, p.preparedVM(Bytecode), p.preparedVM(BytecodeIterative))
	}
}

func setupProgramLabel(moduleOnly bool) string {
	if moduleOnly {
		return "moduleOnly"
	}
	return "source"
}

func TestBudgetSetupPreCancelledLeavesColdProgramsUnprepared(t *testing.T) {
	t.Parallel()
	const text = "#records\na=1\nbc=22\n"
	for _, moduleOnly := range []bool{false, true} {
		for _, backend := range []Backend{Bytecode, BytecodeIterative} {
			for _, recognize := range []bool{false, true} {
				if moduleOnly && recognize {
					continue // Covered by fixed recognition rejection above.
				}
				t.Run(setupProgramLabel(moduleOnly)+"/"+backend.String()+"/recognize="+map[bool]string{true: "yes", false: "no"}[recognize], func(t *testing.T) {
					p := setupProgram(t, `rule main = [a-z]* #atomic $$`, moduleOnly)
					assertSetupBackendState(t, p, backend, false)
					if !moduleOnly && p.Bytecode != nil {
						t.Fatal("pre-cancelled parse compiled its lazy Module")
					}
					ctx, cancel := context.WithCancel(context.Background())
					cancel()
					traceCalls := 0
					n, err := p.ParseWith("main", strings.Repeat("a", 8192), ParseOptions{Backend: backend, Recognize: recognize, Context: ctx, Limits: core.Limits{Compute: 1}, Trace: func(core.TraceEvent) { traceCalls++ }})
					if n != nil || !errors.Is(err, context.Canceled) || traceCalls != 0 {
						t.Fatalf("ParseWith node=%v err=%v traces=%d", n, err, traceCalls)
					}
					assertSetupBackendState(t, p, backend, false)
					if p.rec != nil {
						t.Fatal("pre-cancelled recognition prepared the recognizer")
					}

					docProg := setupProgram(t, `rule main = "a" $$`, moduleOnly)
					ctx, cancel = context.WithCancel(context.Background())
					cancel()
					d, err := docProg.NewDocumentWith("main", strings.Repeat("a", 8192), ParseOptions{Backend: backend, Context: ctx, Limits: core.Limits{Compute: 1}})
					if d != nil || !errors.Is(err, context.Canceled) {
						t.Fatalf("NewDocumentWith document=%v err=%v", d, err)
					}
					assertSetupBackendState(t, docProg, backend, false)
					if !moduleOnly && docProg.Bytecode != nil {
						t.Fatal("pre-cancelled Document construction compiled its lazy Module")
					}

					streamProg := setupProgram(t, codegentest.Records, moduleOnly)
					ctx, cancel = context.WithCancel(context.Background())
					cancel()
					reader := &setupCountingReader{r: strings.NewReader(text)}
					emits, traces := 0, 0
					err = streamProg.ParseStreamWith("main", reader, func(*core.Node) error { emits++; return nil }, ParseOptions{Backend: backend, Context: ctx, Limits: core.Limits{Compute: 1}, Trace: func(core.TraceEvent) { traces++ }})
					if !errors.Is(err, context.Canceled) || reader.reads != 0 || emits != 0 || traces != 0 {
						t.Fatalf("ParseStreamWith err=%v reads=%d emits=%d traces=%d", err, reader.reads, emits, traces)
					}
					assertSetupBackendState(t, streamProg, backend, false)
					if !moduleOnly && streamProg.Bytecode != nil {
						t.Fatal("pre-cancelled stream setup compiled its lazy Module")
					}
				})
			}
		}
	}
}

func TestBudgetSetupPostBackendChecksStopBeforePublication(t *testing.T) {
	t.Parallel()
	for _, moduleOnly := range []bool{false, true} {
		for _, backend := range []Backend{Bytecode, BytecodeIterative} {
			t.Run(setupProgramLabel(moduleOnly)+"/"+backend.String(), func(t *testing.T) {
				prog := setupProgram(t, `rule main = [a-z]* #atomic $$`, moduleOnly)
				gate := newSetupPhaseContext(func() bool { return setupBackendReady(prog, backend) })
				traceCalls := 0
				n, err := prog.ParseWith("main", strings.Repeat("a", 8192), ParseOptions{Backend: backend, Context: gate, Limits: core.Limits{Compute: 1}, Trace: func(core.TraceEvent) { traceCalls++ }})
				if n != nil || !errors.Is(err, context.Canceled) || !gate.triggered || traceCalls != 0 {
					t.Fatalf("ParseWith node=%v err=%v gate=%t traces=%d", n, err, gate.triggered, traceCalls)
				}
				assertSetupBackendState(t, prog, backend, true)
				if p, err := prog.ParseWith("main", "abc", ParseOptions{Backend: backend}); p == nil || err != nil {
					t.Fatalf("ParseWith retry node=%v err=%v", p, err)
				}

				docProg := setupProgram(t, `rule main = "a" $$`, moduleOnly)
				docGate := newSetupPhaseContext(func() bool { return setupBackendReady(docProg, backend) })
				doc, err := docProg.NewDocumentWith("main", strings.Repeat("a", 8192), ParseOptions{Backend: backend, Context: docGate, Limits: core.Limits{Compute: 1}})
				if doc != nil || !errors.Is(err, context.Canceled) || !docGate.triggered {
					t.Fatalf("NewDocumentWith document=%v err=%v gate=%t", doc, err, docGate.triggered)
				}
				assertSetupBackendState(t, docProg, backend, true)
				if d, err := docProg.NewDocumentWith("main", "a", ParseOptions{Backend: backend}); d == nil || err != nil {
					t.Fatalf("NewDocumentWith retry document=%v err=%v", d, err)
				}

				streamProg := setupProgram(t, codegentest.Records, moduleOnly)
				streamGate := newSetupPhaseContext(func() bool { return setupBackendReady(streamProg, backend) })
				reader := &setupCountingReader{r: strings.NewReader("#records\na=1\n")}
				emits, traces := 0, 0
				err = streamProg.ParseStreamWith("main", reader, func(*core.Node) error { emits++; return nil }, ParseOptions{Backend: backend, Context: streamGate, Limits: core.Limits{Compute: 1}, Trace: func(core.TraceEvent) { traces++ }})
				if !errors.Is(err, context.Canceled) || !streamGate.triggered || reader.reads != 0 || emits != 0 || traces != 0 {
					t.Fatalf("ParseStreamWith err=%v gate=%t reads=%d emits=%d traces=%d", err, streamGate.triggered, reader.reads, emits, traces)
				}
				assertSetupBackendState(t, streamProg, backend, true)
				reader = &setupCountingReader{r: strings.NewReader("#records\nxy=3\n")}
				emits = 0
				if err := streamProg.ParseStreamWith("main", reader, func(*core.Node) error { emits++; return nil }, ParseOptions{Backend: backend}); err != nil || emits != 1 {
					t.Fatalf("ParseStreamWith retry err=%v emits=%d", err, emits)
				}
			})
		}
	}
}

func TestBudgetSetupChecksAfterModuleBeforeVM(t *testing.T) {
	t.Parallel()
	prog := setupProgram(t, `rule main = "a" $$`, false)
	if prog.Bytecode != nil || prog.preparedVM(Bytecode) != nil {
		t.Fatalf("program was not cold: module=%p vm=%p", prog.Bytecode, prog.preparedVM(Bytecode))
	}
	gate := newSetupPhaseContext(func() bool { return prog.Bytecode != nil && prog.preparedVM(Bytecode) == nil })
	traceCalls := 0
	n, err := prog.ParseWith("main", "a", ParseOptions{Backend: Bytecode, Context: gate, Trace: func(core.TraceEvent) { traceCalls++ }})
	if n != nil || !errors.Is(err, context.Canceled) || !gate.triggered || traceCalls != 0 {
		t.Fatalf("ParseWith node=%v err=%v gate=%t traces=%d", n, err, gate.triggered, traceCalls)
	}
	if prog.Bytecode == nil || prog.preparedVM(Bytecode) != nil || prog.preparedVM(BytecodeIterative) != nil {
		t.Fatalf("post-module boundary state module=%p vm=%p ivm=%p", prog.Bytecode, prog.preparedVM(Bytecode), prog.preparedVM(BytecodeIterative))
	}
	if n, err := prog.ParseWith("main", "a", ParseOptions{Backend: Bytecode}); n == nil || err != nil {
		t.Fatalf("post-module retry node=%v err=%v", n, err)
	}
}

func TestBudgetSetupRecognizerPhaseChecks(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		backend Backend
		ready   func(*Program) bool
	}{
		{"recognizer", Closure, func(p *Program) bool { return p.rec != nil }},
		{"recognizer-bytecode", Bytecode, func(p *Program) bool { return p.rec != nil && p.rec.preparedVM(Bytecode) != nil }},
		{"recognizer-iterative", BytecodeIterative, func(p *Program) bool { return p.rec != nil && p.rec.preparedVM(BytecodeIterative) != nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prog := setupProgram(t, `rule main = "a" $$`, false)
			gate := newSetupPhaseContext(func() bool { return tc.ready(prog) })
			traceCalls := 0
			n, err := prog.ParseWith("main", strings.Repeat("a", 8192), ParseOptions{Backend: tc.backend, Recognize: true, Context: gate, Limits: core.Limits{Compute: 1}, Trace: func(core.TraceEvent) { traceCalls++ }})
			if n != nil || !errors.Is(err, context.Canceled) || !gate.triggered || traceCalls != 0 {
				t.Fatalf("recognition node=%v err=%v gate=%t traces=%d", n, err, gate.triggered, traceCalls)
			}
			if prog.rec == nil || traceCalls != 0 || prog.preparedVM(Bytecode) != nil || prog.preparedVM(BytecodeIterative) != nil {
				t.Fatalf("recognizer state rec=%p original vm=%p ivm=%p", prog.rec, prog.preparedVM(Bytecode), prog.preparedVM(BytecodeIterative))
			}
			if tc.backend == Closure && (prog.rec.preparedVM(Bytecode) != nil || prog.rec.preparedVM(BytecodeIterative) != nil) {
				t.Fatalf("Closure recognition prepared a VM: vm=%p ivm=%p", prog.rec.preparedVM(Bytecode), prog.rec.preparedVM(BytecodeIterative))
			}
			if tc.backend == Bytecode && (prog.rec.preparedVM(Bytecode) == nil || prog.rec.preparedVM(BytecodeIterative) != nil) || tc.backend == BytecodeIterative && (prog.rec.preparedVM(BytecodeIterative) == nil || prog.rec.preparedVM(Bytecode) != nil) {
				t.Fatalf("recognizer backend state vm=%p ivm=%p", prog.rec.preparedVM(Bytecode), prog.rec.preparedVM(BytecodeIterative))
			}
			if n, err := prog.ParseWith("main", "a", ParseOptions{Backend: tc.backend, Recognize: true}); err != nil || n != nil {
				t.Fatalf("recognition retry node=%v err=%v", n, err)
			}
		})
	}
}

// Input preparation can fail before a parser is published to tracing. Exercise
// its spent budget directly; API fixtures above verify the outer panic boundary.
func TestBudgetSetupInputPreparationPanicIdentity(t *testing.T) {
	t.Parallel()
	for _, unit := range []core.PositionUnit{core.Bytes, core.CodePoints} {
		t.Run(unit.String(), func(t *testing.T) {
			token := &budgetContextPanicToken{phase: "input materialization"}
			ctx := newBudgetContextErrPanic(token, 2)
			c := state.NewBudgetControl(ctx, core.Limits{})
			ctx.control, ctx.armed = c, true
			completed := false
			got := budgetContextRecover(func() {
				_ = state.NewControlledTextInput(strings.Repeat("a", 8192), unit, true, nil, c)
				completed = true
			})
			attributed, ok := got.(state.ContextPanic)
			if !ok || attributed.Value != token || completed {
				t.Fatalf("panic=%#v completed=%t", got, completed)
			}
			if unit == core.CodePoints && c.Used == 0 {
				t.Fatal("CodePoints fixture did not enter preparation loop")
			}
		})
	}
}

type setupPhasePanicContext struct {
	context.Context
	ready func() bool
	token *budgetContextPanicToken
}

func (c *setupPhasePanicContext) Err() error {
	if c.ready() {
		panic(c.token)
	}
	return c.Context.Err()
}

func TestBudgetSetupPostBackendPanicIdentity(t *testing.T) {
	t.Parallel()
	for _, moduleOnly := range []bool{false, true} {
		for _, backend := range []Backend{Bytecode, BytecodeIterative} {
			for _, api := range []string{"batch", "document", "stream"} {
				t.Run(setupProgramLabel(moduleOnly)+"/"+backend.String()+"/"+api, func(t *testing.T) {
					prog := setupProgram(t, codegentest.Records, moduleOnly)
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					token := &budgetContextPanicToken{phase: "post-backend setup"}
					gate := &setupPhasePanicContext{Context: ctx, ready: func() bool { return setupBackendReady(prog, backend) }, token: token}
					callbacks := 0
					reader := &setupCountingReader{r: strings.NewReader("#records\na=1\n")}
					o := ParseOptions{Backend: backend, Context: gate, Trace: func(core.TraceEvent) { callbacks++ }}
					var n *core.Node
					var d *Document
					var err error
					got := budgetContextRecover(func() {
						switch api {
						case "batch":
							n, err = prog.ParseWith("main", "#records\na=1\n", o)
						case "document":
							d, err = prog.NewDocumentWith("main", "#records\na=1\n", o)
						case "stream":
							err = prog.ParseStreamWith("main", reader, func(*core.Node) error { callbacks++; return nil }, o)
						}
					})
					if got != token || n != nil || d != nil || err != nil || callbacks != 0 || reader.reads != 0 {
						t.Fatalf("panic=%#v n=%v d=%v err=%v callbacks=%d reads=%d", got, n, d, err, callbacks, reader.reads)
					}
					assertSetupBackendState(t, prog, backend, true)
					if n, err := prog.ParseWith("main", "#records\na=1\n", ParseOptions{Backend: backend}); n == nil || err != nil {
						t.Fatalf("clean retry n=%v err=%v", n, err)
					}
				})
			}
		}
	}
}

func TestBudgetSetupConcurrentColdCalls(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, `rule main = "abc" #atomic $$`)
	results := make(chan error, 24)
	start := make(chan struct{})
	for i := 0; i < cap(results); i++ {
		go func(i int) {
			<-start
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			aborted := i%4 == 0
			if aborted {
				cancel()
			}
			o := ParseOptions{Backend: []Backend{Closure, Bytecode, BytecodeIterative}[i%3], Unit: []core.PositionUnit{core.Bytes, core.CodePoints}[i%2], Recognize: i%5 == 0, Context: ctx}
			n, err := prog.ParseWith("main", "abc", o)
			if aborted {
				if n != nil || !errors.Is(err, context.Canceled) {
					results <- errors.New("cancelled concurrent call did not return cancellation")
					return
				}
			} else if err != nil || !o.Recognize && n == nil || o.Recognize && n != nil {
				results <- errors.New("independent concurrent call failed")
				return
			}
			results <- nil
		}(i)
	}
	close(start)
	for i := 0; i < cap(results); i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
}
