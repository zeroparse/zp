//go:build !zp_budget_controls_reference

package engine_test

import (
	"context"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/engine"
)

type publicContextPanicToken struct{ label string }

func (*publicContextPanicToken) Error() string { return "public context panic token" }
func (*publicContextPanicToken) RuntimeError() {}

var _ runtime.Error = (*publicContextPanicToken)(nil)

type publicContextErrPanic struct {
	context.Context
	done  chan struct{}
	armed bool
	calls int
	token *publicContextPanicToken
}

func (c *publicContextErrPanic) Done() <-chan struct{} { return c.done }

func (c *publicContextErrPanic) Err() error {
	if !c.armed {
		return nil
	}
	c.calls++
	if c.calls == 2 { // The first call is the post-TraceEnter check; the second is spent budget.
		panic(c.token)
	}
	return nil
}

func TestContextErrRuntimePanicIdentityThroughPublicOptions(t *testing.T) {
	literal := strings.Repeat("a", 1024)
	p, err := compileTestSource(`rule main = "`+literal+`" $$`, "main")
	if err != nil {
		t.Fatal(err)
	}

	for _, backend := range []engine.Backend{engine.Closure, engine.Bytecode, engine.BytecodeIterative} {
		for _, unit := range []core.PositionUnit{core.Bytes, core.CodePoints} {
			for _, recognize := range []bool{false, true} {
				t.Run(backend.String()+"/"+unit.String()+"/recognize="+strconv.FormatBool(recognize), func(t *testing.T) {
					retained, err := p.Parse(literal, engine.WithBackend(backend), engine.WithUnit(unit))
					if retained == nil || err != nil {
						t.Fatalf("baseline node=%v error=%v", retained, err)
					}
					snapshot := retained.String()
					token := &publicContextPanicToken{label: "public context panic"}
					ctx := &publicContextErrPanic{Context: context.Background(), done: make(chan struct{}), token: token}
					traceEntries := 0
					opts := []engine.ParseOption{
						engine.WithBackend(backend),
						engine.WithUnit(unit),
						engine.WithContext(ctx),
						engine.WithTrace(func(e engine.TraceEvent) {
							if e.Kind == engine.TraceEnter && e.Rule == "main" {
								traceEntries++
								ctx.armed = true
							}
						}),
					}
					if recognize {
						opts = append(opts, engine.RecognizeOnly())
					}

					var node *core.Node
					var returnedErr error
					var got any
					func() {
						defer func() { got = recover() }()
						node, returnedErr = p.Parse(literal, opts...)
					}()
					if got != token || node != nil || returnedErr != nil || ctx.calls != 2 || traceEntries != 1 {
						t.Fatalf("panic=%#v node=%v error=%v Err calls=%d trace entries=%d", got, node, returnedErr, ctx.calls, traceEntries)
					}

					retryOpts := []engine.ParseOption{engine.WithBackend(backend), engine.WithUnit(unit)}
					if recognize {
						retryOpts = append(retryOpts, engine.RecognizeOnly())
					}
					retry, retryErr := p.Parse(literal, retryOpts...)
					if retained.String() != snapshot {
						t.Fatal("panic/retry mutated retained Node")
					}
					if retryErr != nil || recognize && retry != nil || !recognize && (retry == nil || retry.String() != snapshot) {
						t.Fatalf("retry node=%v err=%v recognize=%v", retry, retryErr, recognize)
					}
				})
			}
		}
	}
}
