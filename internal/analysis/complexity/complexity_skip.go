package complexity

import (
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/frontend/astfacts"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// Reused skips.
//
// The walks of the skip-run sites of a parse evaluate the element of each site a number of times
// linear in the input, whatever their number and order, plus a constant for each walk (Lemma 2 of
// spec/proofs/skip-runs.md). A skip expression whose parts outside its sites, and the elements of
// its sites, each examine at most a constant amount of the input therefore costs a constant for
// each of its evaluations, plus a cost linear in the input for the whole parse, and is not a
// scanning source (see "Skip Runs" in spec/runtime/parsing.md and Lemma 9 of
// spec/proofs/linear-cost.md).

// reusedSkip reports whether the skip expression of a #recover is a reused skip, and returns its
// unbounded repetitions, each of which is then a skip-run site: every unbounded repetition written
// in it, outside the elements of others, has no #cut that applies to it, and neither its element
// nor the rest of the skip calls a rule, has a predicate, a #recover or an Operators Rule, or, in
// an element, an unbounded repetition. Without calls and predicates the other conditions of a
// skip-run site hold.
func reusedSkip(skip ast.Expr) ([]*ast.Repeat, bool) {
	var sites []*ast.Repeat
	ok, inside := true, map[ast.Expr]bool{}
	astfacts.WalkExpr(skip, func(x ast.Expr) {
		if inside[x] {
			return
		}
		switch x := x.(type) {
		case *ast.Repeat:
			if x.Max < 0 {
				sites = append(sites, x)
				ok = ok && constantInput(x.Expr) && !astfacts.HasLooseCut(x.Expr)
				astfacts.WalkExpr(x.Expr, func(y ast.Expr) { inside[y] = true })
			}
		default:
			ok = ok && !costly(x)
		}
	})
	return sites, ok
}

// constantInput reports whether every evaluation of e examines at most a constant amount of the
// input: e has no unbounded repetition and nothing that costly reports.
func constantInput(e ast.Expr) bool {
	ok := true
	astfacts.WalkExpr(e, func(x ast.Expr) {
		if r, isRep := x.(*ast.Repeat); isRep {
			ok = ok && r.Max >= 0
			return
		}
		ok = ok && !costly(x)
	})
	return ok
}

// costly reports whether the expression x, without its parts, can cost more than a constant in a
// skip expression: a call, which can examine any amount of the input, a predicate, whose terms can
// build values of any size, a #recover or an Operators Rule.
func costly(x ast.Expr) bool {
	switch x := x.(type) {
	case *ast.Ref, *ast.Predicate, *ast.Recovery, *ast.OperatorsRule:
		return true
	case *ast.Attributed:
		return compile.RecoverSkip(x) != nil
	}
	return false
}

// reused reports whether skip is a reused skip.
func reused(skip ast.Expr) bool {
	_, ok := reusedSkip(skip)
	return ok
}
