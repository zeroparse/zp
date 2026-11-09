package grammaranalysis

import "zeroparse.dev/zp/grammar"

// WalkExpr visits the expression and its subexpressions in preorder, including the expression
// arguments of attributes and the operand and operator expressions of an Operators Rule.
func WalkExpr(e grammar.Expr, f func(grammar.Expr)) {
	WalkExprWithPoll(e, f, nil)
}

// WalkExprWithPoll is WalkExpr that also calls poll before each visited expression and in the
// loops over attributes and Operators Rule levels; a nil poll is never called. The callback may
// panic to interrupt the walk.
func WalkExprWithPoll(e grammar.Expr, f func(grammar.Expr), poll func()) {
	if poll != nil {
		poll()
	}
	if e == nil {
		return
	}
	f(e)
	switch e := e.(type) {
	case *grammar.Group:
		WalkExprWithPoll(e.Expr, f, poll)
	case *grammar.Seq:
		for _, it := range e.Items {
			WalkExprWithPoll(it, f, poll)
		}
	case *grammar.Choice:
		for _, alt := range e.Alts {
			WalkExprWithPoll(alt, f, poll)
		}
	case *grammar.Repeat:
		WalkExprWithPoll(e.Expr, f, poll)
	case *grammar.InlineAction:
		WalkExprWithPoll(e.Expr, f, poll)
	case *grammar.Optional:
		WalkExprWithPoll(e.Expr, f, poll)
	case *grammar.And:
		WalkExprWithPoll(e.Expr, f, poll)
	case *grammar.Not:
		WalkExprWithPoll(e.Expr, f, poll)
	case *grammar.Atomic:
		WalkExprWithPoll(e.Expr, f, poll)
	case *grammar.Stream:
		WalkExprWithPoll(e.Expr, f, poll)
	case *grammar.Recovery:
		WalkExprWithPoll(e.Expr, f, poll)
		WalkExprWithPoll(e.Skip, f, poll)
	case *grammar.Discard:
		WalkExprWithPoll(e.Expr, f, poll)
	case *grammar.Capture:
		WalkExprWithPoll(e.Expr, f, poll)
	case *grammar.Attributed:
		WalkExprWithPoll(e.Expr, f, poll)
		for _, at := range e.Attrs {
			if poll != nil {
				poll()
			}
			for _, arg := range ExpressionArgsWithPoll(at, poll) {
				WalkExprWithPoll(arg, f, poll)
			}
		}
	case *grammar.OperatorsRule:
		WalkExprWithPoll(e.Skip, f, poll)
		for _, o := range e.Operands {
			WalkExprWithPoll(o.Expr, f, poll)
		}
		for _, l := range e.Levels {
			if poll != nil {
				poll()
			}
			for _, op := range l.Operators {
				WalkExprWithPoll(op.Expr, f, poll)
			}
		}
	}
}
