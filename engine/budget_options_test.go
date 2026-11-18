//go:build !zp_budget_controls_reference

package engine_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"zeroparse.dev/zp/engine"
	iengine "zeroparse.dev/zp/internal/runtime/engine"
	"zeroparse.dev/zp/pkg/runtime"
	"zeroparse.dev/zp/pkg/runtime/value"
)

func TestBudgetOptionsOrderAndTraceComposition(t *testing.T) {
	p, err := engine.CompileSource(`@start
rule main = "abc" $$`, "main")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	n, err := p.Parse("abc", engine.WithContext(ctx), engine.WithContext(nil), engine.WithLimits(runtime.Limits{Compute: 1}), engine.WithLimits(runtime.Limits{Compute: runtime.Unlimited}))
	if n == nil || err != nil {
		t.Fatalf("last option did not win: node=%v err=%v", n, err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	first, second := 0, 0
	n, err = p.Parse("abc", engine.WithContext(ctx), engine.WithTrace(func(engine.TraceEvent) { first++; cancel() }), engine.WithTrace(func(engine.TraceEvent) { second++ }))
	if n != nil || !errors.Is(err, context.Canceled) || first != 1 || second != 0 {
		t.Fatalf("callback order: node=%v err=%v first=%d second=%d", n, err, first, second)
	}
}

func TestBudgetStreamPublicOptions(t *testing.T) {
	p, err := engine.CompileSource(`@start
rule main = "a"* #stream $$`, "main")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	options, emits := 0, 0
	err = p.ParseStream(strings.NewReader("aaa"), func(*value.Node) error { emits++; return nil },
		engine.WithContext(ctx), engine.WithContext(nil), engine.WithLimits(runtime.Limits{Compute: 1}), engine.WithLimits(runtime.Limits{Compute: runtime.Unlimited}),
		func(o *iengine.ParseOptions) { options++ })
	if err != nil || emits != 3 || options != 1 {
		t.Fatalf("error=%v emits=%d options=%d", err, emits, options)
	}
	first, second := 0, 0
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	err = p.ParseStream(strings.NewReader("aaa"), func(*value.Node) error { t.Fatal("emitted after trace cancellation"); return nil }, engine.WithContext(ctx), engine.WithTrace(func(engine.TraceEvent) { first++; cancel() }), engine.WithTrace(func(engine.TraceEvent) { second++ }))
	if err != context.Canceled || first != 1 || second != 0 {
		t.Fatalf("error=%v trace=%d/%d", err, first, second)
	}
}
