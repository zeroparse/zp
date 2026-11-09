package compile

import (
	"fmt"
	"strings"
	"testing"
	"time"
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
		{"empty list type", "[]_|_", true},
		{"nil type", "*_|_", true},
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

// TestStructZeroDescriptors pins the descriptors that the runtimes build the zero value of an unset
// field from, in declaration order.
func TestStructZeroDescriptors(t *testing.T) {
	t.Parallel()
	g, err := parseTest(`type Inner = { N int; S string }
type Loop = { Back *Loop; Tag int }
type Alias ~ Inner
type Z = { I int; S string; B bool; L []Inner; O *int; T (int, (string, bool)); U (); In Inner; A _; Lp Loop; Al Alias; K node[string] | error; E []_|_; N *_|_ }
@start
rule main Z = k:"x" => Z{K: $k}
`)
	if err != nil {
		t.Fatal(err)
	}
	prog, err := compileStatic(g, Options{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	z := prog.StructZeroOf("Z")
	for _, f := range z.Names {
		got = append(got, f+"="+z.Zeros[f])
	}
	want := "I=i S=s B=b L=l O=n T=(i(sb)) U=() In={Inner} A=n Lp={Loop} Al={Inner} K=- E=l N=n"
	if s := strings.Join(got, " "); s != want {
		t.Errorf("descriptors\n got %s\nwant %s", s, want)
	}
	if prog.StructZeroOf("Nope") != nil {
		t.Error("a name that is not a struct type has descriptors")
	}
}

// TestZeroValueOfDeepStructTypesIsLinear pins that the zero descriptors of a chain of struct types, each
// holding the next twice, are found in time linear in the number of types: the same type is reached
// by 2^depth paths, and visiting every path takes minutes at this depth.
func TestZeroValueOfDeepStructTypesIsLinear(t *testing.T) {
	t.Parallel()
	const depth = 40
	var src strings.Builder
	for i := range depth {
		fmt.Fprintf(&src, "type S%d = { X S%d; Y S%d }\n", i, i+1, i+1)
	}
	fmt.Fprintf(&src, "type S%d = { V int }\ntype Top = { K node[string]; In S0 }\n", depth)
	src.WriteString("@start\nrule main Top = k:\"x\" => Top{K: $k}\n")
	g, err := parseTest(src.String())
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan *Program, 1)
	go func() {
		prog, err := compileStatic(g, Options{}, nil)
		if err != nil {
			t.Error(err)
		}
		done <- prog
	}()
	select {
	case prog := <-done:
		if prog == nil {
			return
		}
		if z := prog.StructZeroOf("Top"); z == nil || z.Zeros["In"] != "{S0}" || z.Zeros["K"] != NoZeroDesc {
			t.Errorf("descriptors of Top: %+v", z)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("the zero descriptors of a chain of struct types take time that grows with the paths through it")
	}
}
