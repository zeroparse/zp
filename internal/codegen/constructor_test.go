package codegen_test

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/gentest"
)

func TestGeneratedNodeConstructors(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testGeneratedNodeConstructors)
}

func testGeneratedNodeConstructors(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	cases := gentest.ConstructorCases()
	gentest.GeneratedParsersCorpus(t, goBin, cases, codegen.GenOptions{})
	gentest.GeneratedTypesCorpus(t, goBin, cases, codegen.GenOptions{ConvertTypes: true}, 4)
	if node, err := exec.LookPath("node"); err == nil {
		gentest.TSParsersCorpus(t, node, cases, codegen.GenOptions{})
	} else {
		t.Error("Node is required for constructor parity")
	}
}
