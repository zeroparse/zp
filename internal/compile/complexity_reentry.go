package compile

import "zeroparse.dev/zp/grammar"

// --- Re-entry into a growth ---

// A continuation of an alternative of a rule of a left-recursion cycle can reach, before it
// examines input, a left call of a leader of the cycle, as the last alternative a does in
// c = "(" c ")" "y" / "(" c ")" "z" / a with a = c "+" / "n". That call examines nothing that the
// alternative examined.
//
//   - While a growth of the cycle is in progress at the position, the call reads the growing
//     result there and makes no further call.
//   - Otherwise the memo answers it after the growth at the position has ended, or it starts that
//     growth, which happens once for each position and whose steps are counted as the steps of
//     any growth. Each step evaluates the bodies of the cycle at the position again, the
//     alternative included; when every scanning source evaluated at a position that is the same
//     in each step lies inside a memoized call, the steps after the first answer those calls from
//     the memo.
//
// Such a path is left out of the comparison when the alternative evaluates its scanning sources
// only inside memoized calls, the bodies of the cycle evaluate theirs that way at the positions
// that are the same in each step (fixedMemoized), and what follows the call of the leader
// contains no scanning source, so that it examines again no more than a constant amount of the
// input that the alternative examined.

// withoutReentry returns the continuations of the unit without the paths that re-enter its
// cycle, when the conditions above hold, and the continuations of the unit otherwise.
func (k *classifier) withoutReentry(u *unit) []contSpec {
	if u.kind != unitAlt || !k.an.cyclic[u.rule] || len(u.self) != 1 || len(u.self[0].items) != 1 {
		return u.conts
	}
	cycle := k.an.cycle[u.rule]
	if !k.fixedMemoized(cycle) || !k.memoOnly(u.self[0].items[0], cycle) {
		return u.conts
	}
	out := make([]contSpec, 0, len(u.conts))
	for _, cs := range u.conts {
		var paths []*cont
		for _, p := range cs.paths {
			rests, first, ok := k.leftPaths(p, cycle, "")
			if !ok || len(rests) == 0 || !k.restBounded(rests) {
				paths = append(paths, p)
				continue
			}
			paths = append(paths, first...)
		}
		if len(paths) > 0 {
			cs.paths = paths
			out = append(out, cs)
		}
	}
	return out
}

// fixedMemoized reports whether the bodies of the rules of the cycle evaluate every scanning
// source at a position that is the same in each step of a growth only inside a call that is
// memoized at the call and reads no variable. A position is the same in each step until the body
// has evaluated a left call of a leader of the cycle, which reads the growing result.
func (k *classifier) fixedMemoized(cycle int) bool {
	if v, ok := k.fixed[cycle]; ok {
		return v
	}
	ok := true
	for _, r := range k.rules {
		if ok && k.reach[r.Name] && k.an.cyclic[r.Name] && k.an.cycle[r.Name] == cycle {
			k.visitFixed(r.Expr, cycle, true, &ok)
		}
	}
	if k.fixed == nil {
		k.fixed = map[int]bool{}
	}
	k.fixed[cycle] = ok
	return ok
}

// memoOnly reports whether e evaluates its scanning sources only inside calls that are memoized
// at the call and read no variable, and makes no left call of the cycle.
func (k *classifier) memoOnly(e grammar.Expr, cycle int) bool {
	ok := true
	k.visitFixed(e, cycle, false, &ok)
	return ok
}

// visitFixed clears ok when e evaluates a scanning source outside a call that is memoized at the
// call and reads no variable. With leaders set, a left call of a leader of the cycle ends what it
// checks, and it returns whether e always evaluates one; without it, a left call of the cycle
// clears ok.
func (k *classifier) visitFixed(e grammar.Expr, cycle int, leaders bool, ok *bool) bool {
	k.control.Poll()
	if !*ok {
		return false
	}
	memoized := func(r *grammar.Ref) bool { return k.memoAt(r) && len(k.vars[r.Name]) == 0 }
	switch e := e.(type) {
	case *grammar.Ref:
		if k.byName[e.Name] == nil {
			return false
		}
		if k.an.cyclic[e.Name] && k.an.cycle[e.Name] == cycle && !k.outside[e] {
			if !leaders {
				*ok = false
				return false
			}
			return k.an.leaders[e.Name]
		}
		if (k.srcExprs[e] || k.ruleSrc[e.Name] != nil) && !memoized(e) {
			*ok = false
		}
		return false
	}
	if k.srcExprs[e] {
		*ok = false
		return false
	}
	switch e := e.(type) {
	case *grammar.Seq:
		for _, it := range e.Items {
			if k.visitFixed(it, cycle, leaders, ok) {
				return true
			}
		}
		return false
	case *grammar.Choice:
		all := len(e.Alts) > 0
		for _, alt := range e.Alts {
			all = k.visitFixed(alt, cycle, leaders, ok) && all
		}
		return all
	case *grammar.Repeat:
		if e.Max == 0 {
			return false
		}
		read := k.visitFixed(e.Expr, cycle, leaders, ok)
		return read && e.Min > 0
	case *grammar.Optional:
		k.visitFixed(e.Expr, cycle, leaders, ok)
		return false
	case *grammar.And:
		k.visitFixed(e.Expr, cycle, leaders, ok)
		return false
	case *grammar.Not:
		k.visitFixed(e.Expr, cycle, leaders, ok)
		return false
	case *grammar.Group:
		return k.visitFixed(e.Expr, cycle, leaders, ok)
	case *grammar.Capture:
		return k.visitFixed(e.Expr, cycle, leaders, ok)
	case *grammar.Discard:
		return k.visitFixed(e.Expr, cycle, leaders, ok)
	case *grammar.Atomic:
		return k.visitFixed(e.Expr, cycle, leaders, ok)
	case *grammar.Stream:
		return k.visitFixed(e.Expr, cycle, leaders, ok)
	case *grammar.InlineAction:
		return k.visitFixed(e.Expr, cycle, leaders, ok)
	case *grammar.Recovery:
		read := k.visitFixed(e.Expr, cycle, leaders, ok)
		k.visitFixed(e.Skip, cycle, false, ok)
		return read
	case *grammar.Attributed:
		read := k.visitFixed(e.Expr, cycle, leaders, ok)
		if skip := recoverSkip(e); skip != nil {
			k.visitFixed(skip, cycle, false, ok)
			return false
		}
		return read
	case *grammar.Literal, *grammar.CharClass, *grammar.Any, *grammar.Predicate, *grammar.Cut,
		*grammar.Top, *grammar.BeginInput, *grammar.BeginLine, *grammar.EndInput:
		return false
	}
	if k.containsSource(e) {
		*ok = false
	}
	return false
}

// restBounded reports whether the paths contain no scanning source, up to the end of the parse
// through every call of each rule whose body they end, so that they examine a bounded amount of
// input.
func (k *classifier) restBounded(paths []*cont) bool {
	budget := leadBudget
	ended := map[endKey]bool{}
	var visit func(p *cont) bool
	visit = func(p *cont) bool {
		for ; p != nil; p = p.next {
			k.control.Poll()
			if budget--; budget < 0 {
				return false
			}
			for _, it := range p.items {
				if it == nil {
					continue
				}
				if _, ok := it.(*grammar.EndInput); ok {
					return true
				}
				if r, ok := it.(*grammar.Ref); ok && r.Level == rhsLevel || k.containsSource(it) {
					return false
				}
			}
			if p.next != nil {
				continue
			}
			switch p.end {
			case endOps:
				return false
			case endRule:
				key := endKey{p.end, p.rule}
				if ended[key] {
					return true
				}
				ended[key] = true
				for _, s := range k.sites {
					if s.callee == p.rule && !visit(s.cont) {
						return false
					}
				}
			}
			return true
		}
		return true
	}
	for _, p := range paths {
		if !visit(p) {
			return false
		}
	}
	return true
}
