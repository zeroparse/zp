package complexity

import "zeroparse.dev/zp/grammar"

// Values that hold one another. The size of a value that copies the value of a call on a
// recursion with its rule adds up over the levels of the recursion (condition 4 of
// spec/runtime/parsing.md), but it doubles over them only where the values hold one another: the
// value of the rule holds the copy, and the value of the call holds the value of the rule in turn,
// as values rather than as the text of the input that they match. A value **holds** the value of a
// call when it is that value, passes it on, or builds a string or a list from it by `+`, `text`,
// `concat`, `filter`, `map` or a fold, measured as the copying operations measure their operands:
// the characters of a string or the elements of a list.

// ruleValue reports whether the value of the inline action a in rule is the value of the rule: the
// rule has no action, and a is its expression or an alternative of it, through groups.
func (k *classifier) ruleValue(rule string, a *grammar.InlineAction) bool {
	r := k.byName[rule]
	if r == nil || r.Action != nil || isOperators(r.Expr) {
		return false
	}
	var visit func(e grammar.Expr) bool
	visit = func(e grammar.Expr) bool {
		switch x := e.(type) {
		case *grammar.Group:
			return visit(x.Expr)
		case *grammar.Atomic:
			return visit(x.Expr)
		case *grammar.Choice:
			for _, alt := range x.Alts {
				if visit(alt) {
					return true
				}
			}
		case *grammar.InlineAction:
			return x == a
		}
		return false
	}
	return visit(r.Expr)
}

// feeds reports whether the value of callee, measured in mode, holds the value of rule, directly
// or through the values of other rules that it holds, so that a value of rule that holds the value
// of a call of callee holds its own value of the level inside it. A call of rule itself does.
func (k *classifier) feeds(callee string, mode sizeMode, rule string) bool {
	if callee == rule {
		return true
	}
	key := feedKey{callee, rule, mode}
	if v, ok := k.feedsMemo[key]; ok {
		return v
	}
	seen := map[sizeKey]bool{}
	var visit func(x sizeKey) bool
	visit = func(x sizeKey) bool {
		if seen[x] {
			return false
		}
		seen[x] = true
		for _, y := range k.heldBy(x) {
			if y.rule == rule || visit(y) {
				return true
			}
		}
		return false
	}
	v := visit(sizeKey{callee, mode})
	k.feedsMemo[key] = v
	return v
}

type feedKey struct {
	callee, rule string
	mode         sizeMode
}

// heldBy returns the values of calls on a recursion with the rule of x that its value holds,
// computed once for each rule and mode.
func (k *classifier) heldBy(x sizeKey) []sizeKey {
	if v, ok := k.heldMemo[x]; ok {
		return v
	}
	k.control.Poll()
	r := k.byName[x.rule]
	var out []sizeKey
	tc := termCtx{rule: x.rule}
	switch {
	case r == nil:
	case isOperators(r.Expr):
		ops := r.Expr.(*grammar.OperatorsRule)
		var actions []grammar.Term
		for _, o := range ops.Operands {
			if o.Action != nil && !o.DefaultAction {
				actions = append(actions, o.Action)
			}
		}
		for _, l := range ops.Levels {
			for _, op := range l.Operators {
				if op.Action != nil && !op.DefaultAction {
					actions = append(actions, op.Action)
				}
			}
		}
		for _, a := range actions {
			out = append(out, k.heldCalls(a, tc, x.mode)...)
		}
	case r.Action != nil && !r.DefaultAction:
		out = k.heldCalls(r.Action, tc, x.mode)
	default:
		out = k.heldExprCalls(r.Expr, tc, x.mode)
	}
	k.heldMemo[x] = out
	return out
}

// heldCalls returns the values of calls on a recursion with the rule of tc that the value of t,
// measured in mode, holds.
func (k *classifier) heldCalls(t grammar.Term, tc termCtx, mode sizeMode) []sizeKey {
	switch t := t.(type) {
	case *grammar.TermGroup:
		return k.heldCalls(t.Term, tc, mode)
	case *grammar.Cast:
		return k.heldCalls(t.X, tc, mode)
	case *grammar.CaptureRef:
		if p, ok := tc.params[t.Name]; ok && (t.Param || tc.lambda) {
			var out []sizeKey
			for _, e := range p.elems {
				out = append(out, k.heldExprCalls(e, tc, mode)...)
			}
			return out
		}
		exprs := k.captures[tc.rule][t.Name]
		if len(exprs) == 0 && isOperators(k.byName[tc.rule].Expr) {
			// An operand of an Operators Rule is a value of the rule itself.
			return []sizeKey{{tc.rule, mode}}
		}
		var out []sizeKey
		for _, e := range exprs {
			out = append(out, k.heldExprCalls(e, tc, mode)...)
		}
		return out
	case *grammar.Member:
		exprs, _, ok := k.termExprs(t, tc)
		if !ok {
			return nil
		}
		var out []sizeKey
		for _, e := range exprs {
			out = append(out, k.heldExprCalls(e, tc, mode)...)
		}
		return out
	case *grammar.Binary:
		if t.Op == "+" && !k.isInt(t.L, tc) && !k.isInt(t.R, tc) {
			return append(k.heldCalls(t.L, tc, textSize), k.heldCalls(t.R, tc, textSize)...)
		}
	case *grammar.Call:
		var out []sizeKey
		switch t.Func {
		case "text":
			for _, a := range t.Args {
				out = append(out, k.heldCalls(a, tc, textSize)...)
			}
		case "concat":
			for _, a := range t.Args {
				out = append(out, k.heldCalls(a, tc, listSize)...)
			}
		case "map", "filter":
			// The elements of the list, or as many values of the lambda.
			if len(t.Args) > 0 && mode == listSize {
				out = k.heldCalls(t.Args[0], tc, listSize)
			}
		case "foldl", "foldr":
			if len(t.Args) > 0 {
				out = k.heldCalls(t.Args[0], tc, mode)
			}
			if lam, ok := lambdaArg(t, 2); ok {
				out = append(out, k.heldCalls(lam.Body, k.heldLambdaCtx(t, lam, tc), mode)...)
			}
		}
		return out
	}
	return nil
}

// heldLambdaCtx returns the context of the body of the lambda lam of the fold call: the parameter
// for the element of the list stands for the expressions whose values the elements are, and the
// accumulator for nothing, since the fold holds its initial value and the values of the lambda.
func (k *classifier) heldLambdaCtx(call *grammar.Call, lam *grammar.Lambda, tc termCtx) termCtx {
	params := map[string]param{}
	for n, p := range tc.params {
		params[n] = p
	}
	var elems []grammar.Expr
	if len(call.Args) > 1 {
		elems = k.elemExprs(call.Args[1], tc)
	}
	for j, n := range lam.Params {
		if j == 0 {
			params[n] = param{acc: true}
		} else {
			params[n] = param{elems: elems}
		}
	}
	tc.params, tc.lambda = params, true
	return tc
}

// heldExprCalls returns the values of calls on a recursion with the rule of tc that the value of a
// match of e, measured in mode, holds: a call passes its value on through choices, options and
// groups, and an inline action holds what its action holds. The node of a sequence holds the text
// of the input that it matches, which does not grow with the values of its calls.
func (k *classifier) heldExprCalls(e grammar.Expr, tc termCtx, mode sizeMode) []sizeKey {
	switch x := e.(type) {
	case *grammar.Group:
		return k.heldExprCalls(x.Expr, tc, mode)
	case *grammar.Capture:
		return k.heldExprCalls(x.Expr, tc, mode)
	case *grammar.Atomic:
		return k.heldExprCalls(x.Expr, tc, mode)
	case *grammar.Optional:
		return k.heldExprCalls(x.Expr, tc, mode)
	case *grammar.Choice:
		var out []sizeKey
		for _, alt := range x.Alts {
			out = append(out, k.heldExprCalls(alt, tc, mode)...)
		}
		return out
	case *grammar.InlineAction:
		if x.Action != nil && !x.DefaultAction {
			return k.heldCalls(x.Action, termCtx{rule: tc.rule}, mode)
		}
	case *grammar.Ref:
		if k.byName[x.Name] != nil && k.onRecursion(tc.rule, x.Name) {
			return []sizeKey{{x.Name, mode}}
		}
	}
	return nil
}
