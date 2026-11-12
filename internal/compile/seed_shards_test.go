package compile_test

import (
	"fmt"
	"runtime"
	"testing"
)

// eachSeed calls f once for every seed below seeds. The seeds are independent, so parallel
// shards run them, seed i in shard i mod GOMAXPROCS; eachSeed returns when every shard is done.
func eachSeed(t *testing.T, seeds uint64, f func(t *testing.T, seed uint64)) {
	t.Helper()
	shards := min(uint64(runtime.GOMAXPROCS(0)), max(seeds, 1))
	t.Run("seeds", func(t *testing.T) {
		for shard := range shards {
			t.Run(fmt.Sprintf("shard-%d", shard), func(t *testing.T) {
				t.Parallel()
				for seed := shard; seed < seeds; seed += shards {
					f(t, seed)
				}
			})
		}
	})
}
