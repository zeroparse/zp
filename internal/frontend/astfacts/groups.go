package astfacts

import "zeroparse.dev/zp/pkg/grammar/ast"

// UngroupType removes source-only grouping without changing its input.
func UngroupType(t ast.TypeExpr) ast.TypeExpr {
	return ungroupType(t, nil)
}

// UngroupTypeWithPoll also visits each traversed node and copied slice element.
// The callback may panic to interrupt lowering; the source is never modified.
func UngroupTypeWithPoll(t ast.TypeExpr, poll func()) ast.TypeExpr {
	return ungroupType(t, poll)
}

func ungroupType(t ast.TypeExpr, poll func()) ast.TypeExpr {
	if poll != nil {
		poll()
	}
	switch t := t.(type) {
	case *ast.TypeGroup:
		return ungroupType(t.Type, poll)
	case *ast.ListType:
		child := ungroupType(t.Elem, poll)
		if child != t.Elem {
			v := *t
			v.Elem = child
			return &v
		}
	case *ast.OptionalType:
		child := ungroupType(t.Elem, poll)
		if child != t.Elem {
			v := *t
			v.Elem = child
			return &v
		}
	case *ast.JoinType:
		var children []ast.TypeExpr
		for i, x := range t.Types {
			child := ungroupType(x, poll)
			if child != x {
				if children == nil {
					children = cloneGroupItems(t.Types, poll)
				}
				children[i] = child
			}
		}
		if children != nil {
			v := *t
			v.Types = children
			return &v
		}
	case *ast.MeetType:
		var children []ast.TypeExpr
		for i, x := range t.Types {
			child := ungroupType(x, poll)
			if child != x {
				if children == nil {
					children = cloneGroupItems(t.Types, poll)
				}
				children[i] = child
			}
		}
		if children != nil {
			v := *t
			v.Types = children
			return &v
		}
	case *ast.TupleType:
		if children := ungroupTypes(t.Types, poll); children != nil {
			v := *t
			v.Types = children
			return &v
		}
	case *ast.TypeApp:
		if children := ungroupTypes(t.Args, poll); children != nil {
			v := *t
			v.Args = children
			return &v
		}
	case *ast.StructType:
		var fields []*ast.Field
		for i, f := range t.Fields {
			child := ungroupType(f.Type, poll)
			if child != f.Type {
				if fields == nil {
					fields = cloneGroupItems(t.Fields, poll)
				}
				v := *f
				v.Type = child
				fields[i] = &v
			}
		}
		if fields != nil {
			v := *t
			v.Fields = fields
			return &v
		}
	}
	return t
}

// ungroupTypes ungroups each of ts, returning a copy if any changed and nil otherwise.
func ungroupTypes(ts []ast.TypeExpr, poll func()) []ast.TypeExpr {
	var children []ast.TypeExpr
	for i, x := range ts {
		child := ungroupType(x, poll)
		if child != x {
			if children == nil {
				children = cloneGroupItems(ts, poll)
			}
			children[i] = child
		}
	}
	return children
}

// UngroupTerm removes source-only grouping without changing its input.
func UngroupTerm(t ast.Term) ast.Term {
	return ungroupTerm(t, nil)
}

// UngroupTermWithPoll is UngroupTerm with the same polling contract as
// UngroupTypeWithPoll, including nested type expressions and slice copies.
func UngroupTermWithPoll(t ast.Term, poll func()) ast.Term {
	return ungroupTerm(t, poll)
}

func ungroupTerm(t ast.Term, poll func()) ast.Term {
	if poll != nil {
		poll()
	}
	switch t := t.(type) {
	case *ast.TermGroup:
		return ungroupTerm(t.Term, poll)
	case *ast.Member:
		child := ungroupTerm(t.X, poll)
		if child != t.X {
			v := *t
			v.X = child
			return &v
		}
	case *ast.TupleLit:
		elems := ungroupTerms(t.Elems, poll)
		if elems != nil {
			v := *t
			v.Elems = elems
			return &v
		}
	case *ast.ListConstructor:
		elem, items := ungroupType(t.Elem, poll), ungroupTerms(t.Items, poll)
		if elem != t.Elem || items != nil {
			v := *t
			v.Elem = elem
			if items != nil {
				v.Items = items
			}
			return &v
		}
	case *ast.New:
		var fields []*ast.FieldInit
		for i, f := range t.Fields {
			child := ungroupTerm(f.Value, poll)
			if child != f.Value {
				if fields == nil {
					fields = cloneGroupItems(t.Fields, poll)
				}
				v := *f
				v.Value = child
				fields[i] = &v
			}
		}
		var args []ast.Term
		for i, x := range t.Args {
			child := ungroupTerm(x, poll)
			if child != x {
				if args == nil {
					args = cloneGroupItems(t.Args, poll)
				}
				args[i] = child
			}
		}
		if fields != nil || args != nil {
			v := *t
			if fields != nil {
				v.Fields = fields
			}
			if args != nil {
				v.Args = args
			}
			return &v
		}
	case *ast.Call:
		args := ungroupTerms(t.Args, poll)
		if args != nil {
			v := *t
			v.Args = args
			return &v
		}
	case *ast.Lambda:
		child := ungroupTerm(t.Body, poll)
		if child != t.Body {
			v := *t
			v.Body = child
			return &v
		}
	case *ast.Binary:
		l, r := ungroupTerm(t.L, poll), ungroupTerm(t.R, poll)
		if l != t.L || r != t.R {
			v := *t
			v.L, v.R = l, r
			return &v
		}
	case *ast.Cast:
		child, typ := ungroupTerm(t.X, poll), ungroupType(t.Type, poll)
		if child != t.X || typ != t.Type {
			v := *t
			v.X, v.Type = child, typ
			return &v
		}
	case *ast.Unary:
		child := ungroupTerm(t.X, poll)
		if child != t.X {
			v := *t
			v.X = child
			return &v
		}
	case *ast.Bind:
		child := ungroupTerm(t.Value, poll)
		if child != t.Value {
			v := *t
			v.Value = child
			return &v
		}
	case *ast.Block:
		children := ungroupTerms(t.Statements, poll)
		if children != nil {
			v := *t
			v.Statements = children
			return &v
		}
	}
	return t
}

func ungroupTerms(ts []ast.Term, poll func()) []ast.Term {
	var children []ast.Term
	for i, x := range ts {
		child := ungroupTerm(x, poll)
		if child != x {
			if children == nil {
				children = cloneGroupItems(ts, poll)
			}
			children[i] = child
		}
	}
	return children
}

// Ordinary callers retain the bulk copy; cancellable traversals count copied
// elements as well as visited nodes. Allocations remain nonpreemptible.
func cloneGroupItems[T any](src []T, poll func()) []T {
	if poll == nil {
		return append([]T(nil), src...)
	}
	poll()
	out := make([]T, len(src))
	for i, value := range src {
		poll()
		out[i] = value
	}
	return out
}
