package gogen_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

func TestNodeTypesGeneratedGoParity(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testNodeTypesGeneratedGoParity)
}

func testNodeTypesGeneratedGoParity(t *testing.T) {
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	codegentest.GeneratedParsersCorpus(t, goBin, codegentest.NodeTypeCases(), codegen.GenOptions{})
	codegentest.GeneratedTypesCorpus(t, goBin, codegentest.NodeTypeCases(), codegen.GenOptions{})
	codegentest.GeneratedTypesCorpus(t, goBin, codegentest.NodeTypeCases(), codegen.GenOptions{ConvertTypes: true})
}
