package engine

import (
	"os"
	"runtime/debug"
)

// The tests allocate many short-lived values over a small live heap, so with the default
// collection target the collector runs continuously and takes about half of the processor time of
// a test process. A larger target, bounded by a soft memory limit, removes that cost; GOGC and
// GOMEMLIMIT in the environment still decide when they are set.
func init() {
	if os.Getenv("GOGC") == "" {
		debug.SetGCPercent(400)
	}
	if os.Getenv("GOMEMLIMIT") == "" {
		debug.SetMemoryLimit(4 << 30)
	}
}
