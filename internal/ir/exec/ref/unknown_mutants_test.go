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
}

// TestUnknownMutants is the proof that TestUnknownNeverLeaks and TestUnknownResults hold the handling
// of Unknown: each mutant, built over the package's sources with -overlay, fails one of them.
func TestUnknownMutants(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the tests on mutated sources")
	}
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range unknownMutants {
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
			cmd := exec.Command("go", "test", "-count=1", "-overlay", ov, "-run", "^TestUnknown(NeverLeaks|Results)$", ".")
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("the tests pass with the mutant:\n%s", out)
			}
			if !strings.Contains(string(out), "--- FAIL: TestUnknown") {
				t.Fatalf("the mutant fails no Unknown test:\n%s", out)
			}
		})
	}
}
