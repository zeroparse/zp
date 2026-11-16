package complexity

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/grammaranalysis"
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
func reusedSkip(skip grammar.Expr) ([]*grammar.Repeat, bool) {
	var sites []*grammar.Repeat
	ok, inside := true, map[grammar.Expr]bool{}
	grammaranalysis.WalkExpr(skip, func(x grammar.Expr) {
		if inside[x] {
			return
		}
		switch x := x.(type) {
		case *grammar.Repeat:
			if x.Max < 0 {
				sites = append(sites, x)
				ok = ok && constantInput(x.Expr) && !grammaranalysis.HasLooseCut(x.Expr)
				grammaranalysis.WalkExpr(x.Expr, func(y grammar.Expr) { inside[y] = true })
			}
		default:
			ok = ok && !costly(x)
		}
	})
	return sites, ok
}

// constantInput reports whether every evaluation of e examines at most a constant amount of the
// input: e has no unbounded repetition and nothing that costly reports.
func constantInput(e grammar.Expr) bool {
	ok := true
	grammaranalysis.WalkExpr(e, func(x grammar.Expr) {
		if r, isRep := x.(*grammar.Repeat); isRep {
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
func costly(x grammar.Expr) bool {
	switch x := x.(type) {
	case *grammar.Ref, *grammar.Predicate, *grammar.Recovery, *grammar.OperatorsRule:
		return true
	case *grammar.Attributed:
		return compile.RecoverSkip(x) != nil
	}
	return false
}

// reused reports whether skip is a reused skip.
func reused(skip grammar.Expr) bool {
	_, ok := reusedSkip(skip)
	return ok
}
