package lower

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/ir/core"
)

// The eligibility of the compiled rules for Flow IR: which rules and expressions are lowered as flat
// rules, as framed bodies and as lines of an Operators Rule. A backend that prints from Flow IR asks
// these, so every backend lowers the same rules.

// BodyOK reports whether Flow IR can lower the expression e in place. A cut is allowed if cuts is set
// and a recovery attribute if recovery is set; an Operators Rule never is.
func BodyOK(expr grammar.Expr, cuts, recovery bool) bool {
	ok := true
	compile.WalkExpr(expr, func(e grammar.Expr) {
		switch e := e.(type) {
		case *grammar.Cut:
			if !cuts {
				ok = false
			}
		case *grammar.OperatorsRule:
			ok = false
		case *grammar.Attributed:
			for _, a := range e.Attrs {
				if a.Name == "recover" && !recovery {
					ok = false
				}
			}
		}
	})
	return ok
}

// LineOK reports whether the Operators Rule line e is a framed body that Flow IR can lower inside the
// runtime-owned frame of its rule.
func LineOK(e grammar.Expr) bool {
	return BodyOK(e, true, true)
}

// RecEligible reports whether the rule r of the Node table or of the recognizer table is lowered as
// a flat rule: a plain, lean, value-free rule that is not a left-recursion leader and whose body has
// no cut, recovery, Operators Rule or semantic predicate. Captures unused by recognition are omitted;
// a predicate keeps the general form because it can require values, capture frames and variable
// bindings.
func RecEligible(r *compile.Rule) bool {
	if !r.Plain || !r.Lean || !r.Novalue || r.Leader || !BodyOK(r.BodyExpr(), false, false) {
		return false
	}
	ok := true
	compile.WalkExpr(r.BodyExpr(), func(e grammar.Expr) {
		if _, predicate := e.(*grammar.Predicate); predicate {
			ok = false
		}
	})
	return ok
}

// NodeFrameOK reports whether the rule r of the Node or recognizer table, whose Core rule is cr, is
// lowered as a framed body.
//
// The body is one that a flat rule could have, except that it may cut or recover, and that the rule
// may be a left-recursion leader. Flow IR lowers a cut as the typed table does (a flag per
// backtracking point, published to the runtime's cut flag on return), so the runtime's frame, memo
// and finish ownership are the same. The runtime owns the growth of a leader's seed around its body,
// so the body is the matcher of any other rule; its left-recursive alternatives have no first set and
// are never guarded. The runtime's recovery sequence is lowered inline.
//
// Core IR gathers the values of a projected repetition, and the actions that read it expect that
// shape. The Node table writes its actions with the projections of the rule, which are the ones Core
// IR makes, so a Core-lowered body agrees with them. The recognizer's table writes its actions
// without projections, so its Core IR is built without them and agrees with them as well; either
// table's body is eligible when its action reads only captures of the frame.
func NodeFrameOK(r *compile.Rule, cr *core.Rule) bool {
	if !BodyOK(r.BodyExpr(), true, true) || !FramedBody(cr.Body) {
		return false
	}
	if r.Action == nil {
		return true
	}
	known := map[string]bool{}
	for _, sl := range cr.Scope {
		known[sl.Name] = true
	}
	ok := true
	capRefs(r.Action, func(name string) {
		if !known[name] {
			ok = false
		}
	})
	return ok
}

// capRefs calls f with the name of each capture the term t refers to outside lambdas that bind the
// name.
func capRefs(t grammar.Term, f func(name string)) {
	var walk func(t grammar.Term, shadow map[string]bool)
	walk = func(t grammar.Term, shadow map[string]bool) {
		switch t := t.(type) {
		case *grammar.CaptureRef:
			if !shadow[t.Name] {
				f(t.Name)
			}
		case *grammar.Lambda:
			inner := map[string]bool{}
			for k := range shadow {
				inner[k] = true
			}
			for _, p := range t.Params {
				inner[p] = true
			}
			walk(t.Body, inner)
		default:
			termChildren(t, func(x grammar.Term) { walk(x, shadow) })
		}
	}
	walk(t, map[string]bool{})
}

// termChildren calls f with the direct subterms of t.
func termChildren(t grammar.Term, f func(grammar.Term)) {
	switch t := t.(type) {
	case *grammar.Member:
		f(t.X)
	case *grammar.New:
		for _, fi := range t.Fields {
			f(fi.Value)
		}
	case *grammar.ListConstructor:
		for _, item := range t.Items {
			f(item)
		}
	case *grammar.TupleLit:
		for _, x := range t.Elems {
			f(x)
		}
	case *grammar.Call:
		for _, a := range t.Args {
			f(a)
		}
	case *grammar.Lambda:
		f(t.Body)
	case *grammar.Binary:
		f(t.L)
		f(t.R)
	case *grammar.Unary:
		f(t.X)
	case *grammar.Cast:
		f(t.X)
	case *grammar.Bind:
		f(t.Value)
	}
}
