package compile

import (
	"slices"

	"zeroparse.dev/zp/grammar"
)

// --- Nesting at one end ---
//
// A call t of a rule Y from a rule Z of its recursion nests at one end when the evaluations of Z
// can nest without a bound through t, each inner one inside the call t of the outer one, with all
// of those calls ending at one position: each evaluation of Z then evaluates what follows t at
// that position, once for each level. That holds when Y is in the set E_t, the least set of rules
// of the recursion with:
//
//   - (E1) Z, when what follows t can match without consuming input;
//   - (E2) a rule that calls a rule of E_t at a call that what can match without consuming input
//     follows;
//   - (E3) a rule A with a unit and a continuation of it that evaluate calls alike, from the
//     start of the unit, where one of those calls, in the unit, is t itself or a call that the
//     unit reaches by looking through a call at t, and the continuation, after it, can reach
//     the end of the body of A without consuming input.
//
// E3 is the restart: the unit fails after a nested call at t ended at a position, and the
// continuation ends A at that position through the same calls, which the memo answers with what
// the unit's calls found. In e = "(" (e R "x" / e) / ".", the call of e in e R "x" nests at one
// end by E3, and R scans the same input once for each level. A restart through a call of a rule
// of E_t needs no clause of its own: the continuation's call of that rule is followed, up to the
// end of the body of A, by what can match without consuming input, so E2 adds A.
//
// The calls that units evaluate alike come from the analyses that rely on them: the shared calls
// of strip and the calls that the exclusion analysis passes alike. nestsAt computes the set from
// the shared calls of every unit of a recursion against each of its continuations before the
// checks, without any call nesting (so without the shortening that nesting causes, which can only
// remove shared calls). The checks record the calls that they relied on, and checkRestarts turns
// any that would make another call nest into a reason.

// restartsAhead makes nestsAt read the shared calls of the units before the checks. It is on; the
// variable is a test hook for checkRestarts.
var restartsAhead = true

// restart is a call that a unit of rule evaluates alike with a continuation, after which the
// continuation can reach the end of the body of rule without consuming input (E3).
type restart struct {
	rule string
	call *grammar.Ref // the call in the unit
	via  *refList     // the calls that the unit looked through before it
}

// nestsAt reports whether the call s nests at one end: its callee is in E_s.
func (k *classifier) nestsAt(s *site) bool {
	if s == nil || k.nestDry {
		return false
	}
	k.computeNesting()
	return k.nestSites[s]
}

// computeNesting computes the call sites that nest at one end, once.
func (k *classifier) computeNesting() {
	if k.nestSites == nil {
		if restartsAhead {
			k.nestDry = true
			k.collectRestarts()
			k.nestDry = false
		}
		k.nestSites = k.nestSet(k.restarts)
	}
}

// collectRestarts records the shared calls of each unit of a recursion against each of its
// continuations: those of checkUnits, the operator parts that the enclosing levels of an
// Operators Rule try again, and the continuations without re-entries.
func (k *classifier) collectRestarts() {
	retried := map[*unit]*cont{}
	for _, r := range k.retried {
		retried[r.u] = r.end
	}
	for _, u := range k.units {
		k.control.Poll()
		// A unit restarts a nesting only if it fails after the scans of nested evaluations, so
		// only after a scanning source (see unitFailsAfterSource); the checks analyze no other.
		if !k.recursive[u.rule] || !u.hasSource || u.unsupported != "" || !k.unitFailsAfterSource(u) {
			continue
		}
		conts := slices.Clone(u.conts)
		if end := retried[u]; end != nil {
			conts = append(conts, contSpec{paths: []*cont{end}})
		}
		if !u.growing {
			conts = append(conts, k.withoutReentry(u)...)
		}
		for _, cs := range conts {
			k.strip(u.self, cs.paths, k.keepFor(0, cs.growth), u)
		}
	}
}

// recordShared records a shared call of owner, a unit, and its continuation (see restart). lu are
// the leads of the unit at the call, lc those of the continuation.
func (k *classifier) recordShared(owner *unit, lu, lc []lead) {
	if owner == nil || !k.recursive[owner.rule] {
		return
	}
	ends := false
	for _, l := range lc {
		if l.call != nil && !l.left && k.endsBody(l.rest, owner) {
			ends = true
			break
		}
	}
	if !ends {
		return
	}
	for _, l := range lu {
		if l.call != nil {
			k.addRestart(restart{owner.rule, l.call, l.via})
		}
	}
}

// addRestart records r where nestsAt reads it while it computes, and where checkRestarts reads it
// afterwards.
func (k *classifier) addRestart(r restart) {
	if k.nestDry {
		k.restarts = append(k.restarts, r)
	} else {
		k.usedRestarts = append(k.usedRestarts, r)
	}
}

// endsBody reports whether the path c, which has not left the body of the rule of owner, can
// reach the end of that body without consuming input. The paths of an operand or an operator part
// end with the part, after which the Operators Rule can end.
func (k *classifier) endsBody(c *cont, owner *unit) bool {
	seen := map[string]bool{}
	var walk func(c *cont) bool
	walk = func(c *cont) bool {
		for ; c != nil; c = c.next {
			k.control.Poll()
			if _, null := k.firstItems(c.items); !null {
				return false
			}
			if c.next != nil {
				continue
			}
			switch {
			case c.end == endNone:
				return owner.kind == unitOperand || owner.kind == unitOperator
			case !c.inner:
				return true
			case seen[c.rule]:
				return false
			}
			// The end of a body that the path looked into: it continues after the call.
			seen[c.rule] = true
			found := false
			if c.end == endOps {
				k.opsLeads(c.rule, true, func(n *cont) { found = found || walk(n) })
			}
			k.endLeads(c.rule, nil, func(n *cont, _ *site) { found = found || walk(n) })
			return found
		}
		return true
	}
	return walk(c)
}

// nestSet returns the call sites that nest at one end, with the restart clause read from rs.
func (k *classifier) nestSet(rs []restart) map[*site]bool {
	// enders[r] are the calls of r from a rule of its recursion that what can match without
	// consuming input follows (E2).
	enders := map[string][]*site{}
	empty := map[*site]bool{}
	for _, s := range k.sites {
		if k.scc[s.caller] == k.scc[s.callee] && k.endsEmpty(s.cont) {
			enders[s.callee] = append(enders[s.callee], s)
			empty[s] = true
		}
	}
	// byRef[c] are the restarts through the call c itself or through its body.
	byRef := map[*grammar.Ref][]*restart{}
	for i := range rs {
		r := &rs[i]
		byRef[r.call] = append(byRef[r.call], r)
		for v := r.via; v != nil; v = v.next {
			byRef[v.ref] = append(byRef[v.ref], r)
		}
	}
	out := map[*site]bool{}
	for _, t := range k.sites {
		k.control.Poll()
		if !k.recursive[t.callee] || k.scc[t.caller] != k.scc[t.callee] {
			continue
		}
		in := map[string]bool{}
		var work []string
		add := func(r string) {
			if !in[r] && k.scc[r] == k.scc[t.caller] {
				in[r] = true
				work = append(work, r)
			}
		}
		if empty[t] {
			add(t.caller) // E1
		}
		for _, r := range byRef[t.ref] {
			add(r.rule) // E3 through t
		}
		for len(work) > 0 {
			r := work[len(work)-1]
			work = work[:len(work)-1]
			for _, s := range enders[r] {
				add(s.caller) // E2
			}
		}
		out[t] = in[t.callee]
	}
	return out
}

// checkRestarts turns into a reason each call that would nest at one end through the shared calls
// that the checks of the units relied on, where nestsAt did not find it: the checks that read
// nestsAt then relied on a bound that does not hold.
func (k *classifier) checkRestarts() {
	if len(k.usedRestarts) == 0 {
		return
	}
	k.computeNesting()
	all := k.nestSet(append(slices.Clip(k.restarts), k.usedRestarts...))
	for _, t := range k.sites {
		if all[t] && !k.nestSites[t] {
			k.reason(t.caller, exprPos(t.ref), "the evaluations of the call of %s at %s can nest without a bound and end at one position, through a unit that evaluates calls alike with a continuation, which the classifier found only after it relied on the call",
				t.callee, at(t.caller, exprPos(t.ref)))
		}
	}
}
