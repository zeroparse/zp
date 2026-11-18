package print_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/testing/codegentest"
)

func TestCaptureAccessGeneratedGoParity(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testCaptureAccessGeneratedGoParity)
}

func testCaptureAccessGeneratedGoParity(t *testing.T) {
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	codegentest.GeneratedParsersCorpus(t, goBin, codegentest.CaptureAccessCases(), common.GenOptions{})
	codegentest.GeneratedTypesCorpus(t, goBin, codegentest.CaptureAccessCases(), common.GenOptions{})
}
