package print_test

import (
	"os/exec"
	"testing"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/testing/codegentest"
)

func TestCompareGeneratedTypeScriptParity(t *testing.T) {
	t.Parallel()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found")
	}
	codegentest.TSParsersCorpus(t, node, codegentest.CompareCases(), common.GenOptions{})
	codegentest.TSParsersCorpus(t, node, codegentest.CompareCases(), common.GenOptions{SourceInfo: true})
}
