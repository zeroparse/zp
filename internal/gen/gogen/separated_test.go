package gogen_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
)

func TestSeparatedGeneratedGoParity(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testSeparatedGeneratedGoParity)
}

func testSeparatedGeneratedGoParity(t *testing.T) {
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	gentest.GeneratedParsersCorpus(t, goBin, gentest.SeparatedCases(), gen.GenOptions{})
}
