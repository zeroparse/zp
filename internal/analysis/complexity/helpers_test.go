package complexity

import (
	"context"
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/testing/compiletest"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// compileStatic compiles g as the engine does, without building matchers.
func compileStatic(g *ast.Grammar) (*compile.Program, error) {
	return compiletest.Compile(g, compile.Options{})
}

// skipOf returns the skip expression of e if e is an expression with #recover, or nil.
func skipOf(e ast.Expr) ast.Expr {
	switch e := e.(type) {
	case *ast.Recovery:
		return e.Skip
	case *ast.Attributed:
		return compile.RecoverSkip(e)
	}
	return nil
}

// pollContext cancels at a deterministic poll, without timing or global hooks.
type pollContext struct {
	context.Context
	cancel context.CancelFunc
	left   int
}

func (c *pollContext) Err() error {
	c.left--
	if c.left == 0 {
		c.cancel()
	}
	return c.Context.Err()
}

// cancelledAt returns a context that is cancelled at its polls-th check.
func cancelledAt(t *testing.T, polls int) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &pollContext{Context: ctx, cancel: cancel, left: polls}
}
