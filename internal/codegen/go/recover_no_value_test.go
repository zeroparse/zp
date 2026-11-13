package gogen_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

// TestGeneratedRecoverNoValue checks that a generated Go parser gives an expression without a
// value under #recover no value, as the engine does, whether it matches or recovers.
// TestRecoverWithoutValueAddsNoElement in internal/engine checks the engine's results.
func TestGeneratedRecoverNoValue(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds generated code")
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	codegentest.EachMode(t, func(t *testing.T) {
		codegentest.GeneratedParsersCorpus(t, goBin, codegentest.RecoverNoValueCases, codegen.GenOptions{})
		codegentest.GeneratedTypesCorpus(t, goBin, codegentest.RecoverNoValueCases, codegen.GenOptions{})
	})
}
