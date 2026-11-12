package compile

import (
	"strings"
	"testing"
)

func TestTypedNodeConstructorErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ action, message string }{
		{`[]Int{nil}`, "cannot use"},
		{`[]Int{Other{}}`, "cannot use"},
		{`[]Int{x}`, "cannot use int as Int in list constructor"},
		{`[][]Int{list(x)}`, "cannot use []int as []Int in list constructor"},
		{`[]Int{$d}`, "cannot use _ as Int in list constructor: the type of the value is not known; convert it with a cast, as in value.(Int)"},
		{`[][]Int{list($d)}`, "in list constructor: the type of the value is not known; convert it with a cast, as in value.([]Int)"},
		{`Int{Text: 1}`, "cannot use int as string in field Text of Int"},
		{`Int{Text: nil}`, "cannot use nil as string in field Text of Int"},
		{`Int{Text: "a", Text: "b"}`, "duplicate field Text"},
		{`Other{Text: "x"}`, "Other has no field Text"},
		// A struct constructor names its fields.
		{`Int{"x"}`, "a struct constructor names its fields: write Int{Text: ...}"},
		{`Int{"a", "b"}`, "a struct constructor names its fields: write Int{Field: value}"},
	} {
		t.Run(tc.action, func(t *testing.T) {
			g, err := parseTest(`type Int = { Text string }
type Other = {}
var x int
rule main = {x := 1} d:dyn "x" => ` + tc.action + `
rule dyn _ = _ => 1`)
			if err != nil {
				t.Fatal(err)
			}
			_, err = compileStatic(g, Options{}, nil)
			if err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("got %v; want %q", err, tc.message)
			}
		})
	}
	// Lists may hold ints, strings and bools.
	for _, action := range []string{`[]_{}`, `[][]_{}`, `[]int{}`, `[][]string{}`, `[]int{1, 2}`, `[]*bool{nil, true}`} {
		g, err := parseTest("var x int\nrule main = {x := 1} \"x\" => " + action)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := compileStatic(g, Options{}, nil); err != nil {
			t.Errorf("%s: %v", action, err)
		}
	}
	// A text struct is an ordinary struct, whose constructor may leave Text unset or name it.
	for _, action := range []string{`Int{}`, `Int{Text: "0"}`} {
		g, err := parseTest("type Int = { Text string }\nrule main = \"x\" => " + action)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := compileStatic(g, Options{}, nil); err != nil {
			t.Errorf("%s: %v", action, err)
		}
	}
}

// A positional constructor of a struct is rejected in actions and predicates.
func TestPositionalStructConstructorErrors(t *testing.T) {
	t.Parallel()
	for _, source := range []string{
		`type Int = { Text string }
var value int
rule main = {value := 1} "x" => Int{value}`,
		`type Int = { Text string }
var value int
rule main = {value := 1} {Int{value} != nil} "x" / "x"`,
	} {
		g, err := parseTest(source)
		if err != nil {
			t.Fatal(err)
		}
		// The checker rejects these constructors when the grammar is compiled.
		if _, err := compileStatic(g, Options{}, nil); err == nil || !strings.Contains(err.Error(), "a struct constructor names its fields: write Int{Text: ...}") {
			t.Fatalf("compilation: %v", err)
		}
	}
}

// The text struct counterpart: a field of the wrong type is a compile error.
func TestConstructedTextStructEvaluationErrors(t *testing.T) {
	t.Parallel()
	for _, source := range []string{
		`type Int = { Text string }
var value int
rule main = {value := 1} "x" => Int{Text: value}`,
		`type Int = { Text string }
var value int
rule main = {value := 1} {Int{Text: value} != nil} "x" / "x"`,
	} {
		// The checker rejects these constructors when the grammar is compiled.
		if got := compileMessage(t, source); !strings.Contains(got, "cannot use int as string in field Text of Int") {
			t.Fatalf("compilation: %q", got)
		}
	}
}

func TestNodeConstructorShapes(t *testing.T) {
	t.Parallel()
	const header = "type Int = { Text string }\ntype Other = {}\nrule main = \"x\" => "
	for _, action := range []string{`[]any{}`, `Int{"a", "b"}`, `Other{"a"}`} {
		g, err := parseTest(header + action)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := compileStatic(g, Options{}, nil); err == nil {
			t.Fatalf("accepted unsupported constructor shape: %s", action)
		}
	}
	for _, action := range []string{`[]int{}`, `[][]string{}`} {
		if g, err := parseTest(header + action); err != nil {
			t.Fatal(err)
		} else if _, err := compileStatic(g, Options{}, nil); err != nil {
			t.Errorf("rejected a list of values %s: %v", action, err)
		}
	}
	if g, err := parseTest(header + `Int{}`); err != nil {
		t.Fatal(err)
	} else if _, err := compileStatic(g, Options{}, nil); err != nil {
		t.Fatalf("Int{} is a text struct with no field set: %v", err)
	}
	// Values of the wrong type are rejected when the grammar is compiled, in actions and predicates.
	for _, action := range []string{`[]Int{nil}`, `[]Int{Other{}}`, `[]Int{1}`, `Int{1}`} {
		g, err := parseTest(header + action)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := compileStatic(g, Options{}, nil); err == nil {
			t.Errorf("%s was accepted", action)
		}
	}
	for _, action := range []string{`[]Int{1}`, `Int{1}`} {
		g, err := parseTest("type Int = { Text string }\nrule main = {" + action + " != nil} \"x\" / \"x\"")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := compileStatic(g, Options{}, nil); err == nil {
			t.Errorf("predicate %s was accepted", action)
		}
	}
}
