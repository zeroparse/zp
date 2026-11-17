package astfacts

import "zeroparse.dev/zp/pkg/grammar/ast"

// WalkExpr visits the expression and its subexpressions in preorder, including the expression
// arguments of attributes and the operand and operator expressions of an Operators Rule.
func WalkExpr(e ast.Expr, f func(ast.Expr)) {
	WalkExprWithPoll(e, f, nil)
}

// WalkExprWithPoll is WalkExpr that also calls poll before each visited expression and in the
// loops over attributes and Operators Rule levels; a nil poll is never called. The callback may
// panic to interrupt the walk.
func WalkExprWithPoll(e ast.Expr, f func(ast.Expr), poll func()) {
	if poll != nil {
		poll()
	}
	if e == nil {
		return
	}
	f(e)
	switch e := e.(type) {
	case *ast.Group:
		WalkExprWithPoll(e.Expr, f, poll)
	case *ast.Seq:
		for _, it := range e.Items {
			WalkExprWithPoll(it, f, poll)
		}
	case *ast.Choice:
		for _, alt := range e.Alts {
			WalkExprWithPoll(alt, f, poll)
		}
	case *ast.Repeat:
		WalkExprWithPoll(e.Expr, f, poll)
	case *ast.InlineAction:
		WalkExprWithPoll(e.Expr, f, poll)
	case *ast.Optional:
		WalkExprWithPoll(e.Expr, f, poll)
	case *ast.And:
		WalkExprWithPoll(e.Expr, f, poll)
	case *ast.Not:
		WalkExprWithPoll(e.Expr, f, poll)
	case *ast.Atomic:
		WalkExprWithPoll(e.Expr, f, poll)
	case *ast.Stream:
		WalkExprWithPoll(e.Expr, f, poll)
	case *ast.Recovery:
		WalkExprWithPoll(e.Expr, f, poll)
		WalkExprWithPoll(e.Skip, f, poll)
	case *ast.Discard:
		WalkExprWithPoll(e.Expr, f, poll)
	case *ast.Capture:
		WalkExprWithPoll(e.Expr, f, poll)
	case *ast.Attributed:
		WalkExprWithPoll(e.Expr, f, poll)
		for _, at := range e.Attrs {
			if poll != nil {
				poll()
			}
			for _, arg := range ExpressionArgsWithPoll(at, poll) {
				WalkExprWithPoll(arg, f, poll)
			}
		}
	case *ast.OperatorsRule:
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
