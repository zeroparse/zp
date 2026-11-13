package tsgen_test

import (
	"os/exec"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

func TestCompareGeneratedTypeScriptParity(t *testing.T) {
	t.Parallel()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found")
	}
	codegentest.TSParsersCorpus(t, node, codegentest.CompareCases(), codegen.GenOptions{})
	codegentest.TSParsersCorpus(t, node, codegentest.CompareCases(), codegen.GenOptions{SourceInfo: true})
}
