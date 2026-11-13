package gentest

import (
	"context"
	"errors"
	"testing"
	"time"
)

// childMargin is how long before the test binary's deadline ChildContext ends: the binary's own
// timeout panics without running deferred calls or cleanups, so a child still running then
// outlives it.
const childMargin = 30 * time.Second

// ErrChildBound is the cause (context.Cause) of a ChildContext that ended at its limit. A context
// that ended earlier, before the test binary's deadline, has another cause: the child was cut
// short by the test's deadline, not by its bound, and a test that counts a stop at the bound as
// a result must not count that one. See ChildStoppedAtBound.
var ErrChildBound = errors.New("child process ran to its bound")

var errChildDeadline = errors.New("child process stopped before the test binary's deadline")

// ChildStoppedAtBound reports whether ctx, a ChildContext, ended at its limit. It is false for a
// context that has not ended and for one that ended at the test's deadline.
func ChildStoppedAtBound(ctx context.Context) bool {
	return ctx.Err() != nil && errors.Is(context.Cause(ctx), ErrChildBound)
}

// ChildContext returns the context of a child process of t, for exec.CommandContext: it ends limit
// from now (never if limit <= 0) or childMargin before the deadline of the test binary, whichever
// comes first. Tests that run programs which may not end, such as those with a mutated library,
// bound them by limit, and a child is killed before the binary's timeout leaves it running. The
// child must be the process that does the work, not a launcher such as go run whose program
// would survive it.
func ChildContext(t *testing.T, limit time.Duration) (context.Context, context.CancelFunc) {
	end, ok := t.Deadline()
	cause := errChildDeadline
	if ok {
		end = end.Add(-childMargin)
	}
	if limit > 0 {
		if l := time.Now().Add(limit); !ok || l.Before(end) {
			end, ok, cause = l, true, ErrChildBound
		}
	}
	if !ok {
		return context.WithCancel(context.Background())
	}
	return context.WithDeadlineCause(context.Background(), end, cause)
}

// ChildWaitDelay is the exec.Cmd.WaitDelay of the children of ChildContext: once the context ends
// and the child is killed, Wait returns after at most this long even if a descendant still holds
// its output open.
const ChildWaitDelay = 10 * time.Second
