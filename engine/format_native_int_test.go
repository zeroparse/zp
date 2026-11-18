package engine_test

import (
	"fmt"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/engine"
	syntaxfmt "zeroparse.dev/zp/internal/syntax/fmt"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

func TestFormatNativeMinimumRoundTrip(t *testing.T) {
	max := int(^uint(0) >> 1)
	minimum := &ast.IntLit{Value: -max - 1}
	for _, tc := range []struct {
		name string
		term ast.ActionExpr
		want int
	}{
		{"literal", minimum, minimum.Value},
		{"negated", &ast.Unary{Op: "-", X: minimum}, minimum.Value},
		{"left addition", &ast.Binary{Op: "+", L: minimum, R: &ast.IntLit{Value: 1}}, minimum.Value + 1},
		{"right subtraction", &ast.Binary{Op: "-", L: &ast.IntLit{Value: 1}, R: minimum}, minimum.Value + 1},
		{"left division", &ast.Binary{Op: "/", L: minimum, R: &ast.IntLit{Value: 2}}, minimum.Value / 2},
		{"right multiplication", &ast.Binary{Op: "*", L: &ast.IntLit{Value: 2}, R: minimum}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, err := engine.ParseGrammar("type N = { V int }\n@start\nrule main = \"é\" $$ => N{V: 0}")
			if err != nil {
				t.Fatal(err)
			}
			g.Rules()[0].Action.(*ast.New).Fields[0].Value = tc.term
			data, err := ast.MarshalJSON(g)
			if err != nil {
				t.Fatal(err)
			}
			jsonAST, err := ast.UnmarshalJSON(data)
			if err != nil {
				t.Fatal(err)
			}
			source := syntaxfmt.Format(jsonAST)
			sourceAST, err := engine.ParseGrammar(source)
			if err != nil {
				t.Fatalf("formatted source cannot be parsed: %v\n%s", err, source)
			}
			if again := syntaxfmt.Format(sourceAST); again != source {
				t.Fatalf("format is not idempotent:\n%s\n%s", source, again)
			}
			for _, tree := range []*ast.Grammar{g, jsonAST, sourceAST} {
				p, err := engine.Compile(tree, "main")
				if err != nil {
					t.Fatal(err)
				}
				encoded, err := p.MarshalBinary()
				if err != nil {
					t.Fatal(err)
				}
				loaded, err := engine.LoadParser(encoded)
				if err != nil {
					t.Fatal(err)
				}
				for _, parser := range []*engine.Parser{p, loaded} {
					for _, backend := range []engine.Backend{engine.Closure, engine.Bytecode, engine.BytecodeIterative} {
						for _, unit := range []core.PositionUnit{core.CodePoints, core.Bytes} {
							n, err := parser.Parse("é", engine.WithBackend(backend), engine.WithUnit(unit))
							if parser == loaded && backend == engine.Closure {
								// A compiled grammar runs on the bytecode backends only.
								if err == nil {
									t.Fatalf("closure backend on a compiled grammar: %v", n)
								}
								continue
							}
							want := fmt.Sprintf("N{V: %d}", tc.want)
							if err != nil || n.String() != want {
								t.Fatalf("backend %v, unit %v: %v, %v; want %s", backend, unit, n, err, want)
							}
						}
					}
				}
			}
		})
	}
}
