package engine

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/diag"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/engine/closure"
	"zeroparse.dev/zp/internal/gentest"
	ir "zeroparse.dev/zp/internal/ir/core"
	corebuild "zeroparse.dev/zp/internal/ir/core/build"
)

// Cancel at a deterministic poll, without timing or global compiler hooks.
type compilePollContext struct {
	context.Context
	cancel context.CancelFunc
	left   int
}

func (c *compilePollContext) Err() error {
	c.left--
	if c.left == 0 {
		c.cancel()
	}
	return c.Context.Err()
}

func compileTestContext(t *testing.T, polls int) *compilePollContext {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &compilePollContext{Context: ctx, cancel: cancel, left: polls}
}

func compileTestGrammar(t *testing.T) *grammar.Grammar {
	t.Helper()
	g, err := parseTest(gentest.InferredChain(1024, false, ""))
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestCompileContextCancellation(t *testing.T) {
	t.Parallel()
	g := compileTestGrammar(t)
	before, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	for _, polls := range []int{1, 3, 12} {
		ctx := compileTestContext(t, polls)
		prog, err := CompileContext(ctx, g, compile.Options{})
		if prog != nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("polls=%d: program=%v error=%v", polls, prog, err)
		}
	}
	after, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("cancelled compilation mutated source grammar")
	}
}

func TestCompileContextPhaseCancellation(t *testing.T) {
	t.Parallel()
	g := compileTestGrammar(t)
	p, err := Compile(g, compile.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"static-analysis", "success-proof", "failure-proof", "cut-proof", "dependency-graph", "type-checking", "type-inference", "list-constructor-type", "core-build", "core-analysis", "core-passes", "core-pass-terminals", "core-pass-guards", "core-pass-scans", "core-pass-class-runs", "core-verify", "closure-core"} {
		t.Run(phase, func(t *testing.T) {
			ctx := compileTestContext(t, 1)
			control := &cancel.Control{Ctx: ctx}
			defer func() {
				failure := recover()
				cancelled, ok := failure.(cancel.Cancellation)
				if !ok || !errors.Is(cancelled.Err, context.Canceled) {
					t.Fatalf("unexpected unwind: %v", failure)
				}
			}()
			switch phase {
			case "static-analysis":
				compile.AnalyzeControlled(g.Rules(), control)
			case "success-proof", "failure-proof", "cut-proof":
				items := make([]grammar.Expr, 1024)
				for i := range items {
					items[i] = &grammar.Top{}
					if phase == "failure-proof" {
						items[i] = &grammar.Bottom{}
					}
				}
				var expr grammar.Expr = &grammar.Seq{Items: items}
				if phase == "failure-proof" {
					expr = &grammar.Not{Expr: &grammar.Choice{Alts: items}}
				} else if phase == "cut-proof" {
					expr = &grammar.Optional{Expr: expr}
				}
				// Only the shared proof traversal can reach the first context
				// checkpoint; no outer analysis loop contributes polling here.
				a := &compile.Analysis{Control: control}
				a.AlwaysSucceeds(expr)
			case "dependency-graph":
				edges := map[string][]string{}
				for _, rd := range g.Rules() {
					edges[rd.Name] = compile.AllCalls(rd.Expr, nil)
				}
				compile.Tarjan(g.Rules(), edges, control)
			case "type-checking":
				var diagnostics diag.ErrorList
				compile.CheckTypesControlled(p.Static(), &diagnostics, control)
			case "type-inference":
				compile.ValueTypes(p.Static(), control)
			case "list-constructor-type":
				// The entry poll alone cannot cancel: cancellation must be
				// observed inside the temporary checker resolving Elem.
				var elem grammar.TypeExpr = &grammar.TypeRef{Name: "Match"}
				for range 1024 {
					elem = &grammar.ListType{Elem: elem}
				}
				c := &compile.Compiler{Prog: p.Static(), Control: control}
				c.CheckTerm(&grammar.ListConstructor{Elem: elem}, compile.NewScope(), nil)
			case "core-build":
				_, _ = corebuild.CoreControlled(p.Static(), control)
			case "core-analysis", "core-passes", "core-pass-terminals", "core-pass-guards", "core-pass-scans", "core-pass-class-runs", "core-verify":
				m, err := corebuild.CoreFrom(p.Static(), nil)
				if err != nil {
					t.Fatal(err)
				}
				// A pipeline run cancels in its first analysis, so each pass and Verify run on an
				// analyzed module to reach the checkpoint through their own polls.
				if phase != "core-analysis" && phase != "core-passes" {
					ir.Analyze(m)
				}
				m.Control = control
				switch phase {
				case "core-analysis":
					ir.Analyze(m)
				case "core-passes":
					_, _ = ir.Run(m, &ir.Env{}, ir.RunOptions{})
				case "core-pass-terminals":
					ir.Terminals(m, func(*ir.Rule) bool { return true })
				case "core-pass-guards":
					ir.Guards(m, true)
				case "core-pass-scans":
					ir.Scans(m)
				case "core-pass-class-runs":
					ir.ClassRuns(m)
				case "core-verify":
					_ = ir.Verify(m)
				}
			case "closure-core":
				_, _ = closure.Core(p.Static(), control)
			}
			t.Fatal("phase completed without cancellation")
		})
	}
}

func TestCompileContextOrdinaryParity(t *testing.T) {
	t.Parallel()
	for _, src := range []string{
		`type P = {}; rule main = "a" => P{}`,
		`rule main = #operators { operand [0-9]; level sum { infix left "+" } }`,
		`rule main = "" / "a"`,
		`rule main = ("a" #cut "b") / ""`,
		`rule main = "a" / missing`,
		`type P = { X node[string] }; rule main = "a" => P{X: 1}`,
	} {
		g, err := parseTest(src)
		if err != nil {
			t.Fatal(err)
		}
		ordinary, ordinaryErr := Compile(g, compile.Options{})
		ctx, cancel := context.WithCancel(context.Background())
		controlled, controlledErr := CompileContext(ctx, g, compile.Options{})
		cancel()
		if (ordinaryErr == nil) != (controlledErr == nil) {
			t.Fatalf("error mismatch: %v / %v", ordinaryErr, controlledErr)
		}
		if ordinaryErr != nil {
			if ordinaryErr.Error() != controlledErr.Error() {
				t.Fatalf("diagnostic mismatch: %v / %v", ordinaryErr, controlledErr)
			}
			continue
		}
		for _, rd := range g.Rules() {
			x, ok := ordinary.RuleType(rd.Name)
			y, controlledOK := controlled.RuleType(rd.Name)
			if x != y || ok != controlledOK {
				t.Fatalf("type mismatch: %s / %s", x, y)
			}
		}
		// Cancelling the compile context after success must not cancel execution.
		x, ex := ordinary.Parse("main", "a")
		y, ey := controlled.Parse("main", "a")
		if (ex == nil) != (ey == nil) {
			t.Fatalf("parse error mismatch: %v / %v", ex, ey)
		}
		xj, _ := json.Marshal(x)
		yj, _ := json.Marshal(y)
		if !reflect.DeepEqual(xj, yj) {
			t.Fatal("parse value mismatch")
		}
	}
}
