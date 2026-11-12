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
// What follows a left call is evaluated again in each step of a growth, at the end of the seed,
// so a path that continues at a left call past the end of a body is evaluated once for each
// evaluation of the cycle that ends there; nestsEnd counts those as nesting when a call of a rule
// of the cycle nests at one end.
//
// The calls that units evaluate alike come from the analyses that rely on them: the shared calls
// of strip and the calls that the exclusion analysis passes alike. nestsAt computes the set from
// the shared calls of the units of a recursion that do not grow against each of their
// continuations before the checks, without any call nesting (so without the shortening that
// nesting causes, which can only remove shared calls). The checks record the calls that they
// relied on, and checkRestarts turns any that would make another call nest into a reason.

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

// nestsEnd reports whether what follows the call s can be evaluated at one position once for each
// of unboundedly many evaluations of its callee that end there: s nests at one end, or s is a left
// call in a left-recursion cycle (a seed read) and the evaluations of the rules of that cycle can
// nest and end at one position: a call of a rule of the cycle nests at one end. Each evaluation of
// a leader of the cycle that ends at a position grows there: the next step of its growth evaluates
// what follows the left calls of the cycle at that position again, after its seed.
func (k *classifier) nestsEnd(s *site) bool {
	if s == nil {
		return false
	}
	if k.nestsAt(s) {
		return true
	}
	if !s.mayStart || !k.an.cyclic[s.callee] || !k.sameCycle(s.caller, s.callee) || k.nestDry {
		return false
	}
	k.computeNesting()
	return k.nestCycles[k.an.cycle[s.callee]]
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
		k.nestCycles = map[int]bool{}
		for t, nests := range k.nestSites {
			if nests && k.an.cyclic[t.callee] {
				k.nestCycles[k.an.cycle[t.callee]] = true
			}
		}
	}
}

// collectRestarts records the shared calls of each unit of a recursion that does not grow against
// each of its continuations. The operator parts that the enclosing levels of an Operators Rule
// try again become a continuation only after this pass, when the rule nests (addRetried), so like
// the paths below they come from the checks. The checks compare growing units, and units whose
// continuations leave out the paths that re-enter a cycle, against other paths; they record the
// shared calls of those themselves, and checkRestarts reports any that this pass did not find.
// A unit that can fail only before its scanning sources restarts no scan, but it is read too:
// leaving it out saves nothing.
func (k *classifier) collectRestarts() {
	for _, u := range k.units {
		k.control.Poll()
		if u.growing || !k.recursive[u.rule] || !u.hasSource || u.unsupported != "" || !k.callsRecursion(u) {
			continue
		}
		for _, self := range k.selfPaths(u) {
			for _, cs := range u.conts {
				k.strip(self, cs.paths, u)
			}
		}
	}
}

// callsRecursion reports whether the unit calls a rule of the recursion of its rule. A restart
// needs a call of the recursion among the calls that the unit evaluates alike or looks through,
// and the bodies of other rules call no rule of the recursion, or they would be in it.
func (k *classifier) callsRecursion(u *unit) bool {
	for n := range u.calls {
		if k.scc[n] == k.scc[u.rule] {
			return true
		}
	}
	return false
}

// recordShared records a shared call of owner, a unit, and its continuation (see restart). lu are
// the leads of the unit at the call, lc those of the continuation. The restart is of each rule
// whose body the continuation can end after the call: the rule of owner, or, for a continuation
// that left that body before the call, the rule of the body that it reached, whose evaluation
// contains the one of the rule of owner.
func (k *classifier) recordShared(owner *unit, lu, lc []lead) {
	if owner == nil || !k.recursive[owner.rule] {
		return
	}
	var rules []string
	for _, l := range lc {
		if l.call == nil {
			continue
		}
		k.bodyEnds(l.rest, owner, func(r string) {
			if !slices.Contains(rules, r) {
				rules = append(rules, r)
			}
		})
	}
	for _, r := range rules {
		for _, l := range lu {
			if l.call != nil {
				k.addRestart(restart{r, l.call, l.via})
			}
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

// bodyEnds calls end with the rule of each body whose end the path c can reach without consuming
// input: the body of the rule of owner, or the body of a rule that the path reached after it left
// that one. The first is the body in which the path is at its start, the one the restart needs.
// For the continuation of a growth that starts at a call of a rule of the cycle other than the
// leader, that is the body of that rule (bodyThen), not the one of its caller: the evaluations
// that the growth restarts are the ones of that call. The bodies that the path reaches after it
// are reported too; the second clause adds those rules anyway within the recursion. The paths of
// an operand or an operator part end with the part, after which the Operators Rule of owner can
// end.
func (k *classifier) bodyEnds(c *cont, owner *unit, end func(rule string)) {
	seen := map[string]bool{}
	var walk func(c *cont)
	walk = func(c *cont) {
		for ; c != nil; c = c.next {
			k.control.Poll()
			if _, null := k.firstItems(c.items); !null {
				return
			}
			if c.next != nil {
				if c.end != endNone {
					// The end of a body that the path entered at a call, after which it goes
					// on after the call.
					end(c.rule)
				}
				continue
			}
			switch {
			case c.end == endNone:
				if owner.kind == unitOperand || owner.kind == unitOperator {
					end(owner.rule)
				}
				return
			case !c.inner:
				end(c.rule)
				return
			case seen[c.rule]:
				return
			}
			// The end of a body that the path looked into: it continues after the call.
			seen[c.rule] = true
			if c.end == endOps {
				k.opsLeads(c.rule, true, walk)
			}
			k.endLeads(c.rule, func(n *cont, _ *site) { walk(n) })
			return
		}
		end(owner.rule)
	}
	walk(c)
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
