package fmt

import (
	"context"
	"testing"
)

// formatTestContext cancels itself at its cancelAt-th call of Err.
type formatTestContext struct {
	context.Context
	cancel          context.CancelFunc
	calls, cancelAt int
}

func (c *formatTestContext) Err() error {
	c.calls++
	if c.calls == c.cancelAt {
		c.cancel()
	}
	return c.Context.Err()
}

func formatTestCancelAt(t *testing.T, cancelAt int) *formatTestContext {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &formatTestContext{Context: ctx, cancel: cancel, cancelAt: cancelAt}
}

// formatPanicContext panics with marker at its second call of Err.
type formatPanicContext struct {
	context.Context
	marker any
	calls  int
}

func (c *formatPanicContext) Err() error {
	c.calls++
	if c.calls == 2 {
		panic(c.marker)
	}
	return nil
}
