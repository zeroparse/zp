package gogen_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/gentest"
)

func TestCompareGeneratedGoParity(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testCompareGeneratedGoParity)
}

func testCompareGeneratedGoParity(t *testing.T) {
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	gentest.GeneratedParsersCorpus(t, goBin, gentest.CompareCases(), codegen.GenOptions{})
	gentest.GeneratedParsersCorpus(t, goBin, gentest.CompareCases(), codegen.GenOptions{SourceInfo: true})
	gentest.GeneratedTypesCorpus(t, goBin, gentest.CompareCases(), codegen.GenOptions{ConvertTypes: true})
}
