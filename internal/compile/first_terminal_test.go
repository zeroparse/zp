package compile

import "zeroparse.dev/zp/pkg/grammar/ast"

// firstTerminal returns the terminal (a *ast.Literal or *ast.CharClass) that the expression
// e must begin with, if any, and the number of rule calls on the way: e is the terminal, possibly
// behind sequences, captures, @, - and calls of rules that are neither left-recursive nor OperatorsRule.
// When the next character cannot start the terminal, e fails at once, recording only the
// terminal's expectation and examining only that character (a call memoizes the same failure, and
// memoization only affects speed), so a choice can skip e unless the calls on the way would exceed
// the nesting limit. The backends take the first terminals from Core IR (Attrs.First); this is the
// grammar-level reference the facts tests compare them with.
func firstTerminal(prog *Program, e ast.Expr, depth int, seen map[string]bool) (ast.Expr, int) {
	switch e := e.(type) {
	case *ast.Literal:
		if e.Value != "" {
			return e, depth
		}
	case *ast.CharClass:
		return e, depth
	case *ast.Seq:
		if len(e.Items) > 0 {
			return firstTerminal(prog, e.Items[0], depth, seen)
		}
	case *ast.Capture:
		return firstTerminal(prog, e.Expr, depth, seen)
	case *ast.Atomic:
		return firstTerminal(prog, e.Expr, depth, seen)
	case *ast.Discard:
		return firstTerminal(prog, e.Expr, depth, seen)
	case *ast.Ref:
		r := prog.ByName[e.Name]
		if r == nil || r.Leader || seen[e.Name] || r.Def == nil {
			break
		}
		if _, operatorsRule := r.Def.Expr.(*ast.OperatorsRule); operatorsRule {
			break
		}
		seen[e.Name] = true
		return firstTerminal(prog, r.Def.Expr, depth+r.EntryDepth(), seen)
	}
	return nil, 0
}
