package codegen_test

import (
	"strings"
	"testing"
	"zeroparse.dev/zp/internal/diagnostic"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/engine"
	"zeroparse.dev/zp/internal/testing/codegentest"
	"zeroparse.dev/zp/internal/testing/starttest"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

const streamRecoveryError = "#recover cannot wrap a #stream repetition; recover inside each element instead"

func TestStreamRecoveryRejected(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testStreamRecoveryRejected)
}

func testStreamRecoveryRejected(t *testing.T) {
	for _, pragmas := range []string{`#stream #recover(skip=.*)`, `#recover(skip=.*) #stream`} {
		src := `rule main = "ab" #repeat(2) ` + pragmas + ` $$`
		g, err := parseTest(src)
		if err != nil {
			t.Fatal(err)
		}
		for _, opts := range []compile.Options{{}, {Recognition: compile.RecognizeKeepStreamValues}} {
			if _, err := engine.Compile(g, opts); err == nil || !strings.Contains(err.Error(), streamRecoveryError) {
				t.Errorf("%s: %v", src, err)
			}
		}
		for name, generate := range map[string]func() error{
			"Go": func() error {
				_, err := codegentest.GenerateCode(t, g, codegen.GenOptions{Start: "main", Package: "test"})
				return err
			},
			"typed Go": func() error {
				_, err := codegentest.GenerateCode(t, g, codegen.GenOptions{Start: "main", Package: "test", Types: true})
				return err
			},
			"TS": func() error { _, err := codegentest.GenerateTSBundle(g, codegen.GenOptions{Start: "main"}); return err },
		} {
			if err := generate(); err == nil || !strings.Contains(err.Error(), streamRecoveryError) {
				t.Errorf("%s generation: %v", name, err)
			}
		}
	}
	pos := ast.Pos{Line: 3, Col: 10}
	g := &ast.Grammar{File: ast.File{Decls: []ast.Decl{&ast.RuleDef{Name: "main", Expr: &ast.Recovery{SourceInfo: ast.SourceInfo{Pos: pos}, Expr: &ast.Stream{Expr: &ast.Repeat{
		Expr: &ast.Literal{Value: "ab"}, Min: 2, Max: 2,
	}},
		Skip: &ast.Repeat{Expr: &ast.Any{}, Min: 0, Max: -1},
	}}}}}
	starttest.MarkAll(g)
	_, err := engine.Compile(g, compile.Options{})
	if errs, ok := err.(diagnostic.ErrorList); !ok || len(errs) != 1 || errs[0].Pos != pos || !strings.Contains(err.Error(), streamRecoveryError) {
		t.Errorf("public AST: %v; want positioned recovery error", err)
	}
}
