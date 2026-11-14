package ref

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// unknownMutants each break one way in which the evaluation keeps Unknown out of known results.
var unknownMutants = []struct{ name, file, old, new string }{
	{"a member of Unknown is selected", "term.go", "V {\n\tneed(x)\n\tswitch x.Kind {\n\tcase KNode:", "V {\n\tswitch x.Kind {\n\tcase KNode:"},
	{"an operator reads Unknown", "term.go", "\tneed(l, r)\n", ""},
	{"a unary operator reads Unknown", "term.go", "need(x)\n\t\tv, msg := Unary", "v, msg := Unary"},
	{"Unknown has elements", "term.go", "[]V {\n\tneed(v)\n", "[]V {\n"},
	{"Unknown has a length", "term.go", "need(x)\n\t\tswitch x.Kind {\n\t\tcase KNil:\n\t\t\treturn Int(0)", "switch x.Kind {\n\t\tcase KNil:\n\t\t\treturn Int(0)"},
	{"Unknown has a text", "term.go", "need(x)\n\t\tswitch x.Kind {\n\t\tcase KNil:\n\t\t\treturn Str(\"\")", "switch x.Kind {\n\t\tcase KNil:\n\t\t\treturn Str(\"\")"},
	{"filter keeps by Unknown", "term.go", "\t\t\tneed(keep)\n", ""},
	{"a type admits or rejects Unknown", "term.go", "bool {\n\tneed(x)\n", "bool {\n"},
	{"a logical operand is Unknown", "term.go", "need(lv)\n", ""},
	{"false or a failure is false", "unknown.go", "\tcase oFalse:\n\t\treturn Bool(false)", "\tcase oFalse, oFalse | oFail:\n\t\treturn Bool(false)"},
	{"a failing left operand is dropped", "unknown.go", "out := l & oFail\n", "out := outcome(0)\n"},
	{"! keeps true", "unknown.go", "if in&oTrue != 0 {\n\t\t\tout |= oFalse", "if in&oTrue != 0 {\n\t\t\tout |= oTrue"},
	{"an unknown predicate holds", "unknown.go", "if unknown {\n\t\treturn false, false\n\t}\n\treturn Holds", "if unknown {\n\t\treturn true, true\n\t}\n\treturn Holds"},
	{"an unknown value is taken as known", "unknown.go", "return v, \"\", v.Kind == KUnknown", "return v, \"\", false"},
	{"Eval turns Unknown into nil", "term.go", "v, err = Unknown, \"\"", "v, err = Nil, \"\""},
	{"a partial node has a value", "value.go", "\tcase UnknownShape:\n\t\tneed(Unknown)\n", ""},
	{"a partial node has no captures", "term.go", "if x.Shape == UnknownShape && x.Caps == nil {\n\t\t\t\tneed(Unknown)\n\t\t\t}\n", ""},
	{"a partial node has no elements", "term.go", "\t\tcase UnknownShape:\n\t\t\tneed(Unknown)\n\t\t}\n\t}\n\tfail(\"%s: expected a list", "\t\t}\n\t}\n\tfail(\"%s: expected a list"},
	{"a partial node has a length", "term.go", "if x.Shape == UnknownShape {\n\t\t\t\tneed(Unknown)\n\t\t\t}\n", ""},
	{"a partial node has no type", "term.go", "if x.Kind == KNode && x.Shape == UnknownShape {\n\t\t\tneed(Unknown) // the type of the node is unknown\n\t\t}\n", ""},
	{"a cast ignores aliases", "term.go", "if a, ok := e.Aliases[t.Name]; ok {\n\t\t\treturn e.admits(a, x)\n\t\t}\n", ""},
	{"a constructor keeps the alias", "term.go", "x.Type = e.typeName(t.Type)", "x.Type = t.Type"},
}

// errorMutants each break one way in which the cast and the list built-ins treat an error value.
var errorMutants = []struct{ name, file, old, new string }{
	{"error admits no value", "term.go", "return x.Kind == KError\n", "return false\n"},
	{"error admits every node", "term.go", "return x.Kind == KError\n", "return x.Kind == KError || x.Kind == KNode\n"},
	{"missing admits a skipped range", "term.go", "return x.Kind == KError && x.Missing\n", "return x.Kind == KError\n"},
	{"missing admits no token", "term.go", "return x.Kind == KError && x.Missing\n", "return false\n"},
	{"a type name reads the node of the error", "value.go", "\tcase KError:\n\t\tif v.Missing {\n\t\t\treturn \"missing\"\n\t\t}\n\t\treturn \"error\"\n", "\tcase KError:\n\t\treturn \"error\"\n"},
	{"an error value is not a list", "term.go", "case KNil, KError: // an error value counts as an empty list", "case KNil:"},
}

// TestUnknownMutants is the proof that TestUnknownNeverLeaks and TestUnknownResults, with
// TestPartialNodeNeverLeaks and TestPartialNodeResults for nodes of UnknownShape and aliases, hold the
// handling of Unknown: each mutant, built over the package's sources with -overlay, fails one of them.
func TestUnknownMutants(t *testing.T) {
	runMutants(t, unknownMutants, "^Test(Unknown|PartialNode)(NeverLeaks|Results)$", "--- FAIL: TestUnknown", "--- FAIL: TestPartialNode")
}

// TestErrorValueMutants is the proof that TestErrorValueCast and TestErrorValueTerms hold the types that
// admit an error value and the terms that take one: each mutant fails one of them.
func TestErrorValueMutants(t *testing.T) {
	runMutants(t, errorMutants, "^TestErrorValue(Cast|Terms)$", "--- FAIL: TestErrorValue")
}

// runMutants builds each mutant over the package's sources with -overlay, runs the tests that match run, and
// requires that they fail with a line that contains one of fails.
func runMutants(t *testing.T, mutants []struct{ name, file, old, new string }, run string, fails ...string) {
	if testing.Short() {
		t.Skip("runs the tests on mutated sources")
	}
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range mutants {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(dir, m.file)
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if n := strings.Count(string(src), m.old); n != 1 {
				t.Fatalf("%s has %d copies of %q", m.file, n, m.old)
			}
			tmp := t.TempDir()
			mutated := filepath.Join(tmp, m.file)
			if err := os.WriteFile(mutated, []byte(strings.Replace(string(src), m.old, m.new, 1)), 0o644); err != nil {
				t.Fatal(err)
			}
			overlay, _ := json.Marshal(map[string]any{"Replace": map[string]string{path: mutated}})
			ov := filepath.Join(tmp, "overlay.json")
			if err := os.WriteFile(ov, overlay, 0o644); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("go", "test", "-count=1", "-overlay", ov, "-run", run, ".")
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("the tests pass with the mutant:\n%s", out)
			}
			failed := false
			for _, f := range fails {
				failed = failed || strings.Contains(string(out), f)
			}
			if !failed {
				t.Fatalf("the mutant fails none of %q:\n%s", fails, out)
			}
		})
	}
}
