package codegen_test

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/gentest"
)

func TestGeneratedConstructedFieldChecks(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testGeneratedConstructedFieldChecks)
}

func testGeneratedConstructedFieldChecks(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	cases := gentest.FieldCheckCases()
	gentest.GeneratedParsersCorpus(t, goBin, cases, codegen.GenOptions{})
	gentest.GeneratedTypesCorpus(t, goBin, cases, codegen.GenOptions{ConvertTypes: true})
	if node, err := exec.LookPath("node"); err == nil {
		gentest.TSParsersCorpus(t, node, cases, codegen.GenOptions{})
	} else {
		t.Error("Node is required for field-check parity")
	}
}
