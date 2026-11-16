package check

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/frontend/astfacts"
)

// walkExprControlled is astfacts.WalkExprWithPoll polling control.
func walkExprControlled(e grammar.Expr, f func(grammar.Expr), control *cancel.Control) {
	if control == nil {
		astfacts.WalkExpr(e, f)
		return
	}
	astfacts.WalkExprWithPoll(e, f, control.Poll)
}

// expressionArgs returns the expression arguments of the attribute, polling control.
func expressionArgs(control *cancel.Control, a *grammar.Attribute) []grammar.Expr {
	if control == nil {
		return a.ExpressionArgs()
	}
	return astfacts.ExpressionArgsWithPoll(a, control.Poll)
}
