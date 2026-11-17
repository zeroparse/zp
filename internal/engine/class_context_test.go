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

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/engine/closure"
	"zeroparse.dev/zp/internal/engine/state"
	"zeroparse.dev/zp/internal/testing/starttest"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

func contextClassRanges() []ast.CharRange {
	rs := make([]ast.CharRange, 4096)
	for i := range rs {
		rs[i] = ast.CharRange{Lo: rune(256 + i*2), Hi: rune(256 + i*2)}
	}
	return rs
}

func TestClassConstructionContextCancellation(t *testing.T) {
	t.Parallel()
	cl := &ast.CharClass{Ranges: contextClassRanges()}
	before := append([]ast.CharRange(nil), cl.Ranges...)
	var deep ast.Expr = cl
	for range 1024 {
		deep = &ast.Atomic{Expr: deep}
	}
	for _, phase := range []struct {
		name string
		run  func(*cancel.Control)
	}{
		{"descriptor", func(c *cancel.Control) { compile.CharClassStringControlled(cl, c) }},
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
	cases := [][]ast.CharRange{nil, {{Lo: 'a', Hi: 'z'}}, {{Lo: 'a', Hi: 'z'}, {Lo: 'β', Hi: 'β'}}, {{Lo: 'a', Hi: 'f'}, {Lo: '0', Hi: '9'}, {Lo: 'β', Hi: 'β'}, {Lo: '😀', Hi: '😀'}}}
	for i, rs := range cases {
		for _, neg := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/%t", i, neg), func(t *testing.T) {
				cl := &ast.CharClass{Ranges: rs, Negated: neg}
				c := &cancel.Control{Ctx: context.Background()}
				if got := compile.CharClassStringControlled(cl, c); got != compile.CharClassString(cl) {
					t.Fatalf("descriptor %s", got)
				}
				accept := closure.ClassAccept(cl)
				runes := []rune{'β', '😀', 0x10ffff}
				for r := rune(0); r < 256; r++ {
					runes = append(runes, r)
				}
				for _, r := range runes {
					for _, active := range []bool{false, true} {
						p := &state.Parser{}
						if active {
							p.Control = state.NewBudgetControl(nil, core.Limits{Compute: 1 << 30})
						}
						expected := false
						for _, rg := range rs {
							expected = expected || rg.Lo <= r && r <= rg.Hi
						}
						expected = expected != neg
						if accept(p, r) != expected {
							t.Fatalf("membership rune %U active=%t", r, active)
						}
					}
				}
			})
		}
	}
	cl := &ast.CharClass{Ranges: []ast.CharRange{{Lo: '-', Hi: '-'}, {Lo: '[', Hi: '['}, {Lo: ']', Hi: ']'}, {Lo: '^', Hi: '^'}, {Lo: '\\', Hi: '\\'}}}
	if got := compile.CharClassStringControlled(cl, &cancel.Control{Ctx: context.Background()}); got != `[\-\[\]\^\\]` {
		t.Fatalf("escaped descriptor %q", got)
	}
	rs := contextClassRanges()
	cl = &ast.CharClass{Ranges: rs}
	accept := closure.ClassAccept(cl)
	rs[0] = ast.CharRange{Lo: 'a', Hi: 'a'}
	if !accept(&state.Parser{}, 256) || accept(&state.Parser{}, 'a') {
		t.Fatal("matcher aliases source range slice")
	}
}

func TestClassConstructionCompileContextParity(t *testing.T) {
	t.Parallel()
	for _, shape := range []string{"single", "choice", "scan"} {
		cl := &ast.CharClass{Ranges: []ast.CharRange{{Lo: 'a', Hi: 'f'}, {Lo: '0', Hi: '9'}, {Lo: 'β', Hi: 'β'}}}
		var e ast.Expr = cl
		if shape == "choice" {
			e = &ast.Choice{Alts: []ast.Expr{cl, &ast.Literal{Value: "z"}}}
		}
		if shape == "scan" {
			e = &ast.Discard{Expr: &ast.Repeat{Expr: cl, Min: 1, Max: -1}}
		}
		g := &ast.Grammar{File: ast.File{Decls: []ast.Decl{&ast.RuleDef{Name: "main", Expr: e}}}}
		starttest.MarkAll(g)
		for _, compiledRecognition := range []bool{false, true} {
			a, err := Compile(g, compile.Options{Recognition: recognitionMode(compiledRecognition)})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			b, err := CompileContext(ctx, g, compile.Options{Recognition: recognitionMode(compiledRecognition)})
			cancel()
			if err != nil {
				t.Fatal(err)
			}
			for _, unit := range []core.PositionUnit{core.Bytes, core.CodePoints} {
				for _, recognize := range []bool{false, true} {
					for _, active := range []bool{false, true} {
						for _, s := range []string{"a", "β", "z", "?", "", "aβ1"} {
							// Active budget controls and Recognize lazily compile separate graphs.
							// Their disabled cases exercise the controlled compiler's own matchers,
							// including its directly compiled recognition-only graph.
							opts := ParseOptions{Unit: unit, Recognize: recognize}
							if active {
								opts.Limits = core.Limits{Compute: 1 << 30}
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
	cl := &ast.CharClass{Ranges: []ast.CharRange{{Lo: 'a', Hi: 'a'}, {Lo: 'b', Hi: 'b'}, {Lo: 'β', Hi: 'β'}}}
	g := &ast.Grammar{File: ast.File{Decls: []ast.Decl{&ast.RuleDef{Name: "main", Expr: &ast.Choice{Alts: []ast.Expr{&ast.Seq{Items: []ast.Expr{cl, &ast.EndInput{}}}, &ast.Discard{Expr: &ast.Repeat{Expr: cl, Min: 1, Max: -1}}}}}}}}
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
