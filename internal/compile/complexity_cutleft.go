package compile

import "zeroparse.dev/zp/grammar"

// checkCutLeftCalls rejects, in the rules of a cycle with more than one leader, a #cut that is
// passed after a left call in the alternative, iteration or option that it commits, and a
// lookahead that can make a left call. The growth of one leader reads the other, and such a cut
// or lookahead makes a later step of a nested growth reach an alternative that its first step did
// not, so the growths cannot be told apart by what they include.
func (k *classifier) checkCutLeftCalls() {
	leaders := map[int]int{}
	for name := range k.an.leaders {
		if k.an.cyclic[name] {
			leaders[k.an.cycle[name]]++
		}
	}
	for _, r := range k.rules {
		if !k.reach[r.Name] || !k.an.cyclic[r.Name] || leaders[k.an.cycle[r.Name]] < 2 {
			continue
		}
		k.cutLeftScan(r.Name, r.Expr, false)
	}
}

// cutLeftScan walks e of the rule; left says that a left call was evaluated earlier in the
// alternative, iteration or option that a #cut in e commits.
func (k *classifier) cutLeftScan(rule string, e grammar.Expr, left bool) {
	k.control.Poll()
	switch e := e.(type) {
	case *grammar.Group:
		k.cutLeftScan(rule, e.Expr, left)
	case *grammar.Capture:
		k.cutLeftScan(rule, e.Expr, left)
	case *grammar.Discard:
		k.cutLeftScan(rule, e.Expr, left)
	case *grammar.Atomic:
		k.cutLeftScan(rule, e.Expr, left)
	case *grammar.Stream:
		k.cutLeftScan(rule, e.Expr, left)
	case *grammar.Seq:
		for _, it := range e.Items {
			k.cutLeftScan(rule, it, left)
			if found, _ := k.leftScan(rule, it, leftExamine, 0); found {
				left = true
			}
		}
	case *grammar.Choice:
		for _, alt := range e.Alts {
			k.cutLeftScan(rule, alt, false)
		}
	case *grammar.Optional:
		k.cutLeftScan(rule, e.Expr, false)
	case *grammar.Repeat:
		k.cutLeftScan(rule, e.Expr, false)
	case *grammar.And:
		k.cutLeftLookahead(rule, e, e.Expr)
	case *grammar.Not:
		k.cutLeftLookahead(rule, e, e.Expr)
	case *grammar.Cut:
		if left {
			k.reason(rule, exprPos(e), "the #cut at %s is passed after a left call in a cycle with more than one leader, which the classifier does not analyze", at(rule, exprPos(e)))
		}
	}
}

func (k *classifier) cutLeftLookahead(rule string, e, body grammar.Expr) {
	if k.startsWithLeftCall(rule, body, leftConsume) {
		k.reason(rule, exprPos(e), "the lookahead %s at %s can make a left call in a cycle with more than one leader, which the classifier does not analyze", k.show(e), at(rule, exprPos(e)))
		return
	}
	k.cutLeftScan(rule, body, false)
}
