package ref

import (
	"testing"

	syntaxfmt "zeroparse.dev/zp/internal/syntax/fmt"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// TestTypeTextMatchesFormatter checks that the type printer of the evaluator spells every kind of
// type as the formatter does when the type has no layout.
func TestTypeTextMatchesFormatter(t *testing.T) {
	r := func(n string) ast.TypeExpr { return &ast.TypeRef{Name: n} }
	join := func(ts ...ast.TypeExpr) ast.TypeExpr { return &ast.JoinType{Types: ts} }
	meet := func(ts ...ast.TypeExpr) ast.TypeExpr { return &ast.MeetType{Types: ts} }
	for _, typ := range []ast.TypeExpr{
		r("int"),
		&ast.ListType{Elem: r("string")},
		&ast.ListType{Elem: join(r("A"), meet(r("B"), r("C")))},
		&ast.OptionalType{Elem: meet(r("A"), r("B"))},
		&ast.ListType{Elem: &ast.OptionalType{Elem: r("A")}},
		join(r("A"), join(r("B"), r("C"))),
		meet(r("B"), join(r("A"), r("C"))),
		&ast.TupleType{},
		&ast.TupleType{Types: []ast.TypeExpr{r("int")}},
		&ast.TupleType{Types: []ast.TypeExpr{r("int"), &ast.ListType{Elem: r("bool")}}},
		&ast.StructType{},
		&ast.StructType{Fields: []*ast.Field{{Name: "a", Type: r("int")}, {Name: "b", Type: &ast.ListType{Elem: r("string")}}}},
		&ast.TypeApp{Name: "Node", Args: []ast.TypeExpr{r("A"), join(r("B"), r("C"))}},
		&ast.TypeGroup{Type: join(r("A"), r("B")), OneLine: true},
	} {
		if got, want := typeText(typ), syntaxfmt.FormatType(typ); got != want {
			t.Errorf("typeText = %q, the formatter writes %q", got, want)
		}
	}
}
