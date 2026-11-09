package desugar

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/grammaranalysis"
)

// walkExpr is grammaranalysis.WalkExprWithPoll polling control; a nil control never polls.
func walkExpr(e grammar.Expr, f func(grammar.Expr), control *cancel.Control) {
	if control == nil {
		grammaranalysis.WalkExpr(e, f)
		return
	}
	grammaranalysis.WalkExprWithPoll(e, f, control.Poll)
}
