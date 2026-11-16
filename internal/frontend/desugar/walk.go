package desugar

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/frontend/astfacts"
)

// walkExpr is astfacts.WalkExprWithPoll polling control; a nil control never polls.
func walkExpr(e grammar.Expr, f func(grammar.Expr), control *cancel.Control) {
	if control == nil {
		astfacts.WalkExpr(e, f)
		return
	}
	astfacts.WalkExprWithPoll(e, f, control.Poll)
}
