package gengo

import "testing"

// The conversion of a named node type keys its struct literal by the embedded field, whose name is
// that of the Go type without pointer, package and type arguments.
func TestEmbeddedName(t *testing.T) {
	for typ, want := range map[string]string{
		"*TNode[string]":                  "TNode",
		"*types.TNode[any]":               "TNode",
		"*types.TNode[types.Tuple2[a,b]]": "TNode",
		"TNode[[]any]":                    "TNode",
		"Word":                            "Word",
	} {
		if got := embeddedName(typ); got != want {
			t.Errorf("embeddedName(%q) = %q, want %q", typ, got, want)
		}
	}
}
