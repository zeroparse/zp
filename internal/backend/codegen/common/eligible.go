package common

import (
	"zeroparse.dev/zp/internal/frontend/astfacts"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/middleend/flowlower"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// The eligibility of the compiled rules for Flow IR: which rules are lowered as flat rules and which
// as framed bodies. Every generator asks these, so every backend lowers the same rules.

// RecEligible reports whether the rule r of the Node table or of the recognizer table is lowered as
// a flat rule: a plain, lean, value-free rule that is not a left-recursion leader and whose body has
// no cut, recovery, Operators Rule or semantic predicate. Captures unused by recognition are omitted;
// a predicate keeps the general form because it can require values, capture frames and variable
// bindings.
func RecEligible(r *Rule) bool {
	if !r.Plain || !r.Lean || !r.Novalue || r.Leader || !flowlower.BodyOK(r.BodyExpr(), false, false) {
		return false
	}
	ok := true
	astfacts.WalkExpr(r.BodyExpr(), func(e ast.Expr) {
		if _, predicate := e.(*ast.Predicate); predicate {
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
func NodeFrameOK(r *Rule, cr *core.Rule) bool {
	if !flowlower.BodyOK(r.BodyExpr(), true, true) || !flowlower.FramedBody(cr.Body) {
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
func capRefs(t ast.ActionExpr, f func(name string)) {
	var walk func(t ast.ActionExpr, shadow map[string]bool)
	walk = func(t ast.ActionExpr, shadow map[string]bool) {
		switch t := t.(type) {
		case *ast.CaptureRef:
			if !shadow[t.Name] {
				f(t.Name)
			}
		case *ast.Lambda:
			inner := map[string]bool{}
			for k := range shadow {
				inner[k] = true
			}
			for _, p := range t.Params {
				inner[p] = true
			}
			walk(t.Body, inner)
		default:
			termChildren(t, func(x ast.ActionExpr) { walk(x, shadow) })
		}
	}
	walk(t, map[string]bool{})
}

// termChildren calls f with the direct subterms of t.
func termChildren(t ast.ActionExpr, f func(ast.ActionExpr)) {
	switch t := t.(type) {
	case *ast.Member:
		f(t.X)
	case *ast.New:
		for _, fi := range t.Fields {
			f(fi.Value)
		}
	case *ast.ListConstructor:
		for _, item := range t.Items {
			f(item)
		}
	case *ast.TupleLit:
		for _, x := range t.Elems {
			f(x)
		}
	case *ast.Call:
		for _, a := range t.Args {
			f(a)
		}
	case *ast.Lambda:
		f(t.Body)
	case *ast.Binary:
		f(t.L)
		f(t.R)
	case *ast.Unary:
		f(t.X)
	case *ast.Cast:
		f(t.X)
	case *ast.Bind:
		f(t.Value)
	}
}
