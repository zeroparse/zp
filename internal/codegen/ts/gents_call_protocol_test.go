package tsgen_test

import (
	"os/exec"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

// TestGeneratedTSCallProtocolMatchesEngine compares the results and errors of TypeScript parsers
// whose rules reuse memo entries in lookaheads, attributes, recovery and growth with the engine's.
func TestGeneratedTSCallProtocolMatchesEngine(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("runs generated TypeScript")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found")
	}
	codegentest.TSParsersCorpus(t, node, codegentest.CallProtocolCases(), codegen.GenOptions{})
}
