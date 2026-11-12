package compile

import (
	"fmt"

	"zeroparse.dev/zp/grammar"
)

// --- Steps of a growth ---
//
// A step of a growth after the first evaluates the body of the leader again at the position where
// it grows, with its left calls reading the seed. A step that fails is dropped: the growth ends
// with the seed, and the parser goes on at the end of the seed with what follows the calls that
// started the growth. What the step examined after the seed can then be examined again, though
// no unit failed: the failure can lie after a scanning source in a part of the body that no unit
// holds, as in S = (S "a" / "b") T with T = "c" / "d" [a-z]* "!", where the source and the "!"
// that fails after it are in the last alternative of T, after the group that holds the growing
// unit S "a".
//
// The step is checked as a unit that starts at the end of the seed. Its paths are the step tails
// of the leader: what the body evaluates after a left call of a leader that reads the seed, made
// in the body or through left calls of other rules of the cycle, up to the end of the body. Its
// continuations are what follows the calls that start the growth, as for a growing unit. It is
// checked when a tail contains a scanning source and can fail after it.
//
// The unit is a unit of the set of calls that nest at one end too: collectRestarts reads its
// shared calls with each continuation. Its continuations follow the calls that start the growth,
// so they have always left the body of the leader, and the rule that a shared call adds is the
// one in whose body the continuation is at the shared call (recordShared).

// addStepUnits adds the unit of a step after the first of the growth of each leader whose step
// tails contain a scanning source and can fail after it. It runs before the units are checked
// and before the call sites that nest at one end are computed, which read the shared calls of
// the units.
func (k *classifier) addStepUnits() {
	for _, r := range k.rules {
		if !k.reach[r.Name] || !k.an.leaders[r.Name] {
			continue
		}
		cycle := k.an.cycle[r.Name]
		tails, ok := k.memberStarts(r.Name, &cont{end: endRule, rule: r.Name}, cycle)
		u := &unit{id: len(k.units), kind: unitStep, rule: r.Name, expr: r.Expr, pos: exprPos(r.Expr),
			self: tails, calls: map[string]bool{}, cycle: cycle}
		if !ok {
			// Without the tails nothing bounds what a dropped step examines; report it when a
			// rule of the cycle contains a scanning source.
			if src := k.cycleSource(cycle); src != nil {
				u.src, u.hasSource = src, true
				u.unsupported = "its body reaches left calls of its cycle through expressions that the classifier does not follow"
				k.units = append(k.units, u)
			}
			continue
		}
		var src *source
		for _, t := range tails {
			if s := k.tailFailsAfterSource(t); s != nil {
				src = s
				break
			}
		}
		if src == nil {
			continue
		}
		u.src, u.hasSource = src, true
		if u.conts, ok = k.growthStarts(cycle); !ok {
			u.unsupported = "its cycle grows from calls whose bodies the classifier does not follow"
		}
		for _, t := range tails {
			for c := t; c != nil; c = c.next {
				for _, it := range c.items {
					if it != nil {
						for _, n := range allCallsControlled(it, nil, k.control) {
							u.calls[n] = true
						}
					}
				}
			}
		}
		k.units = append(k.units, u)
	}
}

// tailFailsAfterSource returns a scanning source of the step tail t after which the tail can fail,
// up to the end of the body of the leader, or nil. As for a unit, the tail can fail after a
// source when one of its items can, or when an item that can fail follows one that contains a
// source.
func (k *classifier) tailFailsAfterSource(t *cont) *source {
	var items []grammar.Expr
	for c := t; c != nil; c = c.next {
		for _, it := range c.items {
			if it != nil {
				items = append(items, it)
			}
		}
	}
	for i, it := range items {
		if k.failsAfterSource(it) {
			return k.sourceIn(it)
		}
		if !k.containsSource(it) {
			continue
		}
		for _, later := range items[i+1:] {
			if !k.an.AlwaysSucceeds(later) {
				return k.sourceIn(it)
			}
		}
	}
	return nil
}

// sourceIn returns the first scanning source in e: one that the walk recorded at an expression
// in e, or the source of a rule that e calls.
func (k *classifier) sourceIn(e grammar.Expr) *source {
	var found *source
	WalkExpr(e, func(x grammar.Expr) {
		if found != nil {
			return
		}
		if k.srcExprs[x] {
			found = k.walkedAt(x)
			if found != nil {
				return
			}
		}
		if r, ok := x.(*grammar.Ref); ok && k.ruleSrc[r.Name] != nil {
			found = k.ruleSrc[r.Name]
		}
	})
	if found == nil {
		found = &source{pos: exprPos(e), what: k.show(e)}
	}
	return found
}

// walkedAt returns the source that the walk recorded at the expression x, or nil.
func (k *classifier) walkedAt(x grammar.Expr) *source {
	pos := exprPos(x)
	for _, r := range k.rules {
		for _, s := range k.walked[r.Name] {
			if s.pos == pos && (s.scan == nil || s.scan == x) {
				return s
			}
		}
	}
	return nil
}

// cycleSource returns a scanning source that a rule of the cycle contains, or nil.
func (k *classifier) cycleSource(cycle int) *source {
	for _, r := range k.rules {
		if k.reach[r.Name] && k.an.cyclic[r.Name] && k.an.cycle[r.Name] == cycle && k.ruleSrc[r.Name] != nil {
			return k.ruleSrc[r.Name]
		}
	}
	return nil
}

// checkStepUnit checks the unit of a step of a growth after the first. It starts at the end of
// the seed, where the parser goes on with what follows the calls that started the growth when the
// step fails.
//
// Each tail is compared with the continuations on its own, as each growing alternative is: a
// tail that examined input beyond its first character matched it, and a call that it shares with
// a continuation is answered at the end of the seed for both.
func (k *classifier) checkStepUnit(u *unit) {
	for _, self := range k.selfPaths(u) {
		if ok, what := k.committing(u, self, u.conts); !ok {
			if decidedUnitsOn {
				tail := *u
				tail.self = self
				if decided, _ := k.unitDecided(&tail, u.conts); decided {
					continue
				}
			}
			u.nc, u.witness = true, what
			return
		}
	}
}

// selfPaths returns the sets of paths of the unit that are compared with its continuations: each
// step tail on its own for the unit of a step, and the paths of the unit together for the others.
func (k *classifier) selfPaths(u *unit) [][]*cont {
	if u.kind != unitStep {
		return [][]*cont{u.self}
	}
	out := make([][]*cont, len(u.self))
	for i, t := range u.self {
		out[i] = []*cont{t}
	}
	return out
}

// stepMessage describes a step that is not committing.
func (k *classifier) stepMessage(u *unit) string {
	return fmt.Sprintf("%s of %s at %s can fail after %s and is not committing against %s",
		u.kind, u.rule, at(u.rule, u.pos), u.src, u.witness)
}
