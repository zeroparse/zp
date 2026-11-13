package codegen_test

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

// TestGeneratedLenOfNodes checks that generated Go parsers, untyped and typed, and generated
// TypeScript parsers give the engine's len of nodes and strings, in both units.
func TestGeneratedLenOfNodes(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testGeneratedLenOfNodes)
}

func testGeneratedLenOfNodes(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	var cases []codegentest.Case
	for _, c := range codegentest.GenCorpus(t) {
		if c.Name == "len of nodes and strings" {
			cases = append(cases, c)
		}
	}
	if len(cases) != 1 {
		t.Fatalf("%d cases", len(cases))
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	codegentest.GeneratedParsersCorpus(t, goBin, cases, codegen.GenOptions{})
	codegentest.GeneratedTypesCorpus(t, goBin, cases, codegen.GenOptions{ConvertTypes: true}, 1)
	if node, err := exec.LookPath("node"); err == nil {
		codegentest.TSParsersCorpus(t, node, cases, codegen.GenOptions{})
	}
}
