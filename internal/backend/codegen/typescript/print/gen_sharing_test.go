package print_test

import (
	"os/exec"
	"testing"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/testing/codegentest"
)

func TestGeneratedUnsharedTSParsersMatchEngine(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("runs generated TypeScript")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found")
	}
	codegentest.TSParsersCorpus(t, node, codegentest.Representative(codegentest.GenCorpus(t)), common.GenOptions{NoOpt: []string{"method-sharing", "literal-sharing"}})
}
