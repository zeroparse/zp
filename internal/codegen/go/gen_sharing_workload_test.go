package gogen_test

import (
	"os/exec"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

func TestGeneratedSharingLocalLayout(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testGeneratedSharingLocalLayout)
}

func testGeneratedSharingLocalLayout(t *testing.T) {
	c := codegentest.TypedLocalLayoutCases[0]
	if testing.Short() {
		return
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	codegentest.GeneratedParsersCorpus(t, goBin, []codegentest.Case{c}, codegen.GenOptions{})
	codegentest.GeneratedTypesCorpus(t, goBin, codegentest.TypedLocalLayoutCases[:5], codegen.GenOptions{NoOpt: []string{"method-sharing", "literal-sharing"}})
}
