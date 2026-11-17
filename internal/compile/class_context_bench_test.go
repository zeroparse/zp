package compile

import (
	"context"
	"testing"

	"zeroparse.dev/zp/internal/testing/starttest"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

func BenchmarkClassCompileCancellation(b *testing.B) {
	ranges := make([]ast.CharRange, 512)
	for i := range ranges {
		ranges[i] = ast.CharRange{Lo: rune(256 + 2*i), Hi: rune(256 + 2*i)}
	}
	class := &ast.CharClass{Ranges: ranges}
	g := &ast.Grammar{File: ast.File{Decls: []ast.Decl{&ast.RuleDef{Name: "main", Expr: &ast.Choice{Alts: []ast.Expr{class, &ast.Discard{Expr: &ast.Repeat{Expr: class, Min: 1, Max: -1}}}}}}}}
	starttest.MarkAll(g)
	for _, controlled := range []bool{false, true} {
		name := "ordinary"
		if controlled {
			name = "cancellable"
		}
		b.Run(name, func(b *testing.B) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			b.ReportAllocs()
			for b.Loop() {
				var err error
				if controlled {
					normalizationBenchmarkProgram, err = compileContext(ctx, g, Options{})
				} else {
					normalizationBenchmarkProgram, err = compileStatic(g, Options{}, nil)
				}
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
