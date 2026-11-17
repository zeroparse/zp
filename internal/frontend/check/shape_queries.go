package check

import (
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/frontend/astfacts"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// ElementScopedControlled reports whether the repetition element e gets a capture scope of its own
// per iteration: when it contains captures (HasCapturesControlled), or its predicates read captures, which
// can then only be its own (made inside &) or, an error, those of the enclosing scope. Every
// backend and generator must decide the same way, since the scope decides where captures are
// written.
func ElementScopedControlled(e ast.Expr, control *cancel.Control) bool {
	control.Poll()
	return HasCapturesControlled(e, control) || predicatesReadCaptures(e, control)
}

// ElementScoped reports whether the repetition element e gets a capture scope of its own per
// iteration (ElementScopedControlled).
func ElementScoped(e ast.Expr) bool { return ElementScopedControlled(e, nil) }

// HasCaptures reports whether e contains a capture.
func HasCaptures(e ast.Expr) bool { return HasCapturesControlled(e, nil) }

// predicatesReadCaptures reports whether a predicate in e refers to a capture.
func predicatesReadCaptures(e ast.Expr, control *cancel.Control) bool {
	control.Poll()
	found := false
	walkExprControlled(e, func(x ast.Expr) {
		if p, ok := x.(*ast.Predicate); ok {
			astfacts.WalkTerm(p.Term, func(t ast.Term) {
				control.Poll()
				if _, ok := t.(*ast.CaptureRef); ok {
					found = true
				}
			})
		}
	}, control)
	return found
}

// HasCapturesControlled reports whether the expression contains captures (excluding nested
// repetitions), also inside positive lookaheads, polling control.
func HasCapturesControlled(e ast.Expr, control *cancel.Control) bool {
	control.Poll()
	switch e := e.(type) {
	case *ast.Capture:
		return true
	case *ast.Seq:
		for _, it := range e.Items {
			control.Poll()
			if HasCapturesControlled(it, control) {
				return true
			}
		}
	case *ast.Choice:
		for _, alt := range e.Alts {
			control.Poll()
			if HasCapturesControlled(alt, control) {
				return true
			}
		}
	case *ast.Optional:
		return HasCapturesControlled(e.Expr, control)
	case *ast.Attributed:
		return HasCapturesControlled(e.Expr, control)
	case *ast.Group:
		return HasCapturesControlled(e.Expr, control)
	case *ast.And:
		// The captures of a positive lookahead are kept beside the element's value.
		return HasCapturesControlled(e.Expr, control)
	}
	return false
}

// IsStream reports whether the expression is an expression with the #stream pragma.
func IsStream(e ast.Expr) bool {
	a, ok := e.(*ast.Attributed)
	if !ok {
		return false
	}
	for _, at := range a.Attrs {
		if at.Name == "stream" {
			return true
		}
	}
	return false
}

// StreamTops returns the expressions of a rule body where #stream is allowed: the body, the items
// of its top-level sequence (also those of a sequence grouped in an item) and the expression a
// capture of such an item holds, each with the groups and the attributes other than #recover and
// #missing around it.
func StreamTops(body ast.Expr) map[ast.Expr]bool {
	top := map[ast.Expr]bool{}
	var mark func(e ast.Expr)
	mark = func(e ast.Expr) {
		for e != nil {
			top[e] = true
			switch x := e.(type) {
			case *ast.Group:
				e = x.Expr
			case *ast.Attributed:
				if Recovers(x) {
					return
				}
				e = x.Expr
			case *ast.Capture:
				mark(x.Expr)
				return
			default:
				return
			}
		}
	}
	var items func(e ast.Expr)
	items = func(e ast.Expr) {
		if seq, ok := SeeThrough(e).(*ast.Seq); ok {
			for _, it := range seq.Items {
				mark(it)
				items(it)
			}
		}
	}
	mark(body)
	items(body)
	return top
}

// SeeThrough returns e without the groups and the attributes other than #recover and #missing
// around it.
func SeeThrough(e ast.Expr) ast.Expr {
	for {
		switch x := e.(type) {
		case *ast.Group:
			e = x.Expr
		case *ast.Attributed:
			if Recovers(x) {
				return e
			}
			e = x.Expr
		default:
			return e
		}
	}
}

// LookKeepsState reports whether a positive lookahead of e can leave a capture or a variable
// binding in effect after it: whether e writes a capture or binds a variable outside the rules it
// calls (the captures and bindings of a called rule end with the call). Such a lookahead is not
// summarized by its outcome in a left-recursion context (see "Evaluations of Rules" in
// spec/runtime/parsing.md): every backend gives it no lookahead site, so the growing results it
// reads stay dependencies of the result around it.
func LookKeepsState(e ast.Expr) bool {
	found := false
	astfacts.WalkExpr(e, func(x ast.Expr) {
		switch x := x.(type) {
		case *ast.Capture:
			found = true
		case *ast.Predicate:
			astfacts.WalkTerm(x.Term, func(t ast.Term) {
				if _, ok := t.(*ast.Bind); ok {
					found = true
				}
			})
		}
	})
	return found
}

// LeanLine reports whether the value of an Operators Rule line is unused: the line is an operand
// line with an action, which reads captures only. An operator line's value is $op or its element.
// The bytecode compiler uses the same test.
func LeanLine(action ast.Term, locals []string) bool {
	return action != nil && locals == nil
}
