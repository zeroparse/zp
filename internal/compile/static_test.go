package compile

import (
	"context"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/desugar"
	"zeroparse.dev/zp/internal/starttest"
	"zeroparse.dev/zp/internal/syntax"
)

// The tests of this package check grammars to their static programs (Check), which builds no
// matchers: they check what the compiler decides, which the engine's tests check by running it.

// parseTest parses ZeroParse source for a test whose subject is not start rules: unless the source has
// declaration attributes, every rule becomes a start rule, the first one the default.
func parseTest(src string) (*grammar.Grammar, error) {
	g, err := syntax.Parse(src)
	return starttest.MarkAll(g), err
}

// parseLegacyTest is parseTest for the legacy syntax.
func parseLegacyTest(src string) (*grammar.Grammar, error) {
	g, err := syntax.ParseLegacy(src)
	return starttest.MarkAll(g), err
}

// compileStatic compiles g as the engine does, without building matchers; flags, if not nil,
// replace static analysis and type checking.
func compileStatic(g *grammar.Grammar, opts Options, flags []RuleFlags) (*Program, error) {
	names := desugar.InlineNames{}
	p, err := Check(g, opts, flags, names, nil)
	if err != nil {
		return nil, names.Rewrite(err)
	}
	return p, nil
}

// compilePollContext cancels at a deterministic poll, without timing or global compiler hooks.
type compilePollContext struct {
	context.Context
	cancel context.CancelFunc
	left   int
}

func (c *compilePollContext) Err() error {
	c.left--
	if c.left == 0 {
		c.cancel()
	}
	return c.Context.Err()
}

// compileTestContext returns a context that is cancelled at its polls-th check.
func compileTestContext(t *testing.T, polls int) *compilePollContext {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &compilePollContext{Context: ctx, cancel: cancel, left: polls}
}
