package compile

import (
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/frontend/check"

	"zeroparse.dev/zp/pkg/grammar/ast"
)

// Projected repetitions.
//
// first:x rest:(-"," r:x)* -> concat(list($first), map($rest, (e) => e.r)) is how lists are
// written: the repetition builds a record (a tuple node with the field r) per element, only for the
// action to take r out of each. When the action reads the capture only that way, every backend
// compiles the repetition to gather the values of r directly, and the map call to take the list
// as it is (both produce a list of the same values; the list the repetition makes is visible to the
// action alone).

// ruleProjectionsControlled returns the captures of repetitions that the rule r's action reads
// only as map($x, (e) => e.$f), mapped to the capture f, polling control. The conditions:
//   - no predicate reads $x, and the names x and f are captured only once in the rule;
//   - f is captured in every element (an item of the element's sequence), with a value that is
//     never nil (nonNil): otherwise an element can have no record, and $e.$f would be an error.
func ruleProjectionsControlled(prog *Program, r *Rule, control *cancel.Control) map[string]string {
	control.Poll()
	if prog.NoProjections || r.Action == nil {
		return nil
	}
	if _, operatorsRule := r.Def.Expr.(*ast.OperatorsRule); operatorsRule {
		return nil
	}
	cands := map[string]*ast.Repeat{}
	var walk func(e ast.Expr)
	walk = func(e ast.Expr) {
		control.Poll()
		switch e := e.(type) {
		case *ast.Capture:
			if rp, ok := e.Expr.(*ast.Repeat); ok && hasCapturesControlled(rp.Expr, control) {
				cands[e.Name] = rp
				return
			}
			walk(e.Expr)
		case *ast.Seq:
			for _, it := range e.Items {
				control.Poll()
				walk(it)
			}
		case *ast.Choice:
			for _, a := range e.Alts {
				control.Poll()
				walk(a)
			}
		case *ast.Optional:
			walk(e.Expr)
		case *ast.Attributed:
			if !IsStream(e) {
				walk(e.Expr)
			}
		case *ast.Repeat:
			if !hasCapturesControlled(e.Expr, control) {
				walk(e.Expr)
			}
		case *ast.And:
			walk(e.Expr)
		}
	}
	walk(r.Def.Expr)
	if len(cands) == 0 {
		return nil
	}
	count := map[string]int{}
	walkExprControlled(r.Def.Expr, func(x ast.Expr) {
		switch x := x.(type) {
		case *ast.Capture:
			count[x.Name]++
		case *ast.Predicate:
			WalkTerm(x.Action, func(t ast.ActionExpr) {
				control.Poll()
				if c, ok := t.(*ast.CaptureRef); ok {
					delete(cands, c.Name)
				}
			})
		}
	}, control)
	fields := map[string]string{}
	bad := map[string]bool{}
	var use func(t ast.ActionExpr, shadow map[string]bool)
	use = func(t ast.ActionExpr, shadow map[string]bool) {
		control.Poll()
		switch t := t.(type) {
		case *ast.CaptureRef:
			if !shadow[t.Name] {
				bad[t.Name] = true
			}
		case *ast.Call:
			if t.Func == "map" && len(t.Args) == 2 {
				if x, ok := t.Args[0].(*ast.CaptureRef); ok && !shadow[x.Name] {
					if f, ok := projField(t.Args[1]); ok && (fields[x.Name] == "" || fields[x.Name] == f) {
						fields[x.Name] = f
						return
					}
				}
			}
			for _, a := range t.Args {
				control.Poll()
				use(a, shadow)
			}
		case *ast.Lambda:
			inner := map[string]bool{}
			for k := range shadow {
				control.Poll()
				inner[k] = true
			}
			for _, p := range t.Params {
				control.Poll()
				inner[p] = true
			}
			use(t.Body, inner)
		case *ast.Member:
			use(t.X, shadow)
		case *ast.New:
			for _, f := range t.Fields {
				control.Poll()
				use(f.Value, shadow)
			}
			for _, x := range t.Args {
				control.Poll()
				use(x, shadow)
			}
		case *ast.ListConstructor:
			for _, item := range t.Items {
				control.Poll()
				use(item, shadow)
			}
		case *ast.TupleLit:
			for _, x := range t.Elems {
				control.Poll()
				use(x, shadow)
			}
		case *ast.Binary:
			use(t.L, shadow)
			use(t.R, shadow)
		case *ast.Unary:
			use(t.X, shadow)
		case *ast.Cast:
			use(t.X, shadow)
		}
	}
	use(r.Action, map[string]bool{})
	proj := map[string]string{}
	for name, rp := range cands {
		control.Poll()
		f := fields[name]
		if f == "" || bad[name] || count[name] > 1 || count[f] > 1 {
			continue
		}
		if c := directCapture(rp.Expr, f, control); c != nil && nonNil(prog, c.Expr, map[string]bool{}, control) {
			proj[name] = f
		}
	}
	return proj
}

// projField reports whether the lambda is (e) => e.$f, or e.f in earlier syntax, and returns f.
func projField(t ast.ActionExpr) (string, bool) {
	l, ok := t.(*ast.Lambda)
	if !ok || len(l.Params) != 1 {
		return "", false
	}
	m, ok := l.Body.(*ast.Member)
	if !ok {
		return "", false
	}
	x, ok := m.X.(*ast.CaptureRef)
	if !ok || x.Name != l.Params[0] {
		return "", false
	}
	return check.ProjectionField(m.Name)
}

// directCapture returns the capture of name that e makes unconditionally: e itself, or an item of
// the sequence e (also behind @error, which changes no value).
func directCapture(e ast.Expr, name string, control *cancel.Control) *ast.Capture {
	control.Poll()
	if a, ok := e.(*ast.Attributed); ok {
		for _, at := range a.Attrs {
			control.Poll()
			if at.Name != "error" {
				return nil
			}
		}
		return directCapture(a.Expr, name, control)
	}
	switch e := e.(type) {
	case *ast.Capture:
		if e.Name == name {
			return e
		}
	case *ast.Seq:
		for _, it := range e.Items {
			control.Poll()
			if c, ok := it.(*ast.Capture); ok && c.Name == name {
				return c
			}
		}
	}
	return nil
}

// nonNil reports whether the value of e, built as a capture builds it, is never nil. A rule's
// value is never nil when its action makes a struct or returns a
// capture whose value is never nil, or when it has no action and its body's value is never nil
// (without the type checker's results, which the engine computes after compiling).
func nonNil(prog *Program, e ast.Expr, seen map[string]bool, control *cancel.Control) bool {
	control.Poll()
	switch e := e.(type) {
	case *ast.Literal, *ast.CharClass, *ast.Any, *ast.Atomic, *ast.Top, *ast.Seq, *ast.Repeat:
		return true
	case *ast.Capture:
		return nonNil(prog, e.Expr, seen, control)
	case *ast.Choice:
		for _, a := range e.Alts {
			control.Poll()
			if !nonNil(prog, a, seen, control) {
				return false
			}
		}
		return true
	case *ast.Attributed:
		return nonNil(prog, e.Expr, seen, control) // #recover makes an Error node
	case *ast.Ref:
		r := prog.ByName[e.Name]
		if r == nil || r.Def == nil {
			return false
		}
		if seen[e.Name] {
			// A value is built by a finite derivation: a rule that reaches itself yields nil only
			// if one of its other cases does, which the check of those cases finds.
			return true
		}
		seen[e.Name] = true
		defer delete(seen, e.Name)
		if pr, ok := r.Def.Expr.(*ast.OperatorsRule); ok {
			switch t := r.Def.Action.(type) {
			case nil:
			case *ast.New, *ast.ListConstructor, *ast.TupleLit:
				return true
			case *ast.CaptureRef:
				if t.Name != OperatorsRuleValue {
					return false
				}
				// $value is the folded value, which the lines below make.
			default:
				return false
			}
			// Every line's value is never nil ($lhs and $rhs are folded values).
			for _, o := range pr.Operands {
				control.Poll()
				if !lineNonNil(prog, o.Expr, o.Action, false, seen, control) {
					return false
				}
			}
			for _, l := range pr.Levels {
				control.Poll()
				for _, op := range l.Operators {
					control.Poll()
					if op.Action != nil && !lineNonNil(prog, op.Expr, op.Action, true, seen, control) {
						return false // (an operator without an action makes a tuple node)
					}
				}
			}
			return true
		}
		return lineNonNil(prog, r.Def.Expr, r.Action, false, seen, control)
	}
	return false
}

// lineNonNil reports whether a rule body or Operators Rule line e with the action t never has a nil value.
func lineNonNil(prog *Program, e ast.Expr, t ast.ActionExpr, operator bool, seen map[string]bool, control *cancel.Control) bool {
	control.Poll()
	switch t := t.(type) {
	case nil:
		return nonNil(prog, e, seen, control)
	case *ast.New, *ast.ListConstructor, *ast.TupleLit:
		return true
	case *ast.CaptureRef:
		if operator && (t.Name == "lhs" || t.Name == "rhs" || t.Name == "op") {
			return true
		}
		c := directCapture(e, t.Name, control)
		return c != nil && nonNil(prog, c.Expr, seen, control)
	}
	return false
}

// RuleProjections returns the projected repetition captures of r (ruleProjectionsControlled).
func RuleProjections(prog *Program, r *Rule) map[string]string {
	return ruleProjectionsControlled(prog, r, nil)
}
