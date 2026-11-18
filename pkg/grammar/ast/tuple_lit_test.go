package ast

import (
	"reflect"
	"strings"
	"testing"
)

func tupleGrammar() *Grammar {
	return &Grammar{File: File{Decls: []Decl{&RuleDef{
		Name: "p", Expr: &Literal{Value: "a"},
		Action: &Call{Func: "list", Args: []ActionExpr{
			&TupleLit{Elems: []ActionExpr{}},
			&TupleLit{Elems: []ActionExpr{&IntLit{Value: 1}}},
			&TupleLit{Elems: []ActionExpr{&IntLit{Value: 1}, &TupleLit{Elems: []ActionExpr{&StringLit{Value: "x"}, &NilLit{}}}}},
		}},
	}}}}
}

func TestTupleLitJSON(t *testing.T) {
	g := tupleGrammar()
	data, err := MarshalJSON(g)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"@type": "TupleLit"`) || !strings.Contains(string(data), `"elems"`) {
		t.Fatalf("tuple literal is not encoded as a TupleLit:\n%s", data)
	}
	for name, decode := range map[string]func([]byte) (*Grammar, error){"UnmarshalJSON": UnmarshalJSON, "DecodeJSON": DecodeJSON} {
		got, err := decode(data)
		if err != nil {
			t.Fatalf("%s: %v\n%s", name, err, data)
		}
		if !reflect.DeepEqual(g, got) {
			t.Errorf("%s: round trip mismatch\n%s", name, data)
		}
	}
}
