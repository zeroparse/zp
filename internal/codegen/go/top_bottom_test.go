package gogen_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

func TestTopBottomGeneratedGoParity(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testTopBottomGeneratedGoParity)
}

func testTopBottomGeneratedGoParity(t *testing.T) {
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	codegentest.GeneratedParsersCorpus(t, goBin, codegentest.TopBottomCases(), codegen.GenOptions{})
	// A field of type _, like one of type any, keeps the reference runtime.
	codegentest.GeneratedTypesCorpus(t, goBin, codegentest.TopBottomCases(), codegen.GenOptions{ConvertTypes: true}, 2)
}
