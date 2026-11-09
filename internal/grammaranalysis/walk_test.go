package grammaranalysis

import (
	"reflect"
	"testing"

	"zeroparse.dev/zp/grammar"
)

// The walk reaches the expression arguments of attributes and every part of an Operators Rule,
// and polling does not change what it visits.
func TestWalkExprVisitsEveryPart(t *testing.T) {
	t.Parallel()
	lit := func(s string) *grammar.Literal { return &grammar.Literal{Value: s} }
	e := &grammar.Seq{Items: []grammar.Expr{
		&grammar.Attributed{Expr: lit("a"), Attrs: []*grammar.Attribute{{Name: "recover", Pragma: true, KwArgs: map[string]any{"skip": lit("skip")}}}},
		&grammar.OperatorsRule{
			Skip:     lit("ws"),
			Operands: []*grammar.OperatorsRuleOperand{{Expr: lit("operand")}},
			Levels:   []*grammar.OperatorsRuleLevel{{Operators: []*grammar.OperatorsRuleOperator{{Expr: lit("op")}}}},
		},
		&grammar.Recovery{Expr: lit("body"), Skip: lit("rskip")},
	}}
	literals := func(walk func(func(grammar.Expr))) []string {
		var out []string
		walk(func(x grammar.Expr) {
			if l, ok := x.(*grammar.Literal); ok {
				out = append(out, l.Value)
			}
		})
		return out
	}
	want := []string{"a", "skip", "ws", "operand", "op", "body", "rskip"}
	if got := literals(func(f func(grammar.Expr)) { WalkExpr(e, f) }); !reflect.DeepEqual(got, want) {
		t.Fatalf("WalkExpr visited %q, want %q", got, want)
	}
	polls := 0
	if got := literals(func(f func(grammar.Expr)) { WalkExprWithPoll(e, f, func() { polls++ }) }); !reflect.DeepEqual(got, want) {
		t.Fatalf("WalkExprWithPoll visited %q, want %q", got, want)
	}
	if polls == 0 {
		t.Fatal("WalkExprWithPoll never polled")
	}
}
