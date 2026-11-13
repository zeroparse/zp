package compile

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
)

// Resumable reports whether a repetition with the element e can be resumed (build.Program). An
// element whose captures are written to the enclosing scope (those inside &, when the element has
// no captures of its own) cannot be: reusing it would not write them.
func (prog *Program) Resumable(e grammar.Expr) bool { return prog.resumable(e, nil) }

func (prog *Program) resumable(e grammar.Expr, control *cancel.Control) bool {
	ok := elementScopedControlled(e, control) || !anyCapture(e, control)
	walkExprControlled(e, func(x grammar.Expr) {
		switch x := x.(type) {
		case *grammar.Predicate:
			ok = false
		case *grammar.Ref:
			if r := prog.ByName[x.Name]; r == nil || len(r.Vars) > 0 {
				ok = false
			}
		}
	}, control)
	return ok
}

// anyCapture reports whether e contains a capture anywhere, lookaheads included.
func anyCapture(e grammar.Expr, control *cancel.Control) bool {
	control.Poll()
	found := false
	walkExprControlled(e, func(x grammar.Expr) {
		if _, ok := x.(*grammar.Capture); ok {
			found = true
		}
	}, control)
	return found
}

// Positional reports whether the value of e may contain position values (build.Program).
func (prog *Program) Positional(e grammar.Expr) bool { return prog.positional(e, nil) }

func (prog *Program) positional(e grammar.Expr, control *cancel.Control) bool {
	found := false
	walkExprControlled(e, func(x grammar.Expr) {
		if x, ok := x.(*grammar.Ref); ok {
			if r := prog.ByName[x.Name]; r == nil || r.Positional {
				found = true
			}
		}
	}, control)
	return found
}
