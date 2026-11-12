package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"testing"
	"weak"
	"zeroparse.dev/zp/internal/cancel"
	ir "zeroparse.dev/zp/internal/ir/core"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/engine/closure"
	"zeroparse.dev/zp/internal/engine/core"
	"zeroparse.dev/zp/internal/starttest"
	"zeroparse.dev/zp/runtime/parse"
)

func contextClassRanges() []grammar.CharRange {
	rs := make([]grammar.CharRange, 4096)
	for i := range rs {
		rs[i] = grammar.CharRange{Lo: rune(256 + i*2), Hi: rune(256 + i*2)}
	}
	return rs
}

func TestClassConstructionContextCancellation(t *testing.T) {
	t.Parallel()
	cl := &grammar.CharClass{Ranges: contextClassRanges()}
	before := append([]grammar.CharRange(nil), cl.Ranges...)
	var deep grammar.Expr = cl
	for range 1024 {
		deep = &grammar.Atomic{Expr: deep}
	}
	for _, phase := range []struct {
		name string
		run  func(*cancel.Control)
	}{
		{"descriptor", func(c *cancel.Control) { compile.CharClassStringControlled(cl, c) }},
		{"copy", func(c *cancel.Control) { compile.CopyClassRanges(cl.Ranges, c) }},
		{"bitmap", func(c *cancel.Control) { compile.ClassASCII(cl.Ranges, false, c) }},
		{"accept-construction", func(c *cancel.Control) { closure.ClassAcceptControlled(cl, c) }},
		{"matcher", func(c *cancel.Control) {
			_, _ = closure.Matcher(&compile.Program{DescTable: []string{"c"}}, &ir.Module{Exps: []string{"c"}}, &ir.ClassNode{Class: coreClass(cl)}, nil, c, false)
		}},
		{"choice-guard", func(c *cancel.Control) {
			ic := coreClass(cl)
			alt := &ir.Alt{Body: &ir.ClassNode{Class: ic}, Guard: &ir.First{Class: ic}}
			_, _ = closure.Matcher(&compile.Program{DescTable: []string{"c"}}, &ir.Module{Exps: []string{"c"}}, &ir.Choice{Alts: []*ir.Alt{alt}}, nil, c, false)
		}},
		{"scan-repeat", func(c *cancel.Control) {
			_, _ = closure.Matcher(&compile.Program{DescTable: []string{"c"}}, &ir.Module{Exps: []string{"c"}}, &ir.Scan{Class: coreClass(cl), Max: -1}, nil, c, false)
		}},
		{"guard-extraction", func(c *cancel.Control) {
			compile.FirstTerminalControlled(&compile.Program{}, deep, 0, map[string]bool{}, c)
		}},
	} {
		t.Run(phase.name, func(t *testing.T) {
			c := &cancel.Control{Ctx: compileTestContext(t, 1)}
			defer func() {
				failure := recover()
				stop, ok := failure.(cancel.Cancellation)
				if !ok || !errors.Is(stop.Err, context.Canceled) {
					t.Fatalf("class phase did not cancel internally: %v", failure)
				}
				if c.Steps != cancel.PollStride {
					t.Fatalf("poll outside class traversal: %d", c.Steps)
				}
				if !reflect.DeepEqual(before, cl.Ranges) {
					t.Fatal("cancellation mutated ranges")
				}
			}()
			phase.run(c)
		})
	}
}

func TestClassConstructionContextFacts(t *testing.T) {
	t.Parallel()
	cases := [][]grammar.CharRange{nil, {{Lo: 'a', Hi: 'z'}}, {{Lo: 'a', Hi: 'z'}, {Lo: 'β', Hi: 'β'}}, {{Lo: 'a', Hi: 'f'}, {Lo: '0', Hi: '9'}, {Lo: 'β', Hi: 'β'}, {Lo: '😀', Hi: '😀'}}}
	for i, rs := range cases {
		for _, neg := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/%t", i, neg), func(t *testing.T) {
				cl := &grammar.CharClass{Ranges: rs, Negated: neg}
				c := &cancel.Control{Ctx: context.Background()}
				if got := compile.CharClassStringControlled(cl, c); got != compile.CharClassString(cl) {
					t.Fatalf("descriptor %s", got)
				}
				ordinary, controlled := closure.ClassAccept(cl), closure.ClassAcceptControlled(cl, c)
				runes := []rune{'β', '😀', 0x10ffff}
				for r := rune(0); r < 256; r++ {
					runes = append(runes, r)
				}
				for _, r := range runes {
					for _, active := range []bool{false, true} {
						p, q := &core.Parser{}, &core.Parser{}
						if active {
							p.Control = core.NewBudgetControl(nil, parse.Limits{Compute: 1 << 30})
							q.Control = core.NewBudgetControl(nil, parse.Limits{Compute: 1 << 30})
						}
						expected := false
						for _, rg := range rs {
							expected = expected || rg.Lo <= r && r <= rg.Hi
						}
						expected = expected != neg
						if ordinary(p, r) != expected || controlled(q, r) != expected {
							t.Fatalf("membership rune %U active=%t", r, active)
						}
						if active && p.Control.Used != q.Control.Used {
							t.Fatal("runtime budget charge changed")
						}
					}
				}
			})
		}
	}
	cl := &grammar.CharClass{Ranges: []grammar.CharRange{{Lo: '-', Hi: '-'}, {Lo: '[', Hi: '['}, {Lo: ']', Hi: ']'}, {Lo: '^', Hi: '^'}, {Lo: '\\', Hi: '\\'}}}
	if got := compile.CharClassStringControlled(cl, &cancel.Control{Ctx: context.Background()}); got != `[\-\[\]\^\\]` {
		t.Fatalf("escaped descriptor %q", got)
	}
	rs := contextClassRanges()
	cl = &grammar.CharClass{Ranges: rs}
	accept := closure.ClassAcceptControlled(cl, &cancel.Control{Ctx: context.Background()})
	rs[0] = grammar.CharRange{Lo: 'a', Hi: 'a'}
	if !accept(&core.Parser{}, 256) || accept(&core.Parser{}, 'a') {
		t.Fatal("matcher aliases source range slice")
	}
}

func TestClassConstructionCompileContextParity(t *testing.T) {
	t.Parallel()
	for _, shape := range []string{"single", "choice", "scan"} {
		cl := &grammar.CharClass{Ranges: []grammar.CharRange{{Lo: 'a', Hi: 'f'}, {Lo: '0', Hi: '9'}, {Lo: 'β', Hi: 'β'}}}
		var e grammar.Expr = cl
		if shape == "choice" {
			e = &grammar.Choice{Alts: []grammar.Expr{cl, &grammar.Literal{Value: "z"}}}
		}
		if shape == "scan" {
			e = &grammar.Discard{Expr: &grammar.Repeat{Expr: cl, Min: 1, Max: -1}}
		}
		g := &grammar.Grammar{File: grammar.File{Decls: []grammar.Decl{&grammar.RuleDef{Name: "main", Expr: e}}}}
		starttest.MarkAll(g)
		for _, compiledRecognition := range []bool{false, true} {
			a, err := Compile(g, compile.Options{Recognize: compiledRecognition})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			b, err := CompileContext(ctx, g, compile.Options{Recognize: compiledRecognition})
			cancel()
			if err != nil {
				t.Fatal(err)
			}
			for _, unit := range []parse.PositionUnit{parse.Bytes, parse.CodePoints} {
				for _, recognize := range []bool{false, true} {
					for _, active := range []bool{false, true} {
						for _, s := range []string{"a", "β", "z", "?", "", "aβ1"} {
							// Active budget controls and Recognize lazily compile separate graphs.
							// Their disabled cases exercise the controlled compiler's own matchers,
							// including its directly compiled recognition-only graph.
							opts := ParseOptions{Unit: unit, Recognize: recognize}
							if active {
								opts.Limits = parse.Limits{Compute: 1 << 30}
							}
							x, xe := a.ParseWith("main", s, opts)
							y, ye := b.ParseWith("main", s, opts)
							xj, xerr := json.Marshal(x)
							yj, yerr := json.Marshal(y)
							if xerr != nil || yerr != nil {
								t.Fatalf("node JSON: %v/%v", xerr, yerr)
							}
							if string(xj) != string(yj) || fmt.Sprint(xe) != fmt.Sprint(ye) {
								t.Fatalf("%s/compiledRecognition=%t/%v/recognize=%t/active=%t/%q mismatch: %s/%v vs %s/%v", shape, compiledRecognition, unit, recognize, active, s, xj, xe, yj, ye)
							}
						}
					}
				}
			}
		}
	}
}

type classCompilationContext struct {
	context.Context
	done chan struct{}
	pad  [128]byte
}

func (c *classCompilationContext) Done() <-chan struct{} { return c.done }

//go:noinline
func classProgramWithoutContext(t *testing.T) (*Program, weak.Pointer[classCompilationContext]) {
	ctx := &classCompilationContext{Context: context.Background(), done: make(chan struct{})}
	ref := weak.Make(ctx)
	cl := &grammar.CharClass{Ranges: []grammar.CharRange{{Lo: 'a', Hi: 'a'}, {Lo: 'b', Hi: 'b'}, {Lo: 'β', Hi: 'β'}}}
	g := &grammar.Grammar{File: grammar.File{Decls: []grammar.Decl{&grammar.RuleDef{Name: "main", Expr: &grammar.Choice{Alts: []grammar.Expr{&grammar.Seq{Items: []grammar.Expr{cl, &grammar.EndInput{}}}, &grammar.Discard{Expr: &grammar.Repeat{Expr: cl, Min: 1, Max: -1}}}}}}}}
	starttest.MarkAll(g)
	p, err := CompileContext(ctx, g, compile.Options{})
	if err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	if ref.Value() != ctx {
		t.Fatal("weak context positive control failed")
	}
	runtime.KeepAlive(ctx)
	return p, ref
}

func TestClassConstructionProgramReleasesCompileContext(t *testing.T) {
	t.Parallel()
	p, ref := classProgramWithoutContext(t)
	released := false
	for range 5 {
		runtime.GC()
		if ref.Value() == nil {
			released = true
			break
		}
	}
	if !released {
		t.Fatal("compiled class closures retain compile context")
	}
	for _, s := range []string{"a", "β", "bbβ"} {
		if _, err := p.Parse("main", s); err != nil {
			t.Fatal(err)
		}
	}
	runtime.KeepAlive(p)
}

// coreClass returns the Core IR class of the class expression cl, recording the expectation 0.
func coreClass(cl *grammar.CharClass) *ir.Class {
	c := &ir.Class{Negated: cl.Negated}
	for _, r := range cl.Ranges {
		c.Ranges = append(c.Ranges, ir.Range{Lo: r.Lo, Hi: r.Hi})
	}
	return c
}
