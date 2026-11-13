package gogen_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/gentest"
)

func TestValueResultsGeneratedGoParity(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testValueResultsGeneratedGoParity)
}

func testValueResultsGeneratedGoParity(t *testing.T) {
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	gentest.GeneratedParsersCorpus(t, goBin, gentest.ValueResultCases(), codegen.GenOptions{})
	gentest.GeneratedTypesCorpus(t, goBin, gentest.ValueResultCases(), codegen.GenOptions{ConvertTypes: true})
}
