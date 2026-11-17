package fmt

import (
	"strconv"
	"strings"
	"testing"

	"zeroparse.dev/zp/pkg/grammar/ast"
)

func TestFormatSourceGroupOpeningComments(t *testing.T) {
	for _, oneLine := range []bool{false, true} {
		for _, comment := range []string{"/* documentation */", "// documentation"} {
			open := &ast.LineBreak{Comments: []*ast.Comment{{Text: comment}}}
			for _, out := range []string{
				FormatType(&ast.TypeGroup{Type: &ast.TypeRef{Name: "T"}, OpenBreak: open, OneLine: oneLine}),
				FormatActionExpr(&ast.ActionGroup{Action: &ast.IntLit{Value: 1}, OpenBreak: open, OneLine: oneLine}),
			} {
				if strings.Count(out, comment) != 1 || !strings.HasPrefix(out, "(") || !strings.HasSuffix(out, ")") {
					t.Fatalf("opening comment lost or duplicated: %q", out)
				}
			}
		}
	}
}

func TestFormatMinimumInteger(t *testing.T) {
	max := int(^uint(0) >> 1)
	minimum := &ast.IntLit{Value: -max - 1}
	expr := "-" + strconv.Itoa(max) + " - 1"
	for _, tc := range []struct {
		term ast.ActionExpr
		want string
	}{
		{minimum, expr},
		{&ast.Unary{Op: "-", X: minimum}, "-(" + expr + ")"},
		{&ast.Binary{Op: "+", L: minimum, R: &ast.IntLit{Value: 1}}, expr + " + 1"},
		{&ast.Binary{Op: "-", L: &ast.IntLit{Value: 1}, R: minimum}, "1 - (" + expr + ")"},
		{&ast.Binary{Op: "*", L: minimum, R: &ast.IntLit{Value: 2}}, "(" + expr + ") * 2"},
	} {
		if got := FormatActionExpr(tc.term); got != tc.want {
			t.Errorf("FormatActionExpr: got %q; want %q", got, tc.want)
		}
	}
}

func TestFormatMinusFragments(t *testing.T) {
	for _, tc := range []struct {
		expr ast.Expr
		want string
	}{
		{&ast.Discard{Expr: &ast.Discard{Expr: &ast.Literal{Value: "a"}}}, `- -"a"`},
		{&ast.Discard{Expr: &ast.Cut{}}, "-#cut"},
	} {
		if got := FormatExpr(tc.expr); got != tc.want {
			t.Errorf("FormatExpr: got %q, want %q", got, tc.want)
		}
	}
	for _, x := range []ast.ActionExpr{
		&ast.Unary{Op: "-", X: &ast.IntLit{Value: 1}},
		&ast.IntLit{Value: -1},
	} {
		if got := FormatActionExpr(&ast.Unary{Op: "-", X: x}); got != "- -1" {
			t.Errorf("FormatActionExpr: got %q, want %q", got, "- -1")
		}
	}
}

// A list with a trailing comma has one element per line; without one it stays on
// one line. The comma of a one-element tuple does not count.
func TestFormatTrailingComma(t *testing.T) {
	one, two := &ast.IntLit{Value: 1}, &ast.IntLit{Value: 2}
	for _, tc := range []struct {
		name string
		term ast.ActionExpr
		want string
	}{
		{"call", &ast.Call{Func: "f", Args: []ast.ActionExpr{one, two}}, "f(1, 2)"},
		{"call with a trailing comma", &ast.Call{Func: "f", Args: []ast.ActionExpr{one, two}, TrailingComma: true}, "f(\n    1,\n    2,\n)"},
		{"constructor", &ast.New{Type: "N", Args: []ast.ActionExpr{one}}, "N{1}"},
		{"constructor with a trailing comma", &ast.New{Type: "N", Args: []ast.ActionExpr{one}, TrailingComma: true}, "N{\n    1,\n}"},
		{"list constructor", &ast.ListConstructor{Elem: &ast.TypeRef{Name: "int"}, Items: []ast.ActionExpr{one, two}}, "[]int{1, 2}"},
		{"list constructor with a trailing comma", &ast.ListConstructor{Elem: &ast.TypeRef{Name: "int"}, Items: []ast.ActionExpr{one, two}, TrailingComma: true}, "[]int{\n    1,\n    2,\n}"},
		{"tuple", &ast.TupleLit{Elems: []ast.ActionExpr{one, two}}, "(1, 2)"},
		{"tuple with a trailing comma", &ast.TupleLit{Elems: []ast.ActionExpr{one, two}, TrailingComma: true}, "(\n    1,\n    2,\n)"},
		{"tuple of one element", &ast.TupleLit{Elems: []ast.ActionExpr{one}}, "(1,)"},
		{"tuple of one element with a trailing comma", &ast.TupleLit{Elems: []ast.ActionExpr{one}, TrailingComma: true}, "(1,)"},
		{"nested", &ast.Call{Func: "f", Args: []ast.ActionExpr{&ast.TupleLit{Elems: []ast.ActionExpr{one, two}, TrailingComma: true}}}, "f(\n    (\n        1,\n        2,\n    ),\n)"},
	} {
		if got := FormatActionExpr(tc.term); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
	a, b := &ast.TypeRef{Name: "A"}, &ast.TypeRef{Name: "B"}
	for _, tc := range []struct {
		name string
		typ  ast.TypeExpr
		want string
	}{
		{"tuple type", &ast.TupleType{Types: []ast.TypeExpr{a, b}}, "(A, B)"},
		{"tuple type with a trailing comma", &ast.TupleType{Types: []ast.TypeExpr{a, b}, TrailingComma: true}, "(\n    A,\n    B,\n)"},
		{"tuple type of one element with a trailing comma", &ast.TupleType{Types: []ast.TypeExpr{a}, TrailingComma: true}, "(A,)"},
		{"type arguments", &ast.TypeApp{Name: "node", Args: []ast.TypeExpr{a}}, "node[A]"},
		{"type arguments with a trailing comma", &ast.TypeApp{Name: "node", Args: []ast.TypeExpr{a}, TrailingComma: true}, "node[\n    A,\n]"},
	} {
		if got := FormatType(tc.typ); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestTupleLitFormat(t *testing.T) {
	for _, tc := range []struct {
		term ast.ActionExpr
		want string
	}{
		{&ast.TupleLit{Elems: []ast.ActionExpr{}}, "()"},
		{&ast.TupleLit{Elems: []ast.ActionExpr{&ast.IntLit{Value: 1}}}, "(1,)"},
		{&ast.TupleLit{Elems: []ast.ActionExpr{&ast.IntLit{Value: 1}, &ast.VarRef{Name: "x"}}}, "(1, x)"},
		{&ast.Binary{Op: "[]", L: &ast.TupleLit{Elems: []ast.ActionExpr{&ast.IntLit{Value: 1}, &ast.IntLit{Value: 2}}}, R: &ast.IntLit{Value: 0}}, "(1, 2)[0]"},
	} {
		if got := FormatActionExpr(tc.term); got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}
