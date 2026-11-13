package gogen_test

import (
	"os/exec"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/gentest"
)

func TestGeneratedSharingLocalLayout(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testGeneratedSharingLocalLayout)
}

func testGeneratedSharingLocalLayout(t *testing.T) {
	c := gentest.TypedLocalLayoutCases[0]
	if testing.Short() {
		return
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	gentest.GeneratedParsersCorpus(t, goBin, []gentest.Case{c}, codegen.GenOptions{})
	gentest.GeneratedTypesCorpus(t, goBin, gentest.TypedLocalLayoutCases[:5], codegen.GenOptions{NoOpt: []string{"method-sharing", "literal-sharing"}})
}
