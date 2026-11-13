//go:build zp_long

package gengo_test

import (
	"testing"

	"zeroparse.dev/zp/internal/gentest"
)

// Every combination of entry points over the grammars whose typed entry points convert Nodes.
func TestGeneratedEntryPointCombinationsConverted(t *testing.T) {
	t.Parallel()
	testEntryCombinations(t, gentest.TypedOnlyConverted)
}
