package codegen_test

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

func TestGeneratedRuleResultChecks(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testGeneratedRuleResultChecks)
}

func testGeneratedRuleResultChecks(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	cases := codegentest.ResultCheckCases()
	codegentest.GeneratedParsersCorpus(t, goBin, cases, codegen.GenOptions{})
	codegentest.GeneratedTypesCorpus(t, goBin, cases, codegen.GenOptions{ConvertTypes: true})
	if node, err := exec.LookPath("node"); err == nil {
		codegentest.TSParsersCorpus(t, node, cases, codegen.GenOptions{})
	} else {
		t.Error("Node is required for result-check parity")
	}
}
