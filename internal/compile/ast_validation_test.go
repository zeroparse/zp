package compile

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/pkg/grammar/ast"
)

func TestCompileMalformedAST(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		g    *ast.Grammar
		want string
	}{
		{"nil grammar", nil, "$"},
		{"nil field", &ast.Grammar{File: ast.File{Decls: []ast.Decl{&ast.TypeDef{Name: "T", Type: &ast.StructType{Fields: []*ast.Field{nil}}}}}}, "$.file.decls[0].type.fields[0]"},
		{"reversed repeat", &ast.Grammar{File: ast.File{Decls: []ast.Decl{&ast.RuleDef{Name: "p", Expr: &ast.Repeat{Expr: &ast.Literal{Value: "a"}, Min: 2, Max: 1}}}}}, "$.file.decls[0].expr.max"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, opts := range []Options{{}} {
				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Errorf("Compile panicked: %v", r)
						}
					}()
					p, err := compileStatic(tc.g, opts, nil)
					if p != nil || err == nil || !strings.Contains(err.Error(), tc.want) {
						t.Errorf("got program %v, error %v; want %s", p != nil, err, tc.want)
					}
				}()
			}
		})
	}
}
