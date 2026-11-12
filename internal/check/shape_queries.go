package check

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/grammaranalysis"
)

// ElementScopedControlled reports whether the repetition element e gets a capture scope of its own
// per iteration: when it contains captures (HasCapturesControlled), or its predicates read captures, which
// can then only be its own (made inside &) or, an error, those of the enclosing scope. Every
// backend and generator must decide the same way, since the scope decides where captures are
// written.
func ElementScopedControlled(e grammar.Expr, control *cancel.Control) bool {
	control.Poll()
	return HasCapturesControlled(e, control) || predicatesReadCaptures(e, control)
}

// predicatesReadCaptures reports whether a predicate in e refers to a capture.
func predicatesReadCaptures(e grammar.Expr, control *cancel.Control) bool {
	control.Poll()
	found := false
	walkExprControlled(e, func(x grammar.Expr) {
		if p, ok := x.(*grammar.Predicate); ok {
			grammaranalysis.WalkTerm(p.Term, func(t grammar.Term) {
				control.Poll()
				if _, ok := t.(*grammar.CaptureRef); ok {
					found = true
				}
			})
		}
	}, control)
	return found
}

// HasCapturesControlled reports whether the expression contains captures (excluding nested
// repetitions), also inside positive lookaheads, polling control.
func HasCapturesControlled(e grammar.Expr, control *cancel.Control) bool {
	control.Poll()
	switch e := e.(type) {
	case *grammar.Capture:
		return true
	case *grammar.Seq:
		for _, it := range e.Items {
			control.Poll()
			if HasCapturesControlled(it, control) {
				return true
			}
		}
	case *grammar.Choice:
		for _, alt := range e.Alts {
			control.Poll()
			if HasCapturesControlled(alt, control) {
				return true
			}
		}
	case *grammar.Optional:
		return HasCapturesControlled(e.Expr, control)
	case *grammar.Attributed:
		return HasCapturesControlled(e.Expr, control)
	case *grammar.Group:
		return HasCapturesControlled(e.Expr, control)
	case *grammar.And:
		// The captures of a positive lookahead are kept beside the element's value.
		return HasCapturesControlled(e.Expr, control)
	}
	return false
}

// IsStream reports whether the expression is an expression with the #stream pragma.
func IsStream(e grammar.Expr) bool {
	a, ok := e.(*grammar.Attributed)
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
func StreamTops(body grammar.Expr) map[grammar.Expr]bool {
	top := map[grammar.Expr]bool{}
	var mark func(e grammar.Expr)
	mark = func(e grammar.Expr) {
		for e != nil {
			top[e] = true
			switch x := e.(type) {
			case *grammar.Group:
				e = x.Expr
			case *grammar.Attributed:
				if Recovers(x) {
					return
				}
				e = x.Expr
			case *grammar.Capture:
				mark(x.Expr)
				return
			default:
				return
			}
		}
	}
	var items func(e grammar.Expr)
	items = func(e grammar.Expr) {
		if seq, ok := SeeThrough(e).(*grammar.Seq); ok {
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
func SeeThrough(e grammar.Expr) grammar.Expr {
	for {
		switch x := e.(type) {
		case *grammar.Group:
			e = x.Expr
		case *grammar.Attributed:
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
func LookKeepsState(e grammar.Expr) bool {
	found := false
	grammaranalysis.WalkExpr(e, func(x grammar.Expr) {
		switch x := x.(type) {
		case *grammar.Capture:
			found = true
		case *grammar.Predicate:
			grammaranalysis.WalkTerm(x.Term, func(t grammar.Term) {
				if _, ok := t.(*grammar.Bind); ok {
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
func LeanLine(action grammar.Term, locals []string) bool {
	return action != nil && locals == nil
}
