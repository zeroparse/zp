package codegen_test

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

func TestGeneratedTextStructs(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testGeneratedTextStructs)
}

func testGeneratedTextStructs(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	cases := []codegentest.Case{{Name: "text structs", Src: codegentest.TextStructGrammar, Inputs: []string{"", "ab", "ab 12 3.5", "<x> ! #a # @b @", "ab ?", "12.", "! ab <yz> 7", "[ab]ab", "[ab]cd 1"}}}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	codegentest.GeneratedParsersCorpus(t, goBin, cases, codegen.GenOptions{})
	codegentest.GeneratedTypesCorpus(t, goBin, cases, codegen.GenOptions{ConvertTypes: true}, 1)
	if node, err := exec.LookPath("node"); err == nil {
		codegentest.TSParsersCorpus(t, node, cases, codegen.GenOptions{})
	}
}
