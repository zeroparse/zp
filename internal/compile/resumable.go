package compile

import (
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// Resumable reports whether a repetition with the element e can be resumed (build.Program). An
// element whose captures are written to the enclosing scope (those inside &, when the element has
// no captures of its own) cannot be: reusing it would not write them.
func (prog *Program) Resumable(e ast.Expr) bool { return prog.resumable(e, nil) }

func (prog *Program) resumable(e ast.Expr, control *cancel.Control) bool {
	ok := elementScopedControlled(e, control) || !anyCapture(e, control)
	walkExprControlled(e, func(x ast.Expr) {
		switch x := x.(type) {
		case *ast.Predicate:
			ok = false
		case *ast.Ref:
			if r := prog.ByName[x.Name]; r == nil || len(r.Vars) > 0 {
				ok = false
			}
		}
	}, control)
	return ok
}

// anyCapture reports whether e contains a capture anywhere, lookaheads included.
func anyCapture(e ast.Expr, control *cancel.Control) bool {
	control.Poll()
	found := false
	walkExprControlled(e, func(x ast.Expr) {
		if _, ok := x.(*ast.Capture); ok {
			found = true
		}
	}, control)
	return found
}

// Positional reports whether the value of e may contain position values (build.Program).
func (prog *Program) Positional(e ast.Expr) bool { return prog.positional(e, nil) }

func (prog *Program) positional(e ast.Expr, control *cancel.Control) bool {
	found := false
	walkExprControlled(e, func(x ast.Expr) {
		if x, ok := x.(*ast.Ref); ok {
			if r := prog.ByName[x.Name]; r == nil || r.Positional {
				found = true
			}
		}
	}, control)
	return found
}
