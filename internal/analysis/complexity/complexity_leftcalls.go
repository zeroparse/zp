package complexity

import "zeroparse.dev/zp/pkg/grammar/ast"

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
func (k *classifier) startsWithLeftCall(rule string, e ast.Expr, mode leftMode) bool {
	found, _ := k.leftScan(rule, e, mode, 0)
	return found
}

// leftScan returns whether e can evaluate such a call, and whether e can complete before it
// examines (leftExamine) or consumes (leftConsume) input, so that what follows e is reached too.
func (k *classifier) leftScan(rule string, e ast.Expr, mode leftMode, depth int) (found, through bool) {
	k.control.Poll()
	if depth > 64 {
		return false, false
	}
	switch e := e.(type) {
	case nil:
		return false, true
	case *ast.Ref:
		if k.an.Cyclic[e.Name] && k.sameCycle(rule, e.Name) && e.Level == "" {
			return true, false
		}
		return false, k.passesBefore(e, mode)
	case *ast.Group:
		return k.leftScan(rule, e.Expr, mode, depth+1)
	case *ast.Capture:
		return k.leftScan(rule, e.Expr, mode, depth+1)
	case *ast.Discard:
		return k.leftScan(rule, e.Expr, mode, depth+1)
	case *ast.Atomic:
		return k.leftScan(rule, e.Expr, mode, depth+1)
	case *ast.Stream:
		return k.leftScan(rule, e.Expr, mode, depth+1)
	case *ast.Seq:
		through = true
		for _, it := range e.Items {
			f, t := k.leftScan(rule, it, mode, depth+1)
			found = found || f
			if !t {
				return found, false
			}
		}
		return found, through
	case *ast.Choice:
		for _, alt := range e.Alts {
			f, t := k.leftScan(rule, alt, mode, depth+1)
			found, through = found || f, through || t
		}
		return found, through
	case *ast.Optional:
		f, _ := k.leftScan(rule, e.Expr, mode, depth+1)
		return f, mode == leftConsume
	case *ast.Repeat:
		f, t := k.leftScan(rule, e.Expr, mode, depth+1)
		return f, mode == leftConsume && (e.Min == 0 || e.Max == 0 || t)
	case *ast.And, *ast.Not:
		return false, mode == leftConsume
	case *ast.Predicate, *ast.Cut, *ast.Top, *ast.BeginInput, *ast.EndInput,
		*ast.BeginLine, *ast.EndLine:
		return false, true
	case *ast.Literal:
		return false, e.Value == ""
	}
	return false, false
}

// passesBefore reports whether a call of a rule that is not on the cycle can match without
// examining the input (leftExamine) or consuming input (leftConsume).
func (k *classifier) passesBefore(e *ast.Ref, mode leftMode) bool {
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
	var nothing func(e ast.Expr) bool
	nothing = func(e ast.Expr) bool {
		switch e := e.(type) {
		case nil:
			return true
		case *ast.Ref:
			return e.Level == "" && k.examinesNothing(e.Name, seen, every)
		case *ast.Group:
			return nothing(e.Expr)
		case *ast.Capture:
			return nothing(e.Expr)
		case *ast.Discard:
			return nothing(e.Expr)
		case *ast.Atomic:
			return nothing(e.Expr)
		case *ast.Seq:
			for _, it := range e.Items {
				if !nothing(it) {
					return false
				}
			}
			return true
		case *ast.Choice:
			for _, alt := range e.Alts {
				if nothing(alt) != every {
					return !every
				}
			}
			return every
		case *ast.Predicate, *ast.Cut, *ast.Top, *ast.BeginInput, *ast.EndInput,
			*ast.BeginLine, *ast.EndLine:
			return true
		case *ast.Literal:
			return e.Value == ""
		}
		return false
	}
	return nothing(r.Expr)
}
