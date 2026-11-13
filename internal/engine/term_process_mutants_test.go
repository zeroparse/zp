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

// TestProcessTermTestsKillArithmeticMutants holds the tests that run in the process, with no generated
// parser to build, to the arithmetic of the engine: with a source overlay that floors the remainder
// or the quotient, the reference evaluator disagrees with the engine on a generated term of a default
// build. The few seeds that a build of generated parsers runs did not reach these.
func TestProcessTermTestsKillArithmeticMutants(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the tests on mutated sources")
	}
	root := gentest.ModuleRoot()
	for _, m := range []struct{ name, old, new string }{
		{"the remainder floors", "any(l.(int) % r.(int))", "any(func() int { m := l.(int) % r.(int); if m != 0 && (m < 0) != (r.(int) < 0) { m += r.(int) }; return m }())"},
		{"the quotient floors", "any(l.(int) / r.(int))", "any(func() int { q := l.(int) / r.(int); if (l.(int)%r.(int) != 0) && ((l.(int) < 0) != (r.(int) < 0)) { q-- }; return q }())"},
	} {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(root, "internal", "engine", "core", "term_exec.go")
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			mutated, ok := gentest.MutateFunc(string(src), "termBinary", m.old, m.new)
			if !ok {
				t.Fatalf("no %q in termBinary", m.old)
			}
			dir := t.TempDir()
			file := filepath.Join(dir, "term_exec.go")
			if err := os.WriteFile(file, []byte(mutated), 0o644); err != nil {
				t.Fatal(err)
			}
			overlay, _ := json.Marshal(map[string]map[string]string{"Replace": {path: file}})
			overlayFile := filepath.Join(dir, "overlay.json")
			if err := os.WriteFile(overlayFile, overlay, 0o644); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("go", "test", "-count=1", "-overlay", overlayFile, "-run", "^TestTermReferenceEvaluator$", "./internal/codegen")
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "GOFLAGS=")
			out, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(out), "--- FAIL: TestTermReferenceEvaluator") {
				t.Fatalf("TestTermReferenceEvaluator does not fail with the mutant %q (%v):\n%.2000s", m.name, err, out)
			}
		})
	}
}
