package gogen_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/gentest"
)

func TestGeneratedUnsharedParsersMatchEngine(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testGeneratedUnsharedParsersMatchEngine)
}

func testGeneratedUnsharedParsersMatchEngine(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated code")
	}
	opts := codegen.GenOptions{NoOpt: []string{"method-sharing", "literal-sharing"}}
	gentest.GeneratedParsersCorpus(t, filepath.Join(runtime.GOROOT(), "bin", "go"), gentest.Representative(gentest.GenCorpus(t)), opts)
	gentest.GeneratedTypesCorpus(t, filepath.Join(runtime.GOROOT(), "bin", "go"), gentest.Representative(gentest.GenCorpus(t)), opts)
}
