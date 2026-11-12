package grammaranalysis

import "zeroparse.dev/zp/grammar"

// WalkTerm visits the value expression and its subexpressions in preorder.
func WalkTerm(t grammar.Term, f func(grammar.Term)) {
	if t == nil {
		return
	}
	f(t)
	switch t := t.(type) {
	case *grammar.TermGroup:
		WalkTerm(t.Term, f)
	case *grammar.Block:
		for _, stmt := range t.Statements {
			WalkTerm(stmt, f)
		}
	case *grammar.Member:
		WalkTerm(t.X, f)
	case *grammar.New:
		for _, fi := range t.Fields {
			WalkTerm(fi.Value, f)
		}
		for _, x := range t.Args {
			WalkTerm(x, f)
		}
	case *grammar.TupleLit:
		for _, x := range t.Elems {
			WalkTerm(x, f)
		}
	case *grammar.ListConstructor:
		for _, item := range t.Items {
			WalkTerm(item, f)
		}
	case *grammar.Call:
		for _, a := range t.Args {
			WalkTerm(a, f)
		}
	case *grammar.Lambda:
		WalkTerm(t.Body, f)
	case *grammar.Binary:
		WalkTerm(t.L, f)
		WalkTerm(t.R, f)
	case *grammar.Unary:
		WalkTerm(t.X, f)
	case *grammar.Cast:
		WalkTerm(t.X, f)
	case *grammar.Bind:
		WalkTerm(t.Value, f)
	}
}
