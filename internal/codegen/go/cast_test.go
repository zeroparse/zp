package gogen_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

func TestCastGeneratedGoParity(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testCastGeneratedGoParity)
}

func testCastGeneratedGoParity(t *testing.T) {
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	codegentest.GeneratedParsersCorpus(t, goBin, codegentest.CastCases(), codegen.GenOptions{})
	codegentest.GeneratedParsersCorpus(t, goBin, codegentest.CastCases(), codegen.GenOptions{SourceInfo: true})
	codegentest.GeneratedTypesCorpus(t, goBin, codegentest.CastCases(), codegen.GenOptions{ConvertTypes: true})
}
