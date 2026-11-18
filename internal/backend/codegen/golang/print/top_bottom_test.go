package print_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/testing/codegentest"
)

func TestTopBottomGeneratedGoParity(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testTopBottomGeneratedGoParity)
}

func testTopBottomGeneratedGoParity(t *testing.T) {
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	codegentest.GeneratedParsersCorpus(t, goBin, codegentest.TopBottomCases(), common.GenOptions{})
	// A field of type _, like one of type any, keeps the reference runtime.
	codegentest.GeneratedTypesCorpus(t, goBin, codegentest.TopBottomCases(), common.GenOptions{ConvertTypes: true}, 2)
}
