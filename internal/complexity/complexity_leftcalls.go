package complexity

import "zeroparse.dev/zp/grammar"

// leftMode says what may come before a left call of the unit that starts with it: in leftExamine,
// nothing that examines the input, and in leftConsume, nothing that consumes input.
type leftMode int

const (
	leftExamine leftMode = iota
	leftConsume
)

// startsWithLeftCall reports whether e can evaluate a call of a rule of the cycle of rule before
// it examines the input (leftExamine) or consumes input (leftConsume). Before such a call may come
// predicates, cuts, anchors and calls of rules off the cycle that can match without examining or
// consuming input; for leftConsume also parts that can match without consuming input, as an
// option does, and lookaheads. The decision errs toward the call: a part that might go on
// without examining or consuming input is looked through.
func (k *classifier) startsWithLeftCall(rule string, e grammar.Expr, mode leftMode) bool {
	found, _ := k.leftScan(rule, e, mode, 0)
	return found
}

// leftScan returns whether e can evaluate such a call, and whether e can complete before it
// examines (leftExamine) or consumes (leftConsume) input, so that what follows e is reached too.
func (k *classifier) leftScan(rule string, e grammar.Expr, mode leftMode, depth int) (found, through bool) {
	k.control.Poll()
	if depth > 64 {
		return false, false
	}
	switch e := e.(type) {
	case nil:
		return false, true
	case *grammar.Ref:
		if k.an.Cyclic[e.Name] && k.sameCycle(rule, e.Name) && e.Level == "" {
			return true, false
		}
		return false, k.passesBefore(e, mode)
	case *grammar.Group:
		return k.leftScan(rule, e.Expr, mode, depth+1)
	case *grammar.Capture:
		return k.leftScan(rule, e.Expr, mode, depth+1)
	case *grammar.Discard:
		return k.leftScan(rule, e.Expr, mode, depth+1)
	case *grammar.Atomic:
		return k.leftScan(rule, e.Expr, mode, depth+1)
	case *grammar.Stream:
		return k.leftScan(rule, e.Expr, mode, depth+1)
	case *grammar.Seq:
		through = true
		for _, it := range e.Items {
			f, t := k.leftScan(rule, it, mode, depth+1)
			found = found || f
			if !t {
				return found, false
			}
		}
		return found, through
	case *grammar.Choice:
		for _, alt := range e.Alts {
			f, t := k.leftScan(rule, alt, mode, depth+1)
			found, through = found || f, through || t
		}
		return found, through
	case *grammar.Optional:
		f, _ := k.leftScan(rule, e.Expr, mode, depth+1)
		return f, mode == leftConsume
	case *grammar.Repeat:
		f, t := k.leftScan(rule, e.Expr, mode, depth+1)
		return f, mode == leftConsume && (e.Min == 0 || e.Max == 0 || t)
	case *grammar.And, *grammar.Not:
		return false, mode == leftConsume
	case *grammar.Predicate, *grammar.Cut, *grammar.Top, *grammar.BeginInput, *grammar.EndInput,
		*grammar.BeginLine, *grammar.EndLine:
		return false, true
	case *grammar.Literal:
		return false, e.Value == ""
	}
	return false, false
}

// passesBefore reports whether a call of a rule that is not on the cycle can match without
// examining the input (leftExamine) or consuming input (leftConsume).
func (k *classifier) passesBefore(e *grammar.Ref, mode leftMode) bool {
	if e.Level != "" || k.byName[e.Name] == nil {
		return false
	}
	if mode == leftConsume {
		return k.nullRule[e.Name]
	}
	return k.examinesNothing(e.Name, map[string]bool{}, false)
}

// examinesNothing reports whether the rule can match without examining the input: some path of
// its body evaluates only predicates, empty literals, anchors and calls of such rules. With every
// set, whether every path does.
func (k *classifier) examinesNothing(name string, seen map[string]bool, every bool) bool {
	r := k.byName[name]
	if r == nil || seen[name] {
		return false
	}
	seen[name] = true
	defer delete(seen, name)
	var nothing func(e grammar.Expr) bool
	nothing = func(e grammar.Expr) bool {
		switch e := e.(type) {
		case nil:
			return true
		case *grammar.Ref:
			return e.Level == "" && k.examinesNothing(e.Name, seen, every)
		case *grammar.Group:
			return nothing(e.Expr)
		case *grammar.Capture:
			return nothing(e.Expr)
		case *grammar.Discard:
			return nothing(e.Expr)
		case *grammar.Atomic:
			return nothing(e.Expr)
		case *grammar.Seq:
			for _, it := range e.Items {
				if !nothing(it) {
					return false
				}
			}
			return true
		case *grammar.Choice:
			for _, alt := range e.Alts {
				if nothing(alt) != every {
					return !every
				}
			}
			return every
		case *grammar.Predicate, *grammar.Cut, *grammar.Top, *grammar.BeginInput, *grammar.EndInput,
			*grammar.BeginLine, *grammar.EndLine:
			return true
		case *grammar.Literal:
			return e.Value == ""
		}
		return false
	}
	return nothing(r.Expr)
}
