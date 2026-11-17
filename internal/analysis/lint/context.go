package lint

import (
	"context"

	"zeroparse.dev/zp/internal/frontend"
	"zeroparse.dev/zp/internal/frontend/astfacts"
	"zeroparse.dev/zp/internal/syntax/fmt"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// runContext lints g, which the front end checked to the result res, from start.
// Cancellation discards all findings, including those accumulated before the
// stopping point. Without an active control nothing polls. Polling covers lint-owned
// analysis, check and suppression loops, recursive analyses and result sorting.
// Shared success proofs, metadata walks, character-range work and diagnostic
// formatting also poll. Other individual operations retain unpolled spans; no
// wall-clock bound is promised.
func runContext(ctx context.Context, res *frontend.Result, g *ast.Grammar, start string, opts options) (findings []Finding, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if ctx.Done() == nil {
		opts.values = ctx
		return run(res, g, start, opts, nil), nil
	}
	defer func() {
		if failure := recover(); failure != nil {
			if cancelled, ok := failure.(lintCancellation); ok {
				findings, err = nil, cancelled.err
			} else {
				panic(failure)
			}
		}
	}()
	return run(res, g, start, opts, &lintControl{ctx: ctx}), nil
}

// Recursive proof routines unwind only to runContext and Rewrites. A control belongs to one
// lint invocation; no global state, background workers or abandoned jobs exist.
type lintCancellation struct{ err error }

type lintControl struct {
	ctx   context.Context
	steps uint32
}

func (c *lintControl) check() {
	if c != nil {
		if err := c.ctx.Err(); err != nil {
			panic(lintCancellation{err})
		}
	}
}

func (c *lintControl) poll() {
	if c == nil {
		return
	}
	c.steps++
	if c.steps%256 == 0 {
		c.check()
	}
}

func (c *lintControl) rules(g *ast.Grammar) []*ast.RuleDef {
	if c == nil {
		return g.Rules()
	}
	result, err := g.RulesContext(c.ctx)
	if err != nil {
		panic(lintCancellation{err})
	}
	return result
}

func (c *lintControl) types(g *ast.Grammar) []ast.TypeDecl {
	if c == nil {
		return g.Types()
	}
	result, err := g.TypesContext(c.ctx)
	if err != nil {
		panic(lintCancellation{err})
	}
	return result
}

func (c *lintControl) comments(g *ast.Grammar) []*ast.Comment {
	if c == nil {
		return g.AllComments()
	}
	result, err := g.AllCommentsContext(c.ctx)
	if err != nil {
		panic(lintCancellation{err})
	}
	return result
}

func (c *lintControl) expressionArgs(a *ast.Attribute) []ast.Expr {
	if c == nil {
		return a.ExpressionArgs()
	}
	return astfacts.ExpressionArgsWithPoll(a, c.poll)
}

func (c *lintControl) formatExpr(e ast.Expr) string {
	if c == nil {
		return fmt.FormatExpr(e)
	}
	result, err := fmt.FormatExprContext(c.ctx, e)
	if err != nil {
		panic(lintCancellation{err})
	}
	return result
}

func (c *lintControl) formatTerm(t ast.ActionExpr) string {
	if c == nil {
		return fmt.FormatActionExpr(astfacts.UngroupTerm(t))
	}
	t = astfacts.UngroupTermWithPoll(t, c.poll)
	result, err := fmt.FormatActionExprContext(c.ctx, t)
	if err != nil {
		panic(lintCancellation{err})
	}
	return result
}
