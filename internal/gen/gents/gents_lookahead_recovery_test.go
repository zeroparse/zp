package gents_test

import (
	"os/exec"
	"testing"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
)

// TestGeneratedTSLookaheadRecoveryMatchesEngine compares the results and errors of TypeScript
// parsers that recover errors inside lookaheads and Operators Rule skips with the engine's.
func TestGeneratedTSLookaheadRecoveryMatchesEngine(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("runs generated TypeScript")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found")
	}
	gentest.TSParsersCorpus(t, node, gentest.LookaheadRecoveryCases(), gen.GenOptions{})
}
