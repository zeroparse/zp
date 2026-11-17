package zpls

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/frontend/parser"
	"zeroparse.dev/zp/internal/testing/starttest"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

type lspCompilePollContext struct {
	context.Context
	cancel context.CancelFunc
	left   int
}

func (c *lspCompilePollContext) Err() error {
	c.left--
	if c.left == 0 {
		c.cancel()
	}
	return c.Context.Err()
}

func TestCompileCancellationDoesNotPublishDiagnostics(t *testing.T) {
	var source strings.Builder
	source.WriteString("@start\nrule main = r0\n")
	for i := range 1024 {
		fmt.Fprintf(&source, "rule r%d = \"a\"\n", i)
	}
	g, err := parser.Parse(source.String())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	controlled := &lspCompilePollContext{Context: ctx, cancel: cancel, left: 4}
	a := newAnalysis(source.String())
	a.g = g
	if err := a.compileContext(controlled); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected compile cancellation, got %v", err)
	}
	if a.prog != nil || len(a.diags) != 0 {
		t.Fatalf("unfinished compilation leaked program or diagnostics: %v, %+v", a.prog, a.diags)
	}
	// A cancellation must not poison the grammar or a subsequent snapshot.
	if err := a.compileContext(context.Background()); err != nil || a.prog == nil || len(a.diags) != 0 {
		t.Fatalf("subsequent ordinary compilation failed: %v, %+v", err, a.diags)
	}
}

func TestGrammarValidationCancellationDoesNotPublishDiagnostics(t *testing.T) {
	items := make([]ast.Expr, 1024)
	for i := range items {
		items[i] = &ast.Literal{Value: "a"}
	}
	items[len(items)-1] = nil
	a := newAnalysis(`rule main = "a"`)
	a.g = starttest.MarkAll(&ast.Grammar{File: ast.File{Decls: []ast.Decl{
		&ast.RuleDef{Name: "main", Expr: &ast.Seq{Items: items}},
	}}})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	controlled := &lspCompilePollContext{Context: ctx, cancel: cancel, left: 4}
	if err := a.compileContext(controlled); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected validation cancellation, got %v", err)
	}
	if a.prog != nil || len(a.diags) != 0 {
		t.Fatalf("cancelled validation published a result: %v, %+v", a.prog, a.diags)
	}
	items[len(items)-1] = &ast.Literal{Value: "a"}
	if err := a.compileContext(context.Background()); err != nil || a.prog == nil || len(a.diags) != 0 {
		t.Fatalf("later compilation failed: %v, %+v", err, a.diags)
	}
}
