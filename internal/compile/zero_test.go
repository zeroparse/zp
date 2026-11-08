package compile

import (
	"strings"
	"testing"
)

// TestZeroValues holds the rule of definite assignment: a constructor may leave out a field only if
// the declared type of the field has a zero value (spec/language/types.md#zero-values).
func TestZeroValues(t *testing.T) {
	t.Parallel()
	const prelude = `type Inner = { N int; S string }
type Deep = { In Inner; Xs []Inner; Pair (int, string); Opt *node[string] }
type Cycle = { Next Cycle }
type Loop = { Back *Loop; Tag int }
type Choice ~ Inner | Loop
type Kid = node[string]
`
	for _, tc := range []struct {
		name, ftype string
		zero        bool
	}{
		{"int", "int", true},
		{"string", "string", true},
		{"bool", "bool", true},
		{"list", "[]int", true},
		{"list of nodes", "[]node[string]", true},
		{"optional node", "*node[string]", true},
		{"optional join", "*(Inner | Loop)", true},
		{"top", "_", true},
		{"tuple", "(int, string)", true},
		{"unit", "()", true},
		{"struct with zero fields", "Inner", true},
		{"nested struct", "Deep", true},
		{"struct through an optional cycle", "Loop", true},
		{"node", "node[string]", false},
		{"node of a tuple", "node[(int, int)]", false},
		{"named node type", "Kid", false},
		{"tuple with a node", "(int, node[string])", false},
		{"struct with a node field", "Holder", false},
		{"struct cycle", "Cycle", false},
		{"join", "Inner | Loop", false},
		{"named join", "Choice", false},
		{"error", "error", false},
		{"join with error", "Inner | error", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g, err := parseTest(prelude + "type Holder = { K node[string] }\ntype Box = { F " + tc.ftype + "; Pad int }\n@start\nrule main Box = \"x\" => Box{Pad: 1}\n")
			if err != nil {
				t.Fatal(err)
			}
			_, err = compileStatic(g, Options{}, nil)
			switch {
			case tc.zero && err != nil:
				t.Fatalf("a field of type %s has a zero value, got %v", tc.ftype, err)
			case !tc.zero && err == nil:
				t.Fatalf("a field of type %s has no zero value, but leaving it out compiles", tc.ftype)
			case !tc.zero && !strings.Contains(err.Error(), "field F of Box has type "+tc.ftype+", which has no zero value: set it in the constructor, or declare it as *"):
				t.Fatalf("message: %v", err)
			}
		})
	}
	// The error names the optional type to declare, and a constructor that sets the field compiles.
	g, err := parseTest("type Box = { F node[string] | error }\n@start\nrule main Box = f:\"x\" => Box{F: $f}\n")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := compileStatic(g, Options{}, nil); err != nil {
		t.Fatalf("a field that is set: %v", err)
	}
}
