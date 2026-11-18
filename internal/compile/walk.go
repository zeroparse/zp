package compile

import (
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/frontend/astfacts"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// The walks of the grammar's expressions and terms that the checks of the compiler use. The rule
// facts are not computed here: they are those of the Core IR module of the rules (RuleFacts).

// AllCalls returns every rule called in the expression.
func AllCalls(e ast.Expr, acc []string) []string {
	return AllCallsControlled(e, acc, nil)
}

// AllCallsControlled is AllCalls polling control for cancellation.
func AllCallsControlled(e ast.Expr, acc []string, control *cancel.Control) []string {
	walkExprControlled(e, func(x ast.Expr) {
		control.Poll()
		if r, ok := x.(*ast.Ref); ok {
			acc = append(acc, r.Name)
		}
	}, control)
	return acc
}

// walkExprControlled is astfacts.WalkExprWithPoll polling control.
func walkExprControlled(e ast.Expr, f func(ast.Expr), control *cancel.Control) {
	if control == nil {
		astfacts.WalkExpr(e, f)
		return
	}
	astfacts.WalkExprWithPoll(e, f, control.Poll)
}

// WalkTerm is astfacts.WalkTerm.
func WalkTerm(t ast.ActionExpr, f func(ast.ActionExpr)) { astfacts.WalkTerm(t, f) }

// WalkExpr is astfacts.WalkExpr.
func WalkExpr(e ast.Expr, f func(ast.Expr)) { astfacts.WalkExpr(e, f) }

// WalkRuleTerms visits every term in a rule's action and predicates, including Operators Rule
// operand and operator actions.
func WalkRuleTerms(r *ast.RuleDef, f func(ast.ActionExpr)) { walkRuleTermsControlled(r, f, nil) }

// walkRuleTermsControlled is WalkRuleTerms polling control for cancellation.
func walkRuleTermsControlled(r *ast.RuleDef, f func(ast.ActionExpr), control *cancel.Control) {
	control.Poll()
	WalkTerm(r.Action, f)
	walkExprControlled(r.Expr, func(e ast.Expr) {
		switch e := e.(type) {
		case *ast.Predicate:
			WalkTerm(e.Action, f)
		case *ast.OperatorsRule:
			for _, o := range e.Operands {
				control.Poll()
				WalkTerm(o.Action, f)
			}
			for _, l := range e.Levels {
				control.Poll()
				for _, op := range l.Operators {
					control.Poll()
					WalkTerm(op.Action, f)
				}
			}
		}
	}, control)
}

// valueRecovery reports whether e has a #recover outside lookaheads and Operators Rule skips, and
// returns the rules that it calls outside them (noRecoveringCaptures).
func valueRecovery(e ast.Expr, control *cancel.Control) (bool, []string) {
	silent := map[ast.Expr]bool{}
	mark := func(x ast.Expr) {
		walkExprControlled(x, func(y ast.Expr) { silent[y] = true }, control)
	}
	walkExprControlled(e, func(x ast.Expr) {
		switch x := x.(type) {
		case *ast.And:
			mark(x.Expr)
		case *ast.Not:
			mark(x.Expr)
		case *ast.OperatorsRule:
			mark(x.Skip)
		}
	}, control)
	found := false
	var calls []string
	walkExprControlled(e, func(x ast.Expr) {
		if silent[x] {
			return
		}
		switch x := x.(type) {
		case *ast.Ref:
			calls = append(calls, x.Name)
		case *ast.Recovery:
			found = true
		case *ast.Attributed:
			found = found || hasRecover(x.Attrs)
		}
	}, control)
	return found, calls
}

// HasPragma reports whether e carries the pragma or attribute name.
func HasPragma(e *ast.Attributed, name string) bool {
	for _, a := range e.Attrs {
		if a.Name == name {
			return true
		}
	}
	return false
}

// hasRecover reports whether attrs has #recover.
func hasRecover(attrs []*ast.Attribute) bool {
	for _, a := range attrs {
		if a.Name == "recover" {
			return true
		}
	}
	return false
}

// RecoverSkip returns the skip expression of a #recover on e, if it has one.
func RecoverSkip(e *ast.Attributed) ast.Expr {
	for _, a := range e.Attrs {
		if a.Name == "recover" {
			if skip, ok := a.Arg("skip"); ok {
				return skip
			}
		}
	}
	return nil
}
