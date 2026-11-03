package gentest

import (
	"fmt"
	"runtime"
	"testing"
)

// EachParallel runs f on every item, in parallel subtests of t named chunk-0, chunk-1 and so on,
// and returns when all have finished. The items are dealt round-robin to as many subtests as
// GOMAXPROCS, each of which runs its items one after another, so that a loop over many
// independent cases, each of which takes a while, keeps every processor busy without a subtest per
// case. f may fail t but must not depend on the order of the items, and the state that the items
// share must be safe for concurrent use. The caller reads results that f accumulates after
// EachParallel returns.
func EachParallel[T any](t *testing.T, items []T, f func(t *testing.T, item T)) {
	t.Helper()
	n := min(runtime.GOMAXPROCS(0), len(items))
	t.Run("all", func(t *testing.T) {
		for c := range n {
			t.Run(fmt.Sprintf("chunk-%d", c), func(t *testing.T) {
				t.Parallel()
				for i := c; i < len(items); i += n {
					f(t, items[i])
				}
			})
		}
	})
}
