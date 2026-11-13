package gogen_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

// TestGeneratedTypedLeaderValues compares the values of typed parsers with the engine's for
// leaders whose values are not nodes, in both position units and on the reference route of
// Node conversion. TestGeneratedTypes covers the cases again within its corpus.
func TestGeneratedTypedLeaderValues(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testGeneratedTypedLeaderValues)
}

func testGeneratedTypedLeaderValues(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated code")
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	if _, err := os.Stat(goBin); err != nil {
		t.Skip("go command not found")
	}
	codegentest.GeneratedTypesCorpus(t, goBin, codegentest.TypedLeaderValueCases, codegen.GenOptions{ConvertTypes: true}, len(codegentest.TypedLeaderValueCases))
}
