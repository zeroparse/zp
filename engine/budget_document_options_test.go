//go:build !zp_budget_controls_reference

package engine_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"zeroparse.dev/zp/engine"
	iengine "zeroparse.dev/zp/internal/runtime/engine"
	"zeroparse.dev/zp/pkg/runtime"
)

func TestBudgetDocumentPublicDefaultsAndOptionsOnce(t *testing.T) {
	p, err := engine.CompileSource(`@start
rule main = inner $$

rule inner = "(" inner ")" / "a"`, "main")
	if err != nil {
		t.Fatal(err)
	}
	for _, backend := range []engine.Backend{engine.Closure, engine.Bytecode, engine.BytecodeIterative} {
		t.Run(backend.String(), func(t *testing.T) {
			constructCalls, parseCalls := 0, 0
			var order []string
			d, err := p.NewDocument("((a))", engine.WithBackend(backend), engine.WithLimits(runtime.Limits{Depth: 1}),
				func(o *iengine.ParseOptions) { constructCalls++ },
				engine.WithTrace(func(e engine.TraceEvent) {
					if e.Rule == "main" && e.Kind == engine.TraceEnter {
						order = append(order, "default")
					}
				}))
			if err != nil || constructCalls != 1 {
				t.Fatalf("construction=%v options=%d", err, constructCalls)
			}
			if n, err := d.Parse(); n != nil || err == nil {
				t.Fatalf("default depth: node=%v error=%v", n, err)
			}
			order = nil
			n, err := d.Parse(engine.WithLimits(runtime.Limits{Depth: 20}), func(o *iengine.ParseOptions) { parseCalls++ },
				engine.WithTrace(func(e engine.TraceEvent) {
					if e.Rule == "main" && e.Kind == engine.TraceEnter {
						order = append(order, "invocation")
					}
				}))
			if n == nil || err != nil || parseCalls != 1 || constructCalls != 1 {
				t.Fatalf("override: node=%v error=%v counts=%d/%d", n, err, constructCalls, parseCalls)
			}
			if !reflect.DeepEqual(order, []string{"default", "invocation"}) {
				t.Fatalf("trace composed twice or out of order: %v", order)
			}
			// Override depth must not replace defaults; clear successful memo so
			// the next invocation genuinely evaluates the nested rule again.
			if err := d.Edit(0, 5, "((a))"); err != nil {
				t.Fatal(err)
			}
			if n, err := d.Parse(); n != nil || err == nil {
				t.Fatalf("default changed: node=%v error=%v", n, err)
			}
		})
	}
}

func TestBudgetDocumentPublicContextOverrideAndTraceOrder(t *testing.T) {
	p, err := engine.CompileSource(`@start
rule main = "a" $$`, "main")
	if err != nil {
		t.Fatal(err)
	}
	for _, backend := range []engine.Backend{engine.Closure, engine.Bytecode, engine.BytecodeIterative} {
		ctx, cancel := context.WithCancel(context.Background())
		d, err := p.NewDocument("a", engine.WithBackend(backend), engine.WithContext(ctx), engine.WithLimits(runtime.Limits{Compute: 10000}))
		if err != nil {
			t.Fatal(err)
		}
		cancel()
		if n, err := d.Parse(); n != nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("%s defaults: node=%v error=%v", backend, n, err)
		}
		if n, err := d.Parse(engine.WithContext(nil), engine.WithLimits(runtime.Limits{})); n == nil || err != nil {
			t.Fatalf("%s override: node=%v error=%v", backend, n, err)
		}
		if n, err := d.Parse(); n != nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("%s defaults mutated: node=%v error=%v", backend, n, err)
		}
		ctx, cancel = context.WithCancel(context.Background())
		first, second := 0, 0
		_, err = d.Parse(engine.WithContext(ctx), engine.WithTrace(func(engine.TraceEvent) { first++; cancel() }), engine.WithTrace(func(engine.TraceEvent) { second++ }))
		cancel()
		if !errors.Is(err, context.Canceled) || first != 1 || second != 0 {
			t.Fatalf("%s callbacks: error=%v first=%d second=%d", backend, err, first, second)
		}
	}
}

// WithLimits merges field by field: a later option overrides the nonzero fields of an earlier one,
// for the options of one Parse and for the call options over the construction defaults of a
// Document.
func TestLimitsMergeAcrossOptions(t *testing.T) {
	p, err := engine.CompileSource(`@start
rule main = inner $$

rule inner = "(" inner ")" / "a"`, "main")
	if err != nil {
		t.Fatal(err)
	}
	kind := func(err error) runtime.LimitKind {
		t.Helper()
		var le *runtime.LimitError
		if !errors.As(err, &le) {
			t.Fatalf("not a LimitError: %v", err)
		}
		return le.Kind
	}
	for _, backend := range []engine.Backend{engine.Closure, engine.Bytecode, engine.BytecodeIterative} {
		t.Run(backend.String(), func(t *testing.T) {
			b := engine.WithBackend(backend)
			if _, err := p.Parse("((a))", b, engine.WithLimits(runtime.Limits{Input: 3}), engine.WithLimits(runtime.Limits{Depth: 20})); kind(err) != runtime.LimitInput {
				t.Fatalf("a later Depth dropped the Input: %v", err)
			}
			if _, err := p.Parse("((a))", b, engine.WithLimits(runtime.Limits{Depth: 1}), engine.WithLimits(runtime.Limits{Compute: runtime.Unlimited})); kind(err) != runtime.LimitDepth {
				t.Fatalf("a later Compute dropped the Depth: %v", err)
			}
			if n, err := p.Parse("((a))", b, engine.WithLimits(runtime.Limits{Depth: 1}), engine.WithLimits(runtime.Limits{Depth: 20})); n == nil || err != nil {
				t.Fatalf("a later Depth did not override: %v", err)
			}
			doc := func(l runtime.Limits) *engine.Document {
				t.Helper()
				d, err := p.NewDocument("((a))", b, engine.WithLimits(l))
				if err != nil {
					t.Fatal(err)
				}
				if err := d.Edit(0, 5, "(((a)))"); err != nil {
					t.Fatal(err)
				}
				return d
			}
			if _, err := doc(runtime.Limits{Input: 5}).Parse(engine.WithLimits(runtime.Limits{Depth: 20})); kind(err) != runtime.LimitInput {
				t.Fatalf("a call Depth dropped the default Input: %v", err)
			}
			d := doc(runtime.Limits{Compute: 20, Input: 5}) // enough to construct, too little to parse
			if _, err := d.Parse(engine.WithLimits(runtime.Limits{Input: runtime.Unlimited, Depth: 20})); kind(err) != runtime.LimitCompute {
				t.Fatalf("a call Input dropped the default Compute: %v", err)
			}
			if n, err := d.Parse(engine.WithLimits(runtime.Limits{Compute: runtime.Unlimited, Input: runtime.Unlimited})); n == nil || err != nil {
				t.Fatalf("call limits did not override the defaults: %v", err)
			}
			if _, err := d.Parse(engine.WithLimits(runtime.Limits{Compute: runtime.Unlimited})); kind(err) != runtime.LimitInput {
				t.Fatalf("a call changed the default Input: %v", err)
			}
		})
	}
}
