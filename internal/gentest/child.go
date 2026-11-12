package gentest

import (
	"context"
	"testing"
	"time"
)

// childMargin is how long before the test binary's deadline ChildContext ends: the binary's own
// timeout panics without running deferred calls or cleanups, so a child still running then
// outlives it.
const childMargin = 30 * time.Second

// ChildContext returns the context of a child process of t, for exec.CommandContext: it ends limit
// from now (never if limit <= 0) or childMargin before the deadline of the test binary, whichever
// comes first. Tests that run programs which may not end, such as those with a mutated runtime,
// bound them by limit, and a child is killed before the binary's timeout leaves it running. The
// child must be the process that does the work, not a launcher such as go run whose program
// would survive it.
func ChildContext(t *testing.T, limit time.Duration) (context.Context, context.CancelFunc) {
	end, ok := t.Deadline()
	if ok {
		end = end.Add(-childMargin)
	}
	if limit > 0 {
		if l := time.Now().Add(limit); !ok || l.Before(end) {
			end, ok = l, true
		}
	}
	if !ok {
		return context.WithCancel(context.Background())
	}
	return context.WithDeadline(context.Background(), end)
}

// ChildWaitDelay is the exec.Cmd.WaitDelay of the children of ChildContext: once the context ends
// and the child is killed, Wait returns after at most this long even if a descendant still holds
// its output open.
const ChildWaitDelay = 10 * time.Second
