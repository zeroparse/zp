// Package astfacts provides conservative matching facts shared by the
// compiler and linter. Unknown facts never justify excluding a rule call.
package astfacts

import "zeroparse.dev/zp/pkg/grammar/ast"

// SucceedingRules finds ordinary, non-cyclic rules that cannot return a matching
// failure. A recursive call can read a failing growth seed even when its rule's
// body always succeeds, so cyclic rules are excluded. OperatorsRule calls are unknown.
func SucceedingRules(rules []*ast.RuleDef, cyclic map[string]bool) map[string]bool {
	return succeedingRules(rules, cyclic, nil)
}

// SucceedingRulesWithPoll also polls during fixed-point passes, rule visits and
// recursive proofs. The callback may panic to cancel; no partial map is returned.
func SucceedingRulesWithPoll(rules []*ast.RuleDef, cyclic map[string]bool, poll func()) map[string]bool {
	return succeedingRules(rules, cyclic, poll)
}

func succeedingRules(rules []*ast.RuleDef, cyclic map[string]bool, poll func()) map[string]bool {
	sure := map[string]bool{}
	for changed := true; changed; {
		if poll != nil {
			poll()
		}
		changed = false
		for _, r := range rules {
			if poll != nil {
				poll()
			}
			if !cyclic[r.Name] && !sure[r.Name] && alwaysSucceeds(r.Expr, sure, poll) {
				sure[r.Name], changed = true, true
			}
		}
	}
	return sure
}

// AlwaysSucceeds reports a sufficient proof that e cannot return a matching
// failure, at any input position. Rule-action errors and execution limits abort
// the parse instead of trying another alternative; they are not matching failure.
func AlwaysSucceeds(e ast.Expr, sure map[string]bool) bool {
	return alwaysSucceeds(e, sure, nil)
}

// AlwaysSucceedsWithPoll also polls while traversing expressions, cut scopes and
// attributes. The proof and its inputs are unchanged; callback panics propagate.
func AlwaysSucceedsWithPoll(e ast.Expr, sure map[string]bool, poll func()) bool {
	return alwaysSucceeds(e, sure, poll)
}

func alwaysSucceeds(e ast.Expr, sure map[string]bool, poll func()) bool {
	if poll != nil {
		poll()
	}
	switch e := e.(type) {
	case *ast.Group:
		return alwaysSucceeds(e.Expr, sure, poll)
	case *ast.Literal:
		return e.Value == ""
	case *ast.Top, *ast.Cut:
		return true
	case *ast.Ref:
		return e.Level == "" && sure[e.Name]
	case *ast.Seq:
		for _, it := range e.Items {
			if !alwaysSucceeds(it, sure, poll) {
				return false
			}
		}
		return true
	case *ast.Choice:
		for _, alt := range e.Alts {
			if alwaysSucceeds(alt, sure, poll) {
				return true
			}
			if hasLooseCut(alt, poll) {
				return false // It may fail after committing this choice.
			}
		}
	case *ast.Optional:
		return !hasLooseCut(e.Expr, poll) || alwaysSucceeds(e.Expr, sure, poll)
	case *ast.Repeat:
		if e.Max == 0 {
			return e.Min == 0
		}
		if e.Min == 0 {
			return !hasLooseCut(e.Expr, poll) || alwaysSucceeds(e.Expr, sure, poll)
		}
		// Repetition ends after its first empty iteration, so an always
		// succeeding body cannot prove larger minima.
		return e.Min == 1 && alwaysSucceeds(e.Expr, sure, poll)
	case *ast.And:
		return alwaysSucceeds(e.Expr, sure, poll)
	case *ast.Not:
		return alwaysFails(e.Expr, sure, poll)
	case *ast.Capture:
		return alwaysSucceeds(e.Expr, sure, poll)
	case *ast.Atomic:
		return alwaysSucceeds(e.Expr, sure, poll)
	case *ast.Discard:
		return alwaysSucceeds(e.Expr, sure, poll)
	case *ast.Stream:
		return alwaysSucceeds(e.Expr, sure, poll)
	case *ast.Recovery:
		return alwaysSucceeds(e.Expr, sure, poll)
	case *ast.Attributed:
		return alwaysSucceeds(e.Expr, sure, poll)
	}
	return false
}

func alwaysFails(e ast.Expr, sure map[string]bool, poll func()) bool {
	if poll != nil {
		poll()
	}
	switch e := e.(type) {
	case *ast.Group:
		return alwaysFails(e.Expr, sure, poll)
	case *ast.Bottom:
		return true
	case *ast.Not:
		return alwaysSucceeds(e.Expr, sure, poll)
	case *ast.And:
		return alwaysFails(e.Expr, sure, poll)
	case *ast.Seq:
		for _, it := range e.Items {
			if alwaysFails(it, sure, poll) {
				return true
			}
		}
	case *ast.Choice:
		for _, alt := range e.Alts {
			if !alwaysFails(alt, sure, poll) {
				return false
			}
		}
		return true
	case *ast.Capture:
		return alwaysFails(e.Expr, sure, poll)
	case *ast.Atomic:
		return alwaysFails(e.Expr, sure, poll)
	case *ast.Discard:
		return alwaysFails(e.Expr, sure, poll)
	case *ast.Stream:
		return alwaysFails(e.Expr, sure, poll)
	case *ast.Recovery:
		return false
	case *ast.Attributed:
		for _, at := range e.Attrs {
			if poll != nil {
				poll()
			}
			if at.Name == "recover" {
				return false
			}
		}
		return alwaysFails(e.Expr, sure, poll)
	}
	return false
}

// HasLooseCut reports a cut that can commit the enclosing choice, optional or
// repetition. Nested choices, optional expressions, repetitions, lookaheads and
// rule calls delimit cuts.
func HasLooseCut(e ast.Expr) bool {
	return hasLooseCut(e, nil)
}

// HasLooseCutWithPoll is HasLooseCut with polling at visited nodes and attributes.
// It does not cross the cut boundaries that the ordinary proof excludes.
func HasLooseCutWithPoll(e ast.Expr, poll func()) bool {
	return hasLooseCut(e, poll)
}

func hasLooseCut(e ast.Expr, poll func()) bool {
	if poll != nil {
		poll()
	}
	switch e := e.(type) {
	case *ast.Group:
		return hasLooseCut(e.Expr, poll)
	case *ast.Cut:
		return true
	case *ast.Seq:
		for _, it := range e.Items {
			if hasLooseCut(it, poll) {
				return true
			}
		}
	case *ast.Capture:
		return hasLooseCut(e.Expr, poll)
	case *ast.Atomic:
		return hasLooseCut(e.Expr, poll)
	case *ast.Discard:
		return hasLooseCut(e.Expr, poll)
	case *ast.Stream:
		return hasLooseCut(e.Expr, poll)
	case *ast.Recovery:
		return hasLooseCut(e.Expr, poll) || hasLooseCut(e.Skip, poll)
	case *ast.Attributed:
		if hasLooseCut(e.Expr, poll) {
			return true
		}
		for _, at := range e.Attrs {
			if poll != nil {
				poll()
			}
			if at.Name == "recover" {
				if skip, ok := at.Arg("skip"); ok && hasLooseCut(skip, poll) {
					return true // Recovery executes skip in the same cut scope.
				}
			}
		}
	}
	return false
}
