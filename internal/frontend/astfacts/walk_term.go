package astfacts

import "zeroparse.dev/zp/pkg/grammar/ast"

// WalkTerm visits the value expression and its subexpressions in preorder.
func WalkTerm(t ast.ActionExpr, f func(ast.ActionExpr)) {
	if t == nil {
		return
	}
	f(t)
	switch t := t.(type) {
	case *ast.ActionGroup:
		WalkTerm(t.Action, f)
	case *ast.Block:
		for _, stmt := range t.Statements {
			WalkTerm(stmt, f)
		}
	case *ast.Member:
		WalkTerm(t.X, f)
	case *ast.New:
		for _, fi := range t.Fields {
			WalkTerm(fi.Value, f)
		}
		for _, x := range t.Args {
			WalkTerm(x, f)
		}
	case *ast.TupleLit:
		for _, x := range t.Elems {
			WalkTerm(x, f)
		}
	case *ast.ListConstructor:
		for _, item := range t.Items {
			WalkTerm(item, f)
		}
	case *ast.Call:
		for _, a := range t.Args {
			WalkTerm(a, f)
		}
	case *ast.Lambda:
		WalkTerm(t.Body, f)
	case *ast.Binary:
		WalkTerm(t.L, f)
		WalkTerm(t.R, f)
	case *ast.Unary:
		WalkTerm(t.X, f)
	case *ast.Cast:
		WalkTerm(t.X, f)
	case *ast.Bind:
		WalkTerm(t.Value, f)
	}
}
