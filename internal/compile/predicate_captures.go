package compile

import (
	"slices"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/grammaranalysis"
)

// PredicateCaptures tells which captures of a rule body record their values in a program that
// builds no tree.
type PredicateCaptures struct {
	// Read holds the captures that a predicate can read (predicateCaptures).
	Read map[*grammar.Capture]bool
	// Names holds the capture names that predicates read. A capture of such a name that no
	// predicate can read records nothing but keeps its slot, so that a predicate which reads the
	// name where no capture of it can hold a value finds the slot unset, as in a parse.
	Names map[string]bool
}

// Records reports whether the capture records its value.
func (p *PredicateCaptures) Records(c *grammar.Capture) bool { return p.Read[c] }

// Slotted reports whether a capture that records nothing still has the slot of its name.
func (p *PredicateCaptures) Slotted(c *grammar.Capture) bool { return p.Names[c.Name] }

// predicateCaptures returns the captures of the expression that a predicate can read, and the
// names that predicates read. A predicate can read the captures of its scope of a name that it
// reads which can hold the value of that name when the predicate runs; a lambda parameter of the
// same name reads no capture. A capture made in another
// alternative, after the predicate, in another scope, or replaced by a later capture of the same
// name on every way to the predicate is not one of them.
func predicateCaptures(e grammar.Expr, control *cancel.Control) *PredicateCaptures {
	control.Poll()
	p := &PredicateCaptures{Read: map[*grammar.Capture]bool{}, Names: map[string]bool{}}
	walkExprControlled(e, func(x grammar.Expr) {
		if pr, ok := x.(*grammar.Predicate); ok {
			WalkTerm(pr.Term, func(t grammar.Term) {
				control.Poll()
				if cr, ok := t.(*grammar.CaptureRef); ok && !cr.Param {
					p.Names[cr.Name] = true
				}
			})
		}
	}, control)
	readCaptures(e, p.Read, control)
	return p
}

// readCaptures adds to read the captures of e that a predicate can read.
func readCaptures(e grammar.Expr, read map[*grammar.Capture]bool, control *cancel.Control) {
	if o, ok := e.(*grammar.OperatorsRule); ok {
		// The skip and each line of an Operators Rule have scopes of their own.
		reachingCaptures(o.Skip, nil, read, control)
		for _, op := range o.Operands {
			reachingCaptures(op.Expr, nil, read, control)
		}
		for _, l := range o.Levels {
			for _, op := range l.Operators {
				reachingCaptures(op.Expr, nil, read, control)
			}
		}
		return
	}
	reachingCaptures(e, nil, read, control)
}

// captureSet maps a capture name to the captures whose value it can hold at a point of a scope.
type captureSet map[string][]*grammar.Capture

// union returns the captures of a and b; it may return a or b itself.
func (a captureSet) union(b captureSet) captureSet {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}
	out := captureSet{}
	for name, cs := range a {
		out[name] = cs
	}
	for name, cs := range b {
		for _, c := range cs {
			if !slices.Contains(out[name], c) {
				out[name] = append(slices.Clip(out[name]), c)
			}
		}
	}
	return out
}

// reachingCaptures walks e entered with the captures before, adds to read the captures that the
// predicates in e can read, and returns the captures after e succeeds. Captures undone by
// backtracking do not survive, so after a choice only the captures of the alternative that
// succeeded hold, and a repetition element is a scope of its own.
func reachingCaptures(e grammar.Expr, before captureSet, read map[*grammar.Capture]bool, control *cancel.Control) captureSet {
	control.Poll()
	switch e := e.(type) {
	case nil:
		return before
	case *grammar.Predicate:
		WalkTerm(e.Term, func(t grammar.Term) {
			control.Poll()
			if cr, ok := t.(*grammar.CaptureRef); ok && !cr.Param {
				for _, c := range before[cr.Name] {
					read[c] = true
				}
			}
		})
		return before
	case *grammar.Capture:
		after := captureSet{}
		for name, cs := range reachingCaptures(e.Expr, before, read, control) {
			after[name] = cs
		}
		after[e.Name] = []*grammar.Capture{e}
		return after
	case *grammar.Seq:
		for _, it := range e.Items {
			before = reachingCaptures(it, before, read, control)
		}
		return before
	case *grammar.Choice:
		var after captureSet
		for _, alt := range e.Alts {
			after = after.union(reachingCaptures(alt, before, read, control))
		}
		return after
	case *grammar.Optional:
		return before.union(reachingCaptures(e.Expr, before, read, control))
	case *grammar.Repeat:
		reachingCaptures(e.Expr, nil, read, control)
		return before
	case *grammar.Not:
		reachingCaptures(e.Expr, before, read, control)
		return before
	case *grammar.Group:
		return reachingCaptures(e.Expr, before, read, control)
	case *grammar.And:
		return reachingCaptures(e.Expr, before, read, control)
	case *grammar.Atomic:
		return reachingCaptures(e.Expr, before, read, control)
	case *grammar.Discard:
		return reachingCaptures(e.Expr, before, read, control)
	case *grammar.Stream:
		return reachingCaptures(e.Expr, before, read, control)
	case *grammar.InlineAction:
		return reachingCaptures(e.Expr, before, read, control)
	case *grammar.Recovery:
		after := reachingCaptures(e.Expr, before, read, control)
		return after.union(reachingCaptures(e.Skip, before.union(after), read, control))
	case *grammar.Attributed:
		after := reachingCaptures(e.Expr, before, read, control)
		for _, at := range e.Attrs {
			for _, arg := range grammaranalysis.ExpressionArgsWithPoll(at, control.Poll) {
				after = after.union(reachingCaptures(arg, before.union(after), read, control))
			}
		}
		return after
	case *grammar.OperatorsRule:
		readCaptures(e, read, control)
		return before
	}
	return before
}
