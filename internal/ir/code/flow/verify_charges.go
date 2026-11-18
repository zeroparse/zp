package flow

import (
	"fmt"
	"slices"

	"zeroparse.dev/zp/internal/ir/code"
)

// verifyCharges checks the budget-charging invariants of a function lowered with budget controls
// (Attrs.Budget), which keep the spend of a parse bounded by its limits and the context polled (the
// runtime polls every 256 units of spend). A charge is a step, a peek (which charges the spend of the
// character it reads), or a rule call (which the runtime charges as a rule call):
//
//   - the function charges before it consumes input: before an advance or a call of an operation in
//     opsConsume;
//   - every iteration of a loop without a counter charges: on each path from the start of the body to
//     a continue or to the end of the body. A repetition of a character class is lowered to a loop of
//     peek and advance without a step, so its iteration charges the spend of the character it reads,
//     which the budget bounds.
//
// A loop has a counter when it initializes one (Loop.Init). A function whose caller charged its entry
// (Attrs.Entered) starts charged. A path is followed through branches, blocks, exits and breaks; a path
// that ends in an exit or break leaves the block or loop it names with the charge state it has, and a
// return ends it.
func verifyCharges(fn *code.Func) error {
	w := &chargeVerifier{exits: map[string]*chargeExit{}}
	w.stmts(fn.Body.Stmts, AttrsOf(fn).Entered)
	if w.err != nil {
		return fmt.Errorf("spend: %w", w.err)
	}
	return nil
}

type chargeExit struct{ charged bool }

type chargeVerifier struct {
	exits map[string]*chargeExit // the state at the exits seen to each label so far (all charged)
	err   error
	loops []*code.Loop
}

func (w *chargeVerifier) fail(format string, args ...any) {
	if w.err == nil {
		w.err = fmt.Errorf(format, args...)
	}
}

// expr checks the expressions xs, evaluated in order, and returns the charge state after them.
func (w *chargeVerifier) expr(charged bool, xs ...code.Expr) bool {
	for _, x := range xs {
		if !charged && hasCall(x, func(c *code.Call) bool { return slices.Contains(opsConsume, c.Callee) }) {
			w.fail("an expression consumes input before a charge")
		}
		charged = charged || hasCall(x, func(c *code.Call) bool { return c.Callee == OpCallRule })
	}
	return charged
}

func (w *chargeVerifier) exit(to string, charged bool) {
	if e := w.exits[to]; e != nil {
		e.charged = e.charged && charged
	} else {
		w.exits[to] = &chargeExit{charged: charged}
	}
}

// leave returns the state after the block or loop label, whose end is reachable in the state end if
// live, and forgets the exits to it.
func (w *chargeVerifier) leave(label string, end bool, live bool) (bool, bool) {
	e := w.exits[label]
	delete(w.exits, label)
	switch {
	case live && e != nil:
		return end && e.charged, true
	case live:
		return end, true
	case e != nil:
		return e.charged, true
	}
	return end, false
}

// stmts walks ss from the state charged, and returns the state at their end and whether the end is
// reachable.
func (w *chargeVerifier) stmts(ss []code.Stmt, charged bool) (bool, bool) {
	for _, s := range ss {
		switch s := s.(type) {
		case *code.Do:
			switch s.Call.Callee {
			case OpStep:
				charged = true
			case OpAdvance:
				if !charged {
					w.fail("an advance before a charge")
				}
				charged = w.expr(charged, s.Call.Args...)
			}
		case *code.Set:
			if c := callOf(s.Values); c != nil && c.Callee == OpPeek {
				charged = true
				continue
			}
			charged = w.expr(charged, s.Values...)
		case *code.If:
			if s.Init != nil {
				charged = w.expr(charged, s.Init.Values...)
			}
			charged = w.expr(charged, s.Cond)
			tc, tl := w.stmts(s.Then.Stmts, charged)
			ec, el := charged, true
			if s.Else != nil {
				ec, el = w.stmts(s.Else.Stmts, charged)
			}
			switch {
			case tl && el:
				charged = tc && ec
			case tl:
				charged = tc
			case el:
				charged = ec
			default:
				return charged, false
			}
		case *code.Exit:
			w.exit(s.Label, charged)
			return charged, false
		case *code.Break:
			w.exit(s.Label, charged)
			return charged, false
		case *code.Return:
			return charged, false
		case *code.Continue:
			if l := w.loops[len(w.loops)-1]; l.Init == nil && !charged {
				w.fail("loop %s continues without charging in its iteration", l.Label)
			}
			return charged, false
		case *code.Block:
			bc, live := w.stmts(s.Stmts, charged)
			var ok bool
			if charged, ok = w.leave(s.Label, bc, live); !ok {
				return charged, false
			}
		case *code.Loop:
			pre := charged
			w.loops = append(w.loops, s)
			c := w.expr(false, s.Cond)
			bc, live := w.stmts(s.Body.Stmts, c)
			w.loops = w.loops[:len(w.loops)-1]
			if live && s.Init == nil && !bc {
				w.fail("loop %s ends an iteration without charging", s.Label)
			}
			e := w.exits[s.Label]
			delete(w.exits, s.Label)
			// After the loop, the state is the meet of the ways out: each exit (which has charged what the
			// loop entry and the iteration charged), and the end of a loop with a bound, which may come
			// before any iteration.
			bounded := s.Cond != nil
			switch {
			case e == nil && !bounded:
				return pre, false // a loop without exits does not end
			case e == nil, bounded:
				charged = pre
			default:
				charged = pre || e.charged
			}
		}
	}
	return charged, true
}
