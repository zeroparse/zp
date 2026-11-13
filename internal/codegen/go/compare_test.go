package gogen_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

func TestCompareGeneratedGoParity(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testCompareGeneratedGoParity)
}

func testCompareGeneratedGoParity(t *testing.T) {
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	codegentest.GeneratedParsersCorpus(t, goBin, codegentest.CompareCases(), codegen.GenOptions{})
	codegentest.GeneratedParsersCorpus(t, goBin, codegentest.CompareCases(), codegen.GenOptions{SourceInfo: true})
	codegentest.GeneratedTypesCorpus(t, goBin, codegentest.CompareCases(), codegen.GenOptions{ConvertTypes: true})
}
