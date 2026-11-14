//go:build !zp_budget_controls_reference

package engine_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/engine"
	iengine "zeroparse.dev/zp/internal/engine"
)

func TestBudgetOptionsOrderAndTraceComposition(t *testing.T) {
	p, err := engine.CompileSource(`@start
rule main = "abc" $$`, "main")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	n, err := p.Parse("abc", engine.WithContext(ctx), engine.WithContext(nil), engine.WithLimits(core.Limits{Compute: 1}), engine.WithLimits(core.Limits{Compute: core.Unlimited}))
	if n == nil || err != nil {
		t.Fatalf("last option did not win: node=%v err=%v", n, err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	first, second := 0, 0
	n, err = p.Parse("abc", engine.WithContext(ctx), engine.WithTrace(func(core.TraceEvent) { first++; cancel() }), engine.WithTrace(func(core.TraceEvent) { second++ }))
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
	err = p.ParseStream(strings.NewReader("aaa"), func(*core.Node) error { emits++; return nil },
		engine.WithContext(ctx), engine.WithContext(nil), engine.WithLimits(core.Limits{Compute: 1}), engine.WithLimits(core.Limits{Compute: core.Unlimited}),
		func(o *iengine.ParseOptions) { options++ })
	if err != nil || emits != 3 || options != 1 {
		t.Fatalf("error=%v emits=%d options=%d", err, emits, options)
	}
	first, second := 0, 0
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	err = p.ParseStream(strings.NewReader("aaa"), func(*core.Node) error { t.Fatal("emitted after trace cancellation"); return nil }, engine.WithContext(ctx), engine.WithTrace(func(core.TraceEvent) { first++; cancel() }), engine.WithTrace(func(core.TraceEvent) { second++ }))
	if err != context.Canceled || first != 1 || second != 0 {
		t.Fatalf("error=%v trace=%d/%d", err, first, second)
	}
}
