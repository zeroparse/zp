package print_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/testing/codegentest"
)

func TestPredicateBlocksGeneratedGoParity(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testPredicateBlocksGeneratedGoParity)
}

func testPredicateBlocksGeneratedGoParity(t *testing.T) {
	codegentest.GeneratedParsersCorpus(t, filepath.Join(runtime.GOROOT(), "bin", "go"), codegentest.PredicateBlockGeneratedCases(), common.GenOptions{})
}
