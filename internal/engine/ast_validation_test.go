package engine

import (
	"strings"
	"testing"
	"zeroparse.dev/zp/internal/diagnostic"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/testing/codegentest"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

func TestMalformedASTEntryPointsAndPosition(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testMalformedASTEntryPointsAndPosition)
}

func testMalformedASTEntryPointsAndPosition(t *testing.T) {
	e := &ast.Optional{SourceInfo: ast.SourceInfo{Pos: ast.Pos{Line: 3, Col: 4}}}
	e.Expr = e
	g := &ast.Grammar{File: ast.File{Decls: []ast.Decl{&ast.RuleDef{Name: "p", Expr: e}}}}
	for name, run := range map[string]func() error{
		"compile":           func() error { _, err := Compile(g, compile.Options{}); return err },
		"saved AST rebuild": func() error { _, err := build(g, compile.Options{}, []compile.RuleFlags{{}}); return err },
		"Go generator": func() error {
			_, err := codegentest.GenerateCode(t, g, codegen.GenOptions{Start: "p", Package: "p"})
			return err
		},
		"typed Go generator": func() error {
			_, err := codegentest.GenerateCode(t, g, codegen.GenOptions{Start: "p", Package: "p", Types: true})
			return err
		},
		"TS generator": func() error { _, err := codegentest.GenerateTSBundle(g, codegen.GenOptions{Start: "p"}); return err },
	} {
		t.Run(name, func(t *testing.T) {
			err := run()
			if errs, ok := err.(diagnostic.ErrorList); !ok || len(errs) != 1 || errs[0].Pos != e.Pos || !strings.Contains(err.Error(), "$.file.decls[0].expr.expr") {
				t.Fatalf("got %v; want positioned pointer-cycle error", err)
			}
		})
	}
}
