package tsgen_test

import (
	"os/exec"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

func TestNodeTypesGeneratedTypeScriptParity(t *testing.T) {
	t.Parallel()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found")
	}
	codegentest.TSParsersCorpus(t, node, codegentest.NodeTypeCases(), codegen.GenOptions{})
}
