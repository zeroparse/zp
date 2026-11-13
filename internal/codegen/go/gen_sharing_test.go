package gogen_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

func TestGeneratedUnsharedParsersMatchEngine(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testGeneratedUnsharedParsersMatchEngine)
}

func testGeneratedUnsharedParsersMatchEngine(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated code")
	}
	opts := codegen.GenOptions{NoOpt: []string{"method-sharing", "literal-sharing"}}
	codegentest.GeneratedParsersCorpus(t, filepath.Join(runtime.GOROOT(), "bin", "go"), codegentest.Representative(codegentest.GenCorpus(t)), opts)
	codegentest.GeneratedTypesCorpus(t, filepath.Join(runtime.GOROOT(), "bin", "go"), codegentest.Representative(codegentest.GenCorpus(t)), opts)
}
