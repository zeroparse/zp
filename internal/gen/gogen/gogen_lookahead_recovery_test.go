package gogen_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
)

// TestGeneratedLookaheadRecoveryMatchesEngine compares the results and errors of node and typed
// parsers that recover errors inside lookaheads and Operators Rule skips with the engine's.
func TestGeneratedLookaheadRecoveryMatchesEngine(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds generated code")
	}
	gentest.PinOrdinary(t)
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	gentest.GeneratedParsersCorpus(t, goBin, gentest.LookaheadRecoveryCases(), gen.GenOptions{})
	gentest.GeneratedTypesCorpus(t, goBin, gentest.LookaheadRecoveryCases(), gen.GenOptions{})
}
