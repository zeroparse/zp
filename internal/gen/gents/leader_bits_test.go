package gents

import (
	"testing"

	"zeroparse.dev/zp/internal/ir/exec/ref"
)

// TestLeaderBitsMatchModel checks that the generated TypeScript parsers give a bit in present sets to
// as many leaders of a cycle as the executable model does, which the conformance cases of the
// dependency memo assume.
func TestLeaderBitsMatchModel(t *testing.T) {
	if maxLeaderBits != ref.CtxLeaderBits {
		t.Fatalf("maxLeaderBits = %d, the model gives a bit to %d leaders", maxLeaderBits, ref.CtxLeaderBits)
	}
}
