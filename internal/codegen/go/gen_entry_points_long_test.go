//go:build zp_long

package gogen_test

import (
	"testing"

	"zeroparse.dev/zp/internal/codegentest"
)

// Every combination of entry points over the grammars whose typed entry points convert Nodes.
func TestGeneratedEntryPointCombinationsConverted(t *testing.T) {
	t.Parallel()
	testEntryCombinations(t, codegentest.TypedOnlyConverted)
}
