package compile

import (
	"context"
	"errors"
	"testing"

	"zeroparse.dev/zp/pkg/grammar/ast"
)

func TestCompileContextCancelsDuringGrammarValidation(t *testing.T) {
	t.Parallel()
	items := make([]ast.Expr, 1024)
	for i := range items {
		items[i] = &ast.Literal{Value: "a"}
	}
	items[len(items)-1] = nil
	g := &ast.Grammar{File: ast.File{Decls: []ast.Decl{
		&ast.RuleDef{Name: "main", Expr: &ast.Seq{Items: items}},
	}}}
	if _, err := compileStatic(g, Options{}, nil); err == nil {
		t.Fatal("fixture must fail ordinary grammar validation")
	}
	// CompileContext and buildControlled make two entry checks, followed by
	// ValidateContext's entry check. The fourth check is within AST traversal.
	ctx := compileTestContext(t, 4)
	prog, err := compileContext(ctx, g, Options{})
	if prog != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("validation cancellation: program=%v error=%v", prog, err)
	}
	if ctx.left > 0 {
		t.Fatal("compiler did not poll inside grammar validation")
	}
}
