package complexity

import (
	"context"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
)

// complexitySinkKey is the context key of the function that WithComplexitySink installs.
type complexitySinkKey struct{}

// WithComplexitySink returns a context under which ClassifyComplexity passes each classification
// it completes to sink. A caller that runs the lint checks with the classification requested
// gets in this way the classification behind the super-linear findings, without classifying the
// grammar again.
func WithComplexitySink(ctx context.Context, sink func(*Complexity)) context.Context {
	return context.WithValue(ctx, complexitySinkKey{}, sink)
}

// ClassifyComplexity is ClassifyComplexityControlled under ctx: it returns ctx's error, and no
// classification, if ctx ends. It passes the classification to the sink of ctx, if any (see
// WithComplexitySink).
func ClassifyComplexity(ctx context.Context, g *grammar.Grammar, start string) (*Complexity, error) {
	c, err := classifyComplexity(ctx, g, start)
	if sink, ok := ctx.Value(complexitySinkKey{}).(func(*Complexity)); ok && err == nil {
		sink(c)
	}
	return c, err
}

func classifyComplexity(ctx context.Context, g *grammar.Grammar, start string) (c *Complexity, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if ctx.Done() == nil {
		return ClassifyComplexityControlled(g, start, nil)
	}
	defer func() {
		if failure := recover(); failure != nil {
			if cancelled, ok := failure.(cancel.Cancellation); ok {
				c, err = nil, cancelled.Err
			} else {
				panic(failure)
			}
		}
	}()
	return ClassifyComplexityControlled(g, start, &cancel.Control{Ctx: ctx})
}

// GrowingBindings is GrowingBindingsControlled under ctx: it returns the Growing member of
// ClassifyComplexity(ctx, g, start) at a lower cost.
func GrowingBindings(ctx context.Context, g *grammar.Grammar, start string) (b []GrowingBinding, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if ctx.Done() == nil {
		return GrowingBindingsControlled(g, start, nil)
	}
	defer func() {
		if failure := recover(); failure != nil {
			if cancelled, ok := failure.(cancel.Cancellation); ok {
				b, err = nil, cancelled.Err
			} else {
				panic(failure)
			}
		}
	}()
	return GrowingBindingsControlled(g, start, &cancel.Control{Ctx: ctx})
}
