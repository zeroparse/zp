package desugar

import (
	"reflect"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/syntax"
)

func TestNormalizationContainerShapes(t *testing.T) {
	t.Parallel()
	literal := &grammar.Literal{Value: "a"}
	for _, expr := range []grammar.Expr{
		&grammar.Seq{}, &grammar.Seq{Items: []grammar.Expr{}},
		&grammar.Choice{}, &grammar.Choice{Alts: []grammar.Expr{}},
		&grammar.OperatorsRule{},
		&grammar.OperatorsRule{Operands: []*grammar.OperatorsRuleOperand{}, Levels: []*grammar.OperatorsRuleLevel{{}}},
		&grammar.Attributed{Expr: literal},
		&grammar.Attributed{Expr: literal, Attrs: []*grammar.Attribute{{Name: "custom"}}},
		&grammar.Attributed{Expr: literal, Attrs: []*grammar.Attribute{{Name: "custom", Args: []any{}, KwArgs: map[string]any{}, KeywordOrder: []string{}}}},
		&grammar.Attributed{Expr: literal, Attrs: []*grammar.Attribute{{Name: "custom", Args: []any{nil, map[string]any{"x": []int{1, 2}}}, KwArgs: map[string]any{"x": []int{3}}}}},
	} {
		g := &grammar.Grammar{File: grammar.File{Decls: []grammar.Decl{&grammar.RuleDef{Name: "main", Expr: expr}}}}
		got, gotErr := NormalizeAttributesWithCopies(g, false)
		want, wantErr := NormalizeAttributesWithCopies(g, true)
		if !reflect.DeepEqual(gotErr, wantErr) || !reflect.DeepEqual(got, want) {
			t.Fatalf("%T: normalized shape differs", expr)
		}
	}
	for _, statements := range [][]grammar.Decl{nil, {}} {
		g := &grammar.Grammar{File: grammar.File{Decls: statements}}
		got, _ := NormalizeAttributesWithCopies(g, false)
		want, _ := NormalizeAttributesWithCopies(g, true)
		if !reflect.DeepEqual(got, want) {
			t.Fatal("empty grammar shape differs")
		}
	}
}

func TestNormalizationAttributeBindingDoesNotMutate(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"error", "recover"} {
		key := "message"
		if name == "recover" {
			key = "skip"
		}
		a := &grammar.Attribute{Name: name, Pragma: name == "recover", Args: []any{&grammar.Literal{Value: "x"}}, KwArgs: map[string]any{"custom": []int{1, 2}}}
		n := &attributeNormalizer{}
		bound := n.bind(n.attribute(a), key)
		if len(a.Args) != 1 || len(a.KwArgs) != 1 || len(bound.Args) != 0 || len(bound.KwArgs) != 2 {
			t.Fatal("binding mutated source")
		}
		if _, found := a.KwArgs[key]; found {
			t.Fatal("keyword inserted in source")
		}
	}
}

func TestNormalizationSharesUnchangedNodes(t *testing.T) {
	t.Parallel()
	g, err := syntax.Parse(`rule main = "a" "b"*`)
	if err != nil {
		t.Fatal(err)
	}
	normalized, err := normalizeAttributes(g)
	if err != nil {
		t.Fatal(err)
	}
	if normalized != g {
		t.Fatal("unchanged grammar was copied")
	}
	g, err = syntax.Parse("rule main = (\"a\") \"b\"*\nrule other = \"c\"")
	if err != nil {
		t.Fatal(err)
	}
	normalized, err = normalizeAttributes(g)
	if err != nil {
		t.Fatal(err)
	}
	if normalized == g || normalized.Rules()[0] == g.Rules()[0] || normalized.Rules()[1] != g.Rules()[1] {
		t.Fatal("changed path sharing differs")
	}
	if normalized.Rules()[0].Expr.(*grammar.Seq).Items[1] != g.Rules()[0].Expr.(*grammar.Seq).Items[1] {
		t.Fatal("unchanged sibling copied")
	}
}
