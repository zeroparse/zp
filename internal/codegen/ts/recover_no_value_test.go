package tsgen_test

import (
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/gentest"
)

// TestGeneratedTSRecoverNoValue checks that a generated TypeScript parser gives an expression
// without a value under #recover no value, as the engine does, whether it matches or recovers.
// TestRecoverWithoutValueAddsNoElement in internal/engine checks the engine's results.
func TestGeneratedTSRecoverNoValue(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("runs generated code")
	}
	gentest.TSParsersCorpus(t, gentest.RequireNode(t), gentest.RecoverNoValueCases, codegen.GenOptions{})
}
