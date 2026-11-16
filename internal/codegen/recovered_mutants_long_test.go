//go:build zp_long

package codegen_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/testing/codegentest"
)

// recoveredMutants each give a recovered value a wrong field or a wrong range in a struct: the engine's fields
// of a missing token, in MissingLiteral or MissingErrorScope of internal/engine/state/missing_token.go and in
// the literal that the closure builder passes to it, and the ranges of the reference model, in
// internal/ir/exec/ref/term.go.
var recoveredMutants = []struct{ name, file, fn, old, new string }{
	{"expected holds the message", "internal/engine/state/missing_token.go", "MissingLiteral", `{Name: "expected", Value: expected}`, `{Name: "expected", Value: p.RecoveryErrorText(e)}`},
	{"expected is quoted", "internal/engine/state/missing_token.go", "MissingLiteral", `{Name: "expected", Value: expected}`, `{Name: "expected", Value: "\"" + expected + "\""}`},
	{"expected is the spelling in the grammar", "internal/engine/closure/build.go", "node", "n.Lit.Text, want)",
		`fmt.Sprintf("%+q", n.Lit.Text)[1:len(fmt.Sprintf("%+q", n.Lit.Text))-1], want)`},
	{"message holds the literal", "internal/engine/state/missing_token.go", "MissingLiteral", `{Name: "message", Value: p.RecoveryErrorText(e)}`, `{Name: "message", Value: expected}`},
	{"message has no position", "internal/engine/state/missing_token.go", "MissingLiteral", `{Name: "message", Value: p.RecoveryErrorText(e)}`, `{Name: "message", Value: e.Message()}`},
	{"message ignores @error", "internal/engine/state/missing_token.go", "MissingErrorScope", "if p.Expectations && p.Missing != nil && p.Missing.label == 0 {", "if false {"},
	{"a recovered field does not cover", "internal/ir/exec/ref/term.go", "structRange", " && x.Kind != KError {", " {"},
	{"a recovered element has the range of the rule", "internal/ir/exec/ref/term.go", "nodeRange", "case KNode, KError:", "case KNode:"},
	{"a recovered value encodes no message", "internal/ir/exec/ref/term.go", "plain", `"error": v.Fields["message"].S}`, `"error": ""}`},
}

// TestRecoveredValueMutants is the proof that TestTermFixedReference holds the fields expected and message of a
// missing token and the ranges of structs that hold a recovered value, in the engine and in the reference model:
// with a source overlay of each mutant, it fails.
func TestRecoveredValueMutants(t *testing.T) {
	root := codegentest.ModuleRoot()
	for _, m := range recoveredMutants {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(root, filepath.FromSlash(m.file))
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			mutated, ok := codegentest.MutateFunc(string(src), m.fn, m.old, m.new)
			if !ok {
				t.Fatalf("%s has no %s with %q", m.file, m.fn, m.old)
			}
			dir := t.TempDir()
			file := filepath.Join(dir, filepath.Base(path))
			if err := os.WriteFile(file, []byte(mutated), 0o644); err != nil {
				t.Fatal(err)
			}
			overlay, _ := json.Marshal(map[string]map[string]string{"Replace": {path: file}})
			overlayFile := filepath.Join(dir, "overlay.json")
			if err := os.WriteFile(overlayFile, overlay, 0o644); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("go", "test", "-count=1", "-overlay", overlayFile, "-run", "^TestTermFixedReference$", "./internal/codegen")
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "GOFLAGS=")
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("TestTermFixedReference passes with the mutant %q:\n%s", m.name, out)
			}
			if !strings.Contains(string(out), "--- FAIL: TestTermFixedReference") {
				t.Fatalf("the mutant %q fails no reference comparison:\n%s", m.name, out)
			}
		})
	}
}
