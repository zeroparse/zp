package bytecode_test

import (
	"os"
	"regexp"
	"strconv"
	"testing"

	"zeroparse.dev/zp/internal/bytecode"
)

// TestSpecInstructionSetVersion checks that spec/bytecode/README.md states the instruction-set version the
// runtime writes and accepts.
func TestSpecInstructionSetVersion(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../spec/bytecode/README.md")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`\| Instruction-set version \| Unsigned integer for this module: (\d+)\.`).FindSubmatch(data)
	if m == nil {
		t.Fatal("spec/bytecode/README.md does not state the instruction-set version of a module")
	}
	if got, _ := strconv.Atoi(string(m[1])); got != bytecode.ISAVersion {
		t.Fatalf("spec/bytecode/README.md states instruction-set version %d; the runtime uses %d", got, bytecode.ISAVersion)
	}
}

// TestDocsInstructionSetVersion checks that the runtime specification of compiled grammars, the
// guide and the release policy give the instruction-set version the runtime writes, as the bytecode
// specification does.
func TestDocsInstructionSetVersion(t *testing.T) {
	t.Parallel()
	for _, d := range []struct{ path, pattern string }{
		{"../../spec/runtime/compiled-grammars.md", `\| Instruction-set version \| (\d+) \|`},
		{"../../docs/guide/compiled-grammars.md", `\| Instruction-set version \| (\d+) \|`},
		{"../../docs/releasing.md", `instruction-set version (\d+)`},
	} {
		data, err := os.ReadFile(d.path)
		if err != nil {
			t.Fatal(err)
		}
		m := regexp.MustCompile(d.pattern).FindSubmatch(data)
		if m == nil {
			t.Errorf("%s does not state the instruction-set version", d.path)
			continue
		}
		if got, _ := strconv.Atoi(string(m[1])); got != bytecode.ISAVersion {
			t.Errorf("%s states instruction-set version %d; the runtime uses %d", d.path, got, bytecode.ISAVersion)
		}
	}
}
