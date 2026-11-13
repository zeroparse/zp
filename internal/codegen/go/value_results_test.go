package gogen_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

func TestValueResultsGeneratedGoParity(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testValueResultsGeneratedGoParity)
}

func testValueResultsGeneratedGoParity(t *testing.T) {
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	codegentest.GeneratedParsersCorpus(t, goBin, codegentest.ValueResultCases(), codegen.GenOptions{})
	codegentest.GeneratedTypesCorpus(t, goBin, codegentest.ValueResultCases(), codegen.GenOptions{ConvertTypes: true})
}
