package gogen_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

func TestUnsetFieldsReadZeroGeneratedGoParity(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testUnsetFieldsReadZeroGeneratedGoParity)
}

func testUnsetFieldsReadZeroGeneratedGoParity(t *testing.T) {
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	codegentest.GeneratedParsersCorpus(t, goBin, codegentest.UnsetZeroCases(), codegen.GenOptions{})
	codegentest.GeneratedTypesCorpus(t, goBin, codegentest.UnsetZeroCases(), codegen.GenOptions{ConvertTypes: true}, 0)
}
