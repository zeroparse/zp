package ast_test

import (
	"errors"
	"strings"
	"testing"

	"zeroparse.dev/zp/pkg/grammar/ast"
)

// Embedded AST nodes promote their interface methods even in other packages.
type wrappedExpr struct {
	*ast.Top
	Data []int
	Pos  int
}
type wrappedStatement struct {
	*ast.RuleDef
	Data []int
}
type wrappedType struct {
	*ast.TypeRef
	Data []int
}
type wrappedTypeDecl struct {
	*ast.TypeDef
	Data []int
}
type wrappedTerm struct {
	*ast.NilLit
	Data []int
}

func TestValidateUnsupportedEmbeddedNodes(t *testing.T) {
	for _, g := range []*ast.Grammar{
		{File: ast.File{Decls: []ast.Decl{wrappedStatement{}}}},
		{File: ast.File{Decls: []ast.Decl{wrappedTypeDecl{}}}},
		{File: ast.File{Decls: []ast.Decl{&ast.RuleDef{Name: "p", Type: wrappedType{}, Expr: &ast.Top{}}}}},
		{File: ast.File{Decls: []ast.Decl{&ast.RuleDef{Name: "p", Expr: wrappedExpr{}}}}},
		{File: ast.File{Decls: []ast.Decl{&ast.RuleDef{Name: "p", Expr: &wrappedExpr{}}}}},
		{File: ast.File{Decls: []ast.Decl{&ast.RuleDef{Name: "p", Expr: &ast.Top{}, Action: wrappedTerm{}}}}},
	} {
		var e *ast.ValidationError
		if err := ast.Validate(g); !errors.As(err, &e) || !strings.Contains(e.Msg, "unsupported") {
			t.Fatalf("got %v; want unsupported-node validation error", err)
		}
	}
}
