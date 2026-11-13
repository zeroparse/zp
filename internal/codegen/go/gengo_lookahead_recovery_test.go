package gogen_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

// TestGeneratedLookaheadRecoveryMatchesEngine compares the results and errors of node and typed
// parsers that recover errors inside lookaheads and Operators Rule skips with the engine's.
func TestGeneratedLookaheadRecoveryMatchesEngine(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds generated code")
	}
	codegentest.PinOrdinary(t)
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	codegentest.GeneratedParsersCorpus(t, goBin, codegentest.LookaheadRecoveryCases(), codegen.GenOptions{})
	codegentest.GeneratedTypesCorpus(t, goBin, codegentest.LookaheadRecoveryCases(), codegen.GenOptions{})
}
