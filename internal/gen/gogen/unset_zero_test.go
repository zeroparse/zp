package gogen_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
)

func TestUnsetFieldsReadZeroGeneratedGoParity(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testUnsetFieldsReadZeroGeneratedGoParity)
}

func testUnsetFieldsReadZeroGeneratedGoParity(t *testing.T) {
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	gentest.GeneratedParsersCorpus(t, goBin, gentest.UnsetZeroCases(), gen.GenOptions{})
	gentest.GeneratedTypesCorpus(t, goBin, gentest.UnsetZeroCases(), gen.GenOptions{ConvertTypes: true}, 0)
}
