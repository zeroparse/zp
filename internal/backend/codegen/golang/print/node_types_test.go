package print_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/testing/codegentest"
)

func TestNodeTypesGeneratedGoParity(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testNodeTypesGeneratedGoParity)
}

func testNodeTypesGeneratedGoParity(t *testing.T) {
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	codegentest.GeneratedParsersCorpus(t, goBin, codegentest.NodeTypeCases(), common.GenOptions{})
	codegentest.GeneratedTypesCorpus(t, goBin, codegentest.NodeTypeCases(), common.GenOptions{})
	codegentest.GeneratedTypesCorpus(t, goBin, codegentest.NodeTypeCases(), common.GenOptions{ConvertTypes: true})
}
