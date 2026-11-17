package complexity

import (
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/frontend/astfacts"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// A unit restarts the input that a scanning source inside it examined only when it fails after
// the source has run. A unit whose sources are followed by nothing that can fail, such as the
// iteration of ([^"]+ / "\"\"")*, fails only before its sources examine any input; what the parser
// evaluates next then examines no more than the unit examined outside its sources, which the other
// conditions bound.

// containsSource reports whether e contains a scanning source, in itself or in a rule it calls.
func (k *classifier) containsSource(e ast.Expr) bool {
	if v, ok := k.srcMemo[e]; ok {
		return v
	}
	found := false
	astfacts.WalkExpr(e, func(x ast.Expr) {
		if found {
			return
		}
		if k.srcExprs[x] {
			found = true
			return
		}
		if r, ok := x.(*ast.Ref); ok && k.ruleSrc[r.Name] != nil {
			found = true
		}
	})
	k.srcMemo[e] = found
	return found
}

// computeFailRules computes, for each reachable rule, whether its body can fail after a scanning
// source in it examined input. The least fixpoint is the answer: a rule fails after a source only
// through an evaluation that ends.
func (k *classifier) computeFailRules() {
	k.failRule = map[string]bool{}
	for changed := true; changed; {
		changed = false
		for _, r := range k.rules {
			k.control.Poll()
			if !k.reach[r.Name] || k.failRule[r.Name] {
				continue
			}
			if k.failsAfterSource(r.Expr) {
				k.failRule[r.Name] = true
				changed = true
			}
		}
	}
}

// failsAfterSource reports whether e can fail after a scanning source in it examined input.
func (k *classifier) failsAfterSource(e ast.Expr) bool {
	switch x := e.(type) {
	case nil:
		return false
	case *ast.Seq:
		for i, it := range x.Items {
			if k.failsAfterSource(it) {
				return true
			}
			if k.containsSource(it) {
				for _, later := range x.Items[i+1:] {
					if !k.an.AlwaysSucceeds(later) {
						return true
					}
				}
			}
		}
		return false
	case *ast.Choice:
		for _, alt := range x.Alts {
			if k.failsAfterSource(alt) {
				return true
			}
		}
		return false
	case *ast.Repeat:
		if x.Min == 0 || x.Max == 0 {
			return false
		}
		// A repetition ends at its first empty iteration, so one with a minimum above one can
		// fail after an iteration ran a source even when its expression always succeeds.
		return k.failsAfterSource(x.Expr) || x.Min > 1 && k.containsSource(x.Expr) && !k.an.AlwaysSucceeds(x)
	case *ast.Optional:
		return false
	case *ast.And:
		return k.failsAfterSource(x.Expr)
	case *ast.Not:
		return k.containsSource(x.Expr)
	case *ast.Group:
		return k.failsAfterSource(x.Expr)
	case *ast.Capture:
		return k.failsAfterSource(x.Expr)
	case *ast.Discard:
		return k.failsAfterSource(x.Expr)
	case *ast.Atomic:
		return k.failsAfterSource(x.Expr)
	case *ast.Stream:
		return k.failsAfterSource(x.Expr)
	case *ast.InlineAction:
		return k.failsAfterSource(x.Expr)
	case *ast.Ref:
		if k.srcExprs[x] {
			return true
		}
		return k.failRule[x.Name]
	case *ast.Predicate:
		return k.srcExprs[x]
	case *ast.Attributed:
		if compile.RecoverSkip(x) != nil || compile.HasPragma(x, "missing") {
			return k.containsSource(x)
		}
		return k.failsAfterSource(x.Expr)
	case *ast.Literal, *ast.CharClass, *ast.Any, *ast.Cut, *ast.Top, *ast.Bottom,
		*ast.BeginInput, *ast.EndInput, *ast.BeginLine, *ast.EndLine:
		return false
	}
	return k.containsSource(e)
}

// unitFailsAfterSource reports whether the unit can fail after a scanning source inside it
// examined input. Lookaheads are abandoned whether they match or not, and the units of an Operators
// Rule and growing units are not refined.
func (k *classifier) unitFailsAfterSource(u *unit) bool {
	switch u.kind {
	case unitAlt:
		if u.growing {
			return true
		}
		return k.failsAfterSource(u.expr)
	case unitIteration:
		return k.failsAfterSource(u.expr.(*ast.Repeat).Expr)
	case unitOption:
		return k.failsAfterSource(u.expr.(*ast.Optional).Expr)
	}
	return true
}
