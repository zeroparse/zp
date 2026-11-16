package complexity

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
)

// checkCutLeftCalls rejects, in the rules of a cycle with more than one leader, a #cut that is
// passed after a left call in the alternative, iteration or option that it commits, and a
// lookahead that can make a left call. The growth of one leader reads the other, and such a cut
// or lookahead makes a later step of a nested growth reach an alternative that its first step did
// not, so the growths cannot be told apart by what they include.
func (k *classifier) checkCutLeftCalls() {
	leaders := map[int]int{}
	for name := range k.an.Leaders {
		if k.an.Cyclic[name] {
			leaders[k.an.Cycle[name]]++
		}
	}
	for _, r := range k.rules {
		if !k.reach[r.Name] || !k.an.Cyclic[r.Name] || leaders[k.an.Cycle[r.Name]] < 2 {
			continue
		}
		k.cutLeftScan(r.Name, r.Expr, cutLeftState{start: true})
	}
}

// cutLeftState is what cutLeftScan knows before an expression of the rule body.
type cutLeftState struct {
	// start says that the expression can begin where the rule body begins: some path from the
	// start of the body to it consumes no input, so a call of a rule of the cycle there is a left
	// call.
	start bool
	// left says that a left call can have been evaluated earlier in the alternative, iteration or
	// option that a #cut at the expression commits.
	left bool
}

// cutLeftScan walks e of the rule from state s. It returns whether e can complete at the start of
// the rule body, and whether e can evaluate a left call. A #cut commits the innermost enclosing
// choice, option or repetition of the rule, through groups, captures, lookaheads and recoveries,
// so only those three begin a new region for the left calls that a cut is passed after.
func (k *classifier) cutLeftScan(rule string, e grammar.Expr, s cutLeftState) (start, found bool) {
	k.control.Poll()
	switch e := e.(type) {
	case nil:
		return s.start, false
	case *grammar.Ref:
		if k.an.Cyclic[e.Name] && k.sameCycle(rule, e.Name) && e.Level == "" {
			return s.start && k.nullable(e), s.start
		}
		return s.start && k.nullable(e), false
	case *grammar.Group:
		return k.cutLeftScan(rule, e.Expr, s)
	case *grammar.Capture:
		return k.cutLeftScan(rule, e.Expr, s)
	case *grammar.Discard:
		return k.cutLeftScan(rule, e.Expr, s)
	case *grammar.Atomic:
		return k.cutLeftScan(rule, e.Expr, s)
	case *grammar.Stream:
		return k.cutLeftScan(rule, e.Expr, s)
	case *grammar.InlineAction:
		return k.cutLeftScan(rule, e.Expr, s)
	case *grammar.Seq:
		for _, it := range e.Items {
			st, f := k.cutLeftScan(rule, it, s)
			s.start = st
			s.left = s.left || f
			found = found || f
		}
		return s.start, found
	case *grammar.Choice:
		for _, alt := range e.Alts {
			st, f := k.cutLeftScan(rule, alt, cutLeftState{start: s.start})
			start = start || st
			found = found || f
		}
		return start, found
	case *grammar.Optional:
		_, f := k.cutLeftScan(rule, e.Expr, cutLeftState{start: s.start})
		return s.start, f
	case *grammar.Repeat:
		st, f := k.cutLeftScan(rule, e.Expr, cutLeftState{start: s.start})
		return s.start && (e.Min == 0 || st), f
	case *grammar.Recovery:
		return k.cutLeftRecovery(rule, e.Expr, e.Skip, s)
	case *grammar.Attributed:
		if skip := compile.RecoverSkip(e); skip != nil {
			return k.cutLeftRecovery(rule, e.Expr, skip, s)
		}
		st, f := k.cutLeftScan(rule, e.Expr, s)
		return st || (s.start && k.nullable(e)), f
	case *grammar.And:
		return k.cutLeftLookahead(rule, e, e.Expr, s)
	case *grammar.Not:
		return k.cutLeftLookahead(rule, e, e.Expr, s)
	case *grammar.Cut:
		if s.left {
			k.reason(rule, exprPos(e), "the #cut at %s is passed after a left call in a cycle with more than one leader, which the classifier does not analyze", at(rule, exprPos(e)))
		}
		return s.start, false
	}
	return s.start && k.nullable(e), false
}

// cutLeftRecovery walks a recovery: its skip runs after its expression fails, at the start of the
// expression or later, so a left call in the expression is earlier than a cut in the skip.
func (k *classifier) cutLeftRecovery(rule string, expr, skip grammar.Expr, s cutLeftState) (start, found bool) {
	st, f := k.cutLeftScan(rule, expr, s)
	ss, fs := k.cutLeftScan(rule, skip, cutLeftState{start: s.start, left: s.left || f})
	return st || ss, f || fs
}

// cutLeftLookahead walks a lookahead. The lookahead consumes no input, so what follows it begins
// where it does; a lookahead whose expression can make a left call is reported, and so is each
// such lookahead nested in it.
func (k *classifier) cutLeftLookahead(rule string, e, body grammar.Expr, s cutLeftState) (start, found bool) {
	if _, found = k.cutLeftScan(rule, body, s); found {
		k.reason(rule, exprPos(e), "the lookahead %s at %s can make a left call in a cycle with more than one leader, which the classifier does not analyze", k.show(e), at(rule, exprPos(e)))
	}
	return s.start, found
}
