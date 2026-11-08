package engine_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gentest"
)

// engineMutant changes the printed code of the operators of terms in the engine in one way: in the
// compositions of internal/engine/core/term_exec.go, or in the primitives of term_value.go.
type engineMutant struct {
	name, file, fn, old, new string
}

var engineMutants = []engineMutant{
	{"swapped operands of -", "term_exec.go", "termBinary", "any(l.(int) - r.(int))", "any(r.(int) - l.(int))"},
	{"a node is equal to a node", "term_exec.go", "termEqual", "return false, false\n}", "return true, true\n}"},
	{"an int equals a bool", "term_exec.go", "termEqual", "if ka != kb {\n\t\treturn false, false\n\t}", "if ka != kb {\n\t\treturn false, true\n\t}"},
	{"an index may be the length", "term_value.go", "intIndex", "k >= n", "k > n"},
	{"the least index is 1", "term_value.go", "intIndex", "k < 0", "k < 1"},
	{"a division by zero succeeds", "term_exec.go", "termBinary", "if r.(int) == 0 {", "if false {"},
	{"< is <=", "term_exec.go", "termBinary", "intCmp(l, r) < 0", "intCmp(l, r) <= 0"},
	{"strings compare as ints", "term_exec.go", "termBinary", "p.strCmp(l, r) < 0", "p.strCmp(l, r) > 0"},
	{"! of a bool does not negate", "term_exec.go", "termUnary", "return any(!x.(bool)), nil", "return any(x.(bool)), nil"},
	{"a logical operand need not be a bool", "term_exec.go", "termLogic", "if valKind(x) != 3 {", "if false {"},
	{"the controlled concatenation drops the right string", "term_value.go", "strAdd", "for _, s := range [...]string{ls, rs} {", "for _, s := range [...]string{ls} {"},
	{"the controlled equality ignores the content", "term_value.go", "strEq", "return len(ls) == len(rs) && p.CompareText(ls, rs) == 0", "return len(ls) == len(rs)"},
	{"the controlled comparison orders a prefix after its extension", "budget_value.go", "CompareText", "if len(l) < len(r) {\n\t\treturn -1", "if len(l) < len(r) {\n\t\treturn 1"},
	{"a remainder floors", "term_exec.go", "termBinary", `any(l.(int) % r.(int))`, `any(func() int { m := l.(int) % r.(int); if m != 0 && (m < 0) != (r.(int) < 0) { m += r.(int) }; return m }())`},
	{"a division floors", "term_exec.go", "termBinary", `any(l.(int) / r.(int))`, `any(func() int { q := l.(int) / r.(int); if (l.(int)%r.(int) != 0) && ((l.(int) < 0) != (r.(int) < 0)) { q-- }; return q }())`},
	{"a box is not opened", "term_value.go", "unboxed", "return unboxed(value.D(n).Box)", "return v"},
}

// TestPrintedOperatorMutants is the proof that TestEngineValueConformance and TestEngineValueBoxedOperands hold the printed code of the
// operators of terms in the engine: with a source overlay of each mutant, it fails.
func TestPrintedOperatorMutants(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the tests on mutated sources")
	}
	root := gentest.ModuleRoot()
	for _, m := range engineMutants {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(root, "internal", "engine", "core", m.file)
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			mutated, ok := gentest.MutateFunc(string(src), m.fn, m.old, m.new)
			if !ok {
				t.Fatalf("%s has no %s with %q", m.file, m.fn, m.old)
			}
			dir := t.TempDir()
			file := filepath.Join(dir, m.file)
			if err := os.WriteFile(file, []byte(mutated), 0o644); err != nil {
				t.Fatal(err)
			}
			overlay, _ := json.Marshal(map[string]map[string]string{"Replace": {path: file}})
			overlayFile := filepath.Join(dir, "overlay.json")
			if err := os.WriteFile(overlayFile, overlay, 0o644); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("go", "test", "-count=1", "-overlay", overlayFile, "-run", "^TestEngineValue(Conformance|BoxedOperands)$", "./internal/engine/core")
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "GOFLAGS=")
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("the value tests of the engine pass with the mutant %q:\n%s", m.name, out)
			}
			if !strings.Contains(string(out), "--- FAIL: TestEngineValue") {
				t.Fatalf("the mutant %q fails no value test of the engine:\n%s", m.name, out)
			}
		})
	}
}
