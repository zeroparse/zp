package ast_test

import (
	"slices"
	"testing"

	"zeroparse.dev/zp/engine"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

func TestTypeRefs(t *testing.T) {
	g, err := engine.ParseGrammar("type Box = { V any; L []Box; O *string }\ntype U ~ Box | Other\ntype Other = { Text string }\n" +
		"rule main Box = x:\"x\" => Box{V: $x.(any), L: []Box{}, O: nil}\n")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range ast.TypeRefs(g) {
		got = append(got, r.Name)
	}
	want := []string{"any", "Box", "string", "Box", "Other", "string", "Box", "any", "Box"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	if refs := ast.TypeRefs(nil); refs != nil {
		t.Errorf("nil: got %v", refs)
	}
}
