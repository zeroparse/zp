package gogen_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/gentest"
)

func TestNodeTypesGeneratedGoParity(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testNodeTypesGeneratedGoParity)
}

func testNodeTypesGeneratedGoParity(t *testing.T) {
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	gentest.GeneratedParsersCorpus(t, goBin, gentest.NodeTypeCases(), codegen.GenOptions{})
	gentest.GeneratedTypesCorpus(t, goBin, gentest.NodeTypeCases(), codegen.GenOptions{})
	gentest.GeneratedTypesCorpus(t, goBin, gentest.NodeTypeCases(), codegen.GenOptions{ConvertTypes: true})
}
