package lint

import (
	"context"
	"testing"

	"zeroparse.dev/zp/pkg/grammar/ast"
)

func BenchmarkClassLintCancellation(b *testing.B) {
	ranges := make([]ast.CharRange, 512)
	for i := range ranges {
		ranges[i] = ast.CharRange{Lo: rune(256 + 2*(len(ranges)-i)), Hi: rune(256 + 2*(len(ranges)-i))}
	}
	class := &ast.CharClass{Ranges: ranges}
	g := &ast.Grammar{File: ast.File{Decls: []ast.Decl{&ast.RuleDef{Name: "main", Expr: &ast.Choice{Alts: []ast.Expr{class, &ast.Literal{Value: "a"}, &ast.CharClass{Ranges: ranges}}}}}}}
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
					proofLintBenchmarkFindings, err = testRunContext(ctx, g, "main", options{})
				} else {
					proofLintBenchmarkFindings = runUnchecked(g, "main", options{})
				}
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
