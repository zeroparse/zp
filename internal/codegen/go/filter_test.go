package gogen_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/gentest"
)

func TestFilterGeneratedGoParity(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testFilterGeneratedGoParity)
}

func testFilterGeneratedGoParity(t *testing.T) {
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	gentest.GeneratedParsersCorpus(t, goBin, gentest.FilterCases(), codegen.GenOptions{})
	gentest.GeneratedTypesCorpus(t, goBin, gentest.FilterCases(), codegen.GenOptions{})
}
