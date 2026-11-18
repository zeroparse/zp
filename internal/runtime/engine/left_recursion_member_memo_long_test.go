//go:build zp_long

package engine

import (
	"fmt"
	"testing"
)

// TestLeftRecursionMemberMemoRandomRecoveringLong checks the seeds of
// TestLeftRecursionMemberMemoRandomRecovering after its first recoveringMemberMemoSeeds, up to
// 3,000 in all.
func TestLeftRecursionMemberMemoRandomRecoveringLong(t *testing.T) {
	t.Parallel()
	const seeds, chunks = 3000, 8
	step := int64(seeds-recoveringMemberMemoSeeds) / chunks
	for i := int64(0); i < chunks; i++ {
		from := recoveringMemberMemoSeeds + i*step
		to := from + step
		if i == chunks-1 {
			to = seeds
		}
		t.Run(fmt.Sprintf("seeds-%d-%d", from, to), func(t *testing.T) {
			t.Parallel()
			checkRecoveringMemberMemo(t, from, to)
		})
	}
}
