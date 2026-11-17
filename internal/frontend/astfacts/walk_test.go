package astfacts

import (
	"reflect"
	"testing"

	"zeroparse.dev/zp/pkg/grammar/ast"
)

// The walk reaches the expression arguments of attributes and every part of an Operators Rule,
// and polling does not change what it visits.
func TestWalkExprVisitsEveryPart(t *testing.T) {
	t.Parallel()
	lit := func(s string) *ast.Literal { return &ast.Literal{Value: s} }
	e := &ast.Seq{Items: []ast.Expr{
		&ast.Attributed{Expr: lit("a"), Attrs: []*ast.Attribute{{Name: "recover", Pragma: true, KwArgs: map[string]any{"skip": lit("skip")}}}},
		&ast.OperatorsRule{
			Skip:     lit("ws"),
			Operands: []*ast.OperatorsRuleOperand{{Expr: lit("operand")}},
			Levels:   []*ast.OperatorsRuleLevel{{Operators: []*ast.OperatorsRuleOperator{{Expr: lit("op")}}}},
		},
		&ast.Recovery{Expr: lit("body"), Skip: lit("rskip")},
	}}
	literals := func(walk func(func(ast.Expr))) []string {
		var out []string
		walk(func(x ast.Expr) {
			if l, ok := x.(*ast.Literal); ok {
				out = append(out, l.Value)
			}
		})
		return out
	}
	want := []string{"a", "skip", "ws", "operand", "op", "body", "rskip"}
	if got := literals(func(f func(ast.Expr)) { WalkExpr(e, f) }); !reflect.DeepEqual(got, want) {
		t.Fatalf("WalkExpr visited %q, want %q", got, want)
	}
	polls := 0
	if got := literals(func(f func(ast.Expr)) { WalkExprWithPoll(e, f, func() { polls++ }) }); !reflect.DeepEqual(got, want) {
		t.Fatalf("WalkExprWithPoll visited %q, want %q", got, want)
	}
	if polls == 0 {
		t.Fatal("WalkExprWithPoll never polled")
	}
}
