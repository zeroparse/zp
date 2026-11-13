package gogen_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/gentest"
)

func TestUnsetFieldsReadZeroGeneratedGoParity(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testUnsetFieldsReadZeroGeneratedGoParity)
}

func testUnsetFieldsReadZeroGeneratedGoParity(t *testing.T) {
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	gentest.GeneratedParsersCorpus(t, goBin, gentest.UnsetZeroCases(), codegen.GenOptions{})
	gentest.GeneratedTypesCorpus(t, goBin, gentest.UnsetZeroCases(), codegen.GenOptions{ConvertTypes: true}, 0)
}
