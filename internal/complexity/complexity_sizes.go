package complexity

import (
	"fmt"
	"strings"
	"zeroparse.dev/zp/internal/grammaranalysis"

	"zeroparse.dev/zp/grammar"
)

// Size degrees of values. The bounds on folds and lambdas take the size of a value from the
// degree, in the size n of the input, that the classifier knows for it: a capture, the value of a
// rule call, an initial value or a list built in the action. A value is **own** when its size is
// at most a constant times one more than the input that its match consumes, so that the values of
// distinct matches, such as the elements of a repetition, add up to at most a constant times n.
// The size of any other value is bounded by composing the forms of growthOf over the rules it
// reads; a rule whose value includes the value of a call on a recursion with it and that is not
// own has no degree that the classifier knows (see condition 4 of spec/runtime/parsing.md).

// unknownDeg is a degree that stands for a size the classifier does not bound by a polynomial.
const unknownDeg = 1000

// addDeg adds two degrees, keeping an unknown degree unknown.
func addDeg(a, b int) int { return min(a+b, unknownDeg) }

// sizeMemo holds the size degrees of the values of rules, computed on demand.
type sizeMemo struct {
	own    map[string]bool            // the rules whose values are own
	look   map[string]map[string]bool // the names captured inside a positive lookahead, by rule
	looked map[string]bool            // the rules whose values can hold a capture in a lookahead
	looks  map[sizeKey]bool           // the rules whose sizes, by mode, can come from such a capture
	shapes map[string]growthShape     // the shapes of the values of rules
	busy   map[string]int             // the computation that bounds each rule being computed
	cur    int                        // the innermost computation, 0 outside any
	next   int                        // the number of computations started
	hit    bool                       // the innermost computation read the shape of a busy rule
}

var (
	// ownShape is the shape of an own value, and of a list of own elements.
	ownShape = func() growthShape {
		el := sized(outsideLin)
		return growthShape{S: outsideLin, N: outsideLin, El: &el}
	}()
	// unknownShape is the shape of a value whose size the classifier does not bound.
	unknownShape = func() growthShape {
		u := lin{{o: unknownDeg}: 1}
		el := sized(u)
		return growthShape{S: u, N: u, El: &el}
	}()
)

// degOf returns the degree in the size of the input of the form x of a value outside a fold: the
// largest power of n in it, or unknownDeg when it depends on the quantities of an accumulator,
// which no fold binds there. A power of Y counts for nothing: outside a fold there is no element
// of a step, and the forms that read one bound what the element adds by the other terms.
func degOf(x lin) int {
	d := 0
	for m := range x {
		if m.x != [vars]int8{} {
			return unknownDeg
		}
		d = max(d, int(m.o))
	}
	return d
}

// shapeDegs returns the degrees of the size, the size of the largest element and the number of
// elements of the value of shape x.
func shapeDegs(x growthShape) [3]int { return [3]int{degOf(x.S), degOf(x.m()), degOf(x.N)} }

// degShape returns the shape of a value whose size, largest element and number of elements have
// the degrees d.
func degShape(d [3]int) growthShape {
	el := sized(lin{{o: int16(d[1])}: 1})
	return growthShape{S: lin{{o: int16(d[0])}: 1}, N: lin{{o: int16(d[2])}: 1}, El: &el}
}

// valueShape bounds the value of t in the context tc, outside the steps of a fold.
func (k *classifier) valueShape(t grammar.Term, tc termCtx) growthShape {
	g, _ := k.growthOf(t, nil, tc)
	return g
}

// outsideShape bounds the value of a capture or a lambda parameter t that a term reads from
// outside the lambdas whose parameters growthOf follows.
func (k *classifier) outsideShape(t *grammar.CaptureRef, tc termCtx) growthShape {
	if p, ok := tc.params[t.Name]; ok && (t.Param || tc.lambda) {
		if p.int {
			return growthShape{}
		}
		return p.shape
	}
	return k.captureShape(tc.rule, t.Name)
}

// captureShape bounds the value of the capture name in rule. The operand values that an Operators
// Rule passes to its actions are values of the rule itself.
func (k *classifier) captureShape(rule, name string) growthShape {
	exprs := k.captures[rule][name]
	if len(exprs) == 0 {
		if r := k.byName[rule]; r != nil && isOperators(r.Expr) {
			return k.ruleShape(rule)
		}
		return unknownShape
	}
	var out growthShape
	for i, e := range exprs {
		s := k.exprShape(e, rule)
		if i == 0 {
			out = s
		} else {
			out = out.join(s)
		}
	}
	return out
}

// exprShape bounds the value of a match of e in rule. A repetition is a list with one element for
// each iteration; a call has the value of its rule; any other match is a node, whose text is part
// of the input and whose members are the values of the captures, calls and repetitions in it.
func (k *classifier) exprShape(e grammar.Expr, rule string) growthShape {
	k.control.Poll()
	if k.ownExpr(e, rule, k.ownRules()) {
		return ownShape
	}
	switch x := e.(type) {
	case *grammar.Group:
		return k.exprShape(x.Expr, rule)
	case *grammar.Capture:
		return k.exprShape(x.Expr, rule)
	case *grammar.Atomic:
		return k.exprShape(x.Expr, rule)
	case *grammar.Discard:
		return k.exprShape(x.Expr, rule)
	case *grammar.Optional:
		return k.exprShape(x.Expr, rule)
	case *grammar.Choice:
		out := growthShape{S: lin{}, N: lin{}}
		for _, alt := range x.Alts {
			out = out.join(k.exprShape(alt, rule))
		}
		return out
	case *grammar.Repeat:
		el := k.exprShape(x.Expr, rule)
		if x.Max >= 0 {
			return growthShape{S: constLin(x.Max + 1).plus(el.S.times(constLin(x.Max))), N: constLin(x.Max), El: &el}
		}
		return growthShape{S: outsideLin.plus(el.S.times(outsideLin)), N: outsideLin, El: &el}
	case *grammar.Ref:
		if k.byName[x.Name] == nil {
			return ownShape
		}
		return k.ruleShape(x.Name)
	}
	out := ownShape
	grammaranalysis.WalkExpr(e, func(y grammar.Expr) {
		if y == e {
			return
		}
		switch y := y.(type) {
		case *grammar.Capture, *grammar.Ref, *grammar.Repeat:
			out = out.join(k.exprShape(y, rule))
		case *grammar.InlineAction:
			if y.Action != nil && !y.DefaultAction {
				out = out.join(k.valueShape(y.Action, termCtx{rule: rule}))
			}
		}
	})
	return out
}

// ruleShape bounds the value of a call of the rule name. The rules of a recursion whose values are
// not own are bounded together: what one level of the recursion adds, with the values of the calls
// on the recursion left out, has some degree d, and a value that holds the value of each such call
// at most once is a tree of at most a constant times n levels, which adds up to degree d + 1. A
// value that can hold the value of such a call twice can double over the levels, and has no shape
// that the classifier knows.
func (k *classifier) ruleShape(name string) growthShape {
	if k.ownRules()[name] {
		return ownShape
	}
	m := &k.sizes
	if s, ok := m.shapes[name]; ok {
		return s
	}
	if m.shapes == nil {
		m.shapes, m.busy = map[string]growthShape{}, map[string]int{}
	}
	if c := m.busy[name]; c != 0 {
		// The value of a call on the recursion, which the levels add up. A rule that a computation
		// reaches and that reaches back to a busy rule is on a recursion with it, so in its SCC and
		// busy in the same computation, unless its value is own and it never starts one: a busy
		// rule is only ever read by the innermost computation, which owns the hit.
		if c != m.cur {
			panic(fmt.Sprintf("internal error: the shape of rule %s is read outside the computation of its recursion", name))
		}
		m.hit = true
		return growthShape{}
	}
	members := []string{name}
	if k.recursive[name] {
		members = nil
		for _, r := range k.rules {
			if k.scc[r.Name] == k.scc[name] && !k.ownRules()[r.Name] {
				members = append(members, r.Name)
			}
		}
	}
	m.next++
	savedCur, savedHit := m.cur, m.hit
	m.cur, m.hit = m.next, false
	for _, n := range members {
		m.busy[n] = m.cur
	}
	level := growthShape{S: lin{}, N: lin{}}
	shapes := map[string]growthShape{}
	for _, n := range members {
		shapes[n] = k.levelShape(k.byName[n])
		level = level.join(shapes[n])
	}
	// The hits of this computation are its own: no rule it reads reaches a rule that an outer
	// computation holds busy, so restoring the outer hit loses none of them.
	hit := m.hit
	m.cur, m.hit = savedCur, savedHit
	for _, n := range members {
		delete(m.busy, n)
	}
	for _, n := range members {
		switch {
		case !hit:
			m.shapes[n] = shapes[n]
		case k.recOnce(members):
			d := shapeDegs(level)
			m.shapes[n] = degShape([3]int{addDeg(d[0], 1), addDeg(d[1], 1), addDeg(d[2], 1)})
		default:
			m.shapes[n] = unknownShape
		}
	}
	return m.shapes[name]
}

// levelShape bounds the value of a call of r, with the values of the calls of the rules whose
// shapes are being computed left out.
func (k *classifier) levelShape(r *grammar.RuleDef) growthShape {
	if ops, ok := r.Expr.(*grammar.OperatorsRule); ok {
		out := growthShape{S: lin{}, N: lin{}}
		add := func(e grammar.Expr, action grammar.Term, def bool) {
			if action != nil && !def {
				out = out.join(k.valueShape(action, termCtx{rule: r.Name}))
			} else {
				out = out.join(k.exprShape(e, r.Name))
			}
		}
		for _, o := range ops.Operands {
			add(o.Expr, o.Action, o.DefaultAction)
		}
		for _, l := range ops.Levels {
			for _, op := range l.Operators {
				add(op.Expr, op.Action, op.DefaultAction)
			}
		}
		return out
	}
	if r.Action != nil && !r.DefaultAction {
		return k.valueShape(r.Action, termCtx{rule: r.Name})
	}
	return k.exprShape(r.Expr, r.Name)
}

// recOnce reports whether the value of each of the rules of a recursion holds the value of each
// call on the recursion at most once: no capture of such a call is read twice, read inside a
// lambda or as the list of a lambda, which adds to each of its elements, or made in a positive
// lookahead, where it can be the value that another capture holds too.
func (k *classifier) recOnce(rules []string) bool {
	for _, name := range rules {
		r := k.byName[name]
		w := &ownWalk{k: k, rule: name, rec: map[string]int{}, ok: true}
		look := k.lookCaptures(name)
		for n := range look {
			if w.recCapture(n) {
				return false
			}
		}
		var terms []grammar.Term
		if ops, ok := r.Expr.(*grammar.OperatorsRule); ok {
			for _, o := range ops.Operands {
				terms = append(terms, o.Action)
			}
			for _, l := range ops.Levels {
				for _, op := range l.Operators {
					terms = append(terms, op.Action)
				}
			}
		} else {
			terms = append(terms, r.Action)
		}
		for _, t := range terms {
			if t != nil && !w.recUses(t, false) {
				return false
			}
		}
		for _, n := range w.rec {
			if n > 1 {
				return false
			}
		}
	}
	return true
}

// recUses counts in w.rec the reads of captures that can hold a call on a recursion with the rule
// in t, and reports false for such a read inside a lambda or in the list of a lambda, unless the
// list is a capture of a repetition and the lambdas hold each element at most once.
func (w *ownWalk) recUses(t grammar.Term, inLambda bool) bool {
	ok := true
	var visit func(t grammar.Term, inLambda bool)
	visit = func(t grammar.Term, inLambda bool) {
		switch t := t.(type) {
		case nil:
		case *grammar.CaptureRef:
			if !t.Param && w.recCapture(t.Name) {
				ok = ok && !inLambda
				w.rec["$"+t.Name]++
			}
		case *grammar.Call:
			fi := 1
			if t.Func == "foldl" || t.Func == "foldr" {
				fi = 2
			}
			for i, a := range t.Args {
				if lam, isLam := a.(*grammar.Lambda); isLam {
					visit(lam.Body, true)
					continue
				}
				if (t.Func == "map" || t.Func == "foldl" || t.Func == "foldr") && i == fi-1 && w.readsRec(a) {
					lam, isLam := lambdaArg(t, fi)
					ok = ok && w.repCapture(a) && isLam && paramsOnce(lam, t.Func != "map")
				}
				visit(a, inLambda)
			}
		default:
			for _, x := range termChildren(t) {
				visit(x, inLambda)
			}
		}
	}
	visit(t, inLambda)
	return ok
}

// ownRules returns the rules whose values are own: the greatest set of rules whose bodies and
// actions build their values only from own parts, computed once.
func (k *classifier) ownRules() map[string]bool {
	m := &k.sizes
	if m.own != nil {
		return m.own
	}
	own := map[string]bool{}
	for _, r := range k.rules {
		own[r.Name] = true
	}
	m.own = own
	for changed := true; changed; {
		changed = false
		for _, r := range k.rules {
			if own[r.Name] && !k.ownRule(r, own) {
				own[r.Name] = false
				changed = true
			}
		}
	}
	return own
}

// ownRule reports whether the value of r is own when the rules in own are.
func (k *classifier) ownRule(r *grammar.RuleDef, own map[string]bool) bool {
	k.control.Poll()
	if ops, ok := r.Expr.(*grammar.OperatorsRule); ok {
		for _, o := range ops.Operands {
			if !k.ownExpr(o.Expr, r.Name, own) || o.Action != nil && !o.DefaultAction && !k.ownTerm(o.Action, r.Name, own) {
				return false
			}
		}
		for _, l := range ops.Levels {
			for _, op := range l.Operators {
				if !k.ownExpr(op.Expr, r.Name, own) || op.Action != nil && !op.DefaultAction && !k.ownTerm(op.Action, r.Name, own) {
					return false
				}
			}
		}
		return true
	}
	if r.Action != nil && !r.DefaultAction {
		return k.ownTerm(r.Action, r.Name, own)
	}
	return k.ownExpr(r.Expr, r.Name, own)
}

// ownExpr reports whether the value of a match of e is own when the rules in own are: no capture
// in a positive lookahead, which stays in effect over input that the match does not consume, is
// part of it, and the actions and rules in it build own values.
func (k *classifier) ownExpr(e grammar.Expr, rule string, own map[string]bool) bool {
	ok := true
	var visit func(e grammar.Expr, look bool)
	visit = func(e grammar.Expr, look bool) {
		grammaranalysis.WalkExpr(e, func(x grammar.Expr) {
			if !ok {
				return
			}
			switch x := x.(type) {
			case *grammar.Capture:
				ok = !look
			case *grammar.And:
				if !look {
					visit(x.Expr, true)
				}
			case *grammar.InlineAction:
				ok = x.Action == nil || x.DefaultAction || k.ownTerm(x.Action, rule, own)
			case *grammar.OperatorsRule:
				ok = false
			case *grammar.Ref:
				// A lookahead discards the node of a rule it calls, so a capture in its body never
				// reaches the value.
				if _, known := own[x.Name]; known {
					ok = own[x.Name]
				}
			}
		})
	}
	visit(e, false)
	return ok
}

// lookCaptures returns the names that rule captures inside a positive lookahead.
func (k *classifier) lookCaptures(rule string) map[string]bool {
	m := &k.sizes
	if s, ok := m.look[rule]; ok {
		return s
	}
	if m.look == nil {
		m.look = map[string]map[string]bool{}
	}
	s := map[string]bool{}
	if r := k.byName[rule]; r != nil {
		grammaranalysis.WalkExpr(r.Expr, func(x grammar.Expr) {
			if a, ok := x.(*grammar.And); ok {
				grammaranalysis.WalkExpr(a.Expr, func(y grammar.Expr) {
					if c, ok := y.(*grammar.Capture); ok {
						s[c.Name] = true
					}
				})
			}
		})
	}
	m.look[rule] = s
	return s
}

// lookedRules returns the rules whose values can hold a capture made in a positive lookahead,
// directly or through the values of the rules and captures that they read: the least such set,
// computed once.
func (k *classifier) lookedRules() map[string]bool {
	m := &k.sizes
	if m.looked != nil {
		return m.looked
	}
	looked := map[string]bool{}
	m.looked = looked
	for changed := true; changed; {
		changed = false
		for _, r := range k.rules {
			if !looked[r.Name] && k.ruleLooks(r) {
				looked[r.Name] = true
				changed = true
			}
		}
	}
	return looked
}

// looks reports whether the size of the value of rule, measured in mode, can come from a capture
// made in a positive lookahead, as lookedRules does but by the size that a copy of the value
// copies: a list built by list(...) has one element for each argument whatever they hold. It is
// the least such assignment, computed once by iterating sizeOf over the rules.
func (k *classifier) looks(rule string, mode sizeMode) bool {
	m := &k.sizes
	if m.looks == nil {
		k.lookedRules()
		m.looks = map[sizeKey]bool{}
		for changed := true; changed; {
			changed = false
			for _, r := range k.rules {
				for _, md := range []sizeMode{textSize, listSize} {
					key := sizeKey{r.Name, md}
					if !m.looks[key] && k.ruleLooksIn(r, md) {
						m.looks[key] = true
						changed = true
					}
				}
			}
		}
	}
	return m.looks[sizeKey{rule, mode}]
}

// ruleLooksIn reports whether the size of the value of r, measured in mode, can come from a
// capture in a positive lookahead, when the sizes in k.sizes.looks can.
func (k *classifier) ruleLooksIn(r *grammar.RuleDef, mode sizeMode) bool {
	tc := termCtx{rule: r.Name}
	switch {
	case isOperators(r.Expr):
		return k.ruleLooks(r)
	case r.Action != nil:
		return k.sizeOf(r.Action, tc, mode).look
	default:
		return k.exprSize(r.Expr, tc, mode, r.Name).look
	}
}

// ruleLooks reports whether the value of r can hold a capture in a positive lookahead, when the
// rules in lookedRules can.
func (k *classifier) ruleLooks(r *grammar.RuleDef) bool {
	k.control.Poll()
	if ops, ok := r.Expr.(*grammar.OperatorsRule); ok {
		for _, o := range ops.Operands {
			if k.exprLooks(o.Expr, r.Name) || o.Action != nil && !o.DefaultAction && k.termLooks(o.Action, r.Name) {
				return true
			}
		}
		for _, l := range ops.Levels {
			for _, op := range l.Operators {
				if k.exprLooks(op.Expr, r.Name) || op.Action != nil && !op.DefaultAction && k.termLooks(op.Action, r.Name) {
					return true
				}
			}
		}
		return false
	}
	if r.Action != nil && !r.DefaultAction {
		return k.termLooks(r.Action, r.Name)
	}
	return k.exprLooks(r.Expr, r.Name)
}

// exprLooks reports whether the value of a match of e in rule can hold a capture in a positive
// lookahead whose size is not bounded by a constant: e has such a capture, or calls or acts on a
// value that can hold one.
func (k *classifier) exprLooks(e grammar.Expr, rule string) bool {
	found := false
	grammaranalysis.WalkExpr(e, func(x grammar.Expr) {
		if found {
			return
		}
		switch x := x.(type) {
		case *grammar.And:
			grammaranalysis.WalkExpr(x.Expr, func(y grammar.Expr) {
				if y, ok := y.(*grammar.Capture); ok && !k.bounded(y.Expr) {
					found = true
				}
			})
		case *grammar.Ref:
			found = k.sizes.looked[x.Name]
		case *grammar.InlineAction:
			found = x.Action != nil && !x.DefaultAction && k.termLooks(x.Action, rule)
		}
	})
	return found
}

// termLooks reports whether the value of the term t in rule can hold a capture in a positive
// lookahead: it reads one, a capture whose value can hold one, or the matched values by index.
func (k *classifier) termLooks(t grammar.Term, rule string) bool {
	found := false
	grammaranalysis.WalkTerm(t, func(x grammar.Term) {
		if found {
			return
		}
		switch x := x.(type) {
		case *grammar.CaptureRef:
			if x.Param {
				return
			}
			exprs := k.captures[rule][x.Name]
			if k.lookCaptures(rule)[x.Name] {
				for _, e := range exprs {
					found = found || !k.bounded(e)
				}
				if len(exprs) == 0 || found {
					found = true
					return
				}
			}
			for _, e := range exprs {
				found = found || k.exprLooks(e, rule)
			}
		}
	})
	return found
}

// ownWalk checks that a term builds an own value. Each part of an own value is at most a constant
// times the input it consumes; a part that holds the value of a call on a recursion with the rule
// may appear once, so that the parts do not double over the levels of the recursion, and a lambda
// reads nothing unbounded from outside it, which it would hold once for each element.
type ownWalk struct {
	k    *classifier
	rule string
	own  map[string]bool
	rec  map[string]int // the uses of the captures that hold a call on a recursion with the rule
	ok   bool
}

// ownLam is the lambda around a term in the walk.
type ownLam struct {
	params []string // the parameters of the innermost lambda
	outer  []string // the parameters of the lambdas around it
	acc    string   // the accumulator of the innermost lambda, if it is a fold
	uses   int      // the uses of acc
}

// ownTerm reports whether the term t in rule builds an own value when the rules in own are.
func (k *classifier) ownTerm(t grammar.Term, rule string, own map[string]bool) bool {
	w := &ownWalk{k: k, rule: rule, own: own, rec: map[string]int{}, ok: true}
	w.term(t, nil)
	for _, n := range w.rec {
		if n > 1 {
			return false
		}
	}
	return w.ok
}

// recCapture reports whether the capture name of the rule can hold a call on a recursion with it.
// The operand values of an Operators Rule are values of the rule itself.
func (w *ownWalk) recCapture(name string) bool {
	exprs := w.k.captures[w.rule][name]
	if len(exprs) == 0 {
		return true
	}
	found := false
	for _, e := range exprs {
		grammaranalysis.WalkExpr(e, func(x grammar.Expr) {
			if r, ok := x.(*grammar.Ref); ok && w.k.byName[r.Name] != nil && w.k.onRecursion(w.rule, r.Name) {
				found = true
			}
		})
	}
	return found
}

// readsRec reports whether t reads a capture that can hold a call on a recursion with the rule.
func (w *ownWalk) readsRec(t grammar.Term) bool {
	found := false
	grammaranalysis.WalkTerm(t, func(x grammar.Term) {
		switch x := x.(type) {
		case *grammar.CaptureRef:
			if !x.Param && w.recCapture(x.Name) {
				found = true
			}
		}
	})
	return found
}

func (w *ownWalk) term(t grammar.Term, lam *ownLam) {
	if !w.ok || t == nil {
		return
	}
	k := w.k
	switch t := t.(type) {
	case *grammar.IntLit, *grammar.BoolLit, *grammar.NilLit, *grammar.StringLit, *grammar.Unary:
	case *grammar.VarRef:
		// The value of a variable is not part of the input the match consumes.
		w.ok = false
	case *grammar.CaptureRef:
		if lam != nil && t.Param {
			switch {
			case t.Name == lam.acc:
				lam.uses++
				w.ok = lam.uses <= 1
			case containsName(lam.params, t.Name):
			default:
				// A parameter of a lambda around the innermost one, read once for each element.
				w.ok = false
			}
			return
		}
		exprs := k.captures[w.rule][t.Name]
		if lam != nil {
			// A value from outside the lambda is held once for each element.
			for _, e := range exprs {
				w.ok = w.ok && k.bounded(e)
			}
			if len(exprs) == 0 {
				w.ok = false
			}
		}
		if k.lookCaptures(w.rule)[t.Name] {
			w.ok = false
		}
		for _, e := range exprs {
			w.ok = w.ok && k.ownExpr(e, w.rule, w.own)
		}
		if len(exprs) == 0 {
			if r := k.byName[w.rule]; r == nil || !isOperators(r.Expr) {
				w.ok = false
				return
			}
			w.ok = w.ok && w.own[w.rule]
		}
		if w.recCapture(t.Name) {
			w.rec["$"+t.Name]++
		}
	case *grammar.TermGroup:
		w.term(t.Term, lam)
	case *grammar.Cast:
		w.term(t.X, lam)
	case *grammar.Member:
		w.term(t.X, lam)
	case *grammar.New:
		for _, a := range t.Args {
			w.term(a, lam)
		}
		for _, f := range t.Fields {
			w.term(f.Value, lam)
		}
	case *grammar.TupleLit:
		for _, x := range t.Elems {
			w.term(x, lam)
		}
	case *grammar.ListConstructor:
		for _, x := range t.Items {
			w.term(x, lam)
		}
	case *grammar.Binary:
		switch {
		case t.Op == "+":
			w.term(t.L, lam)
			w.term(t.R, lam)
		case t.Op == "[]":
			// An element of a list, taken by an int.
			w.term(t.L, lam)
		default:
			// An int or a Boolean.
		}
	case *grammar.Call:
		switch t.Func {
		case "len":
		case "text", "list", "concat":
			for _, a := range t.Args {
				w.term(a, lam)
			}
		case "filter":
			// The kept elements are elements of the list.
			if len(t.Args) > 0 {
				w.term(t.Args[0], lam)
			}
		case "map", "foldl", "foldr":
			fold := t.Func != "map"
			li, fi := 0, 1
			if fold {
				li, fi = 1, 2
				if len(t.Args) > 0 {
					w.term(t.Args[0], lam)
				}
			}
			f, isLam := lambdaArg(t, fi)
			if len(t.Args) <= li || !isLam {
				w.ok = false
				return
			}
			// The lambda adds to each element of the list: over the list of a recursion, these
			// additions add up over its levels. The elements of a repetition are distinct matches
			// instead, so a lambda that holds each of them at most once holds the value of each
			// call on the recursion in them at most once.
			if w.readsRec(t.Args[li]) && !(w.repCapture(t.Args[li]) && paramsOnce(f, fold)) {
				w.ok = false
				return
			}
			w.term(t.Args[li], lam)
			inner := &ownLam{params: f.Params}
			if lam != nil {
				inner.outer = append(append(inner.outer, lam.outer...), lam.params...)
			}
			if fold && len(f.Params) > 0 {
				inner.acc = f.Params[0]
			}
			w.term(f.Body, inner)
		default:
			w.ok = false
		}
	default:
		w.ok = false
	}
}

// repCapture reports whether t is a capture of the rule of repetitions only, whose elements are
// distinct matches.
func (w *ownWalk) repCapture(t grammar.Term) bool {
	r, ok := t.(*grammar.CaptureRef)
	if !ok || r.Param {
		return false
	}
	exprs := w.k.captures[w.rule][r.Name]
	for _, e := range exprs {
		for {
			g, ok := e.(*grammar.Group)
			if !ok {
				break
			}
			e = g.Expr
		}
		if _, ok := e.(*grammar.Repeat); !ok {
			return false
		}
	}
	return len(exprs) > 0
}

// paramsOnce reports whether lam and every lambda inside it read each part of each of their
// parameters other than the accumulator of a fold at most once: no member path of a parameter,
// such as x or x.$a, is read twice or read with a path inside it. Distinct members of an element
// of a repetition are disjoint parts of its match. fold says whether lam is the lambda of a fold.
func paramsOnce(lam *grammar.Lambda, fold bool) bool {
	var params []string
	for i, n := range lam.Params {
		if i > 0 || !fold {
			params = append(params, n)
		}
	}
	var paths []string
	ok := true
	var visit func(t grammar.Term)
	visit = func(t grammar.Term) {
		if p, isPath := paramPath(t, params); isPath {
			paths = append(paths, p)
			return
		}
		if c, isCall := t.(*grammar.Call); isCall {
			fi := 1
			if c.Func == "foldl" || c.Func == "foldr" {
				fi = 2
			}
			if inner, isLam := lambdaArg(c, fi); isLam {
				ok = ok && paramsOnce(inner, fi == 2)
			}
		}
		for _, x := range termChildren(t) {
			visit(x)
		}
	}
	visit(lam.Body)
	for i, p := range paths {
		for _, q := range paths[i+1:] {
			if p == q || strings.HasPrefix(q, p+".") || strings.HasPrefix(p, q+".") {
				ok = false
			}
		}
	}
	return ok
}

// paramPath returns the member path of t, such as x.$a, when t is one of the parameters names or
// a member of one.
func paramPath(t grammar.Term, names []string) (string, bool) {
	switch t := t.(type) {
	case *grammar.CaptureRef:
		return t.Name, t.Param && containsName(names, t.Name)
	case *grammar.Member:
		p, ok := paramPath(t.X, names)
		return p + "." + t.Name, ok
	}
	return "", false
}

func containsName(names []string, n string) bool {
	for _, x := range names {
		if x == n {
			return true
		}
	}
	return false
}

// foldAccShape returns the shape of the accumulator of the lambda of a fold call, whose degrees
// the step matrix gives from the initial value and the list.
func (k *classifier) foldAccShape(call *grammar.Call, lam *grammar.Lambda, tc termCtx) growthShape {
	exp, deg, _, ok := k.foldDegrees(call, lam, tc)
	if !ok {
		return growthShape{}
	}
	if exp {
		return unknownShape
	}
	return degShape(deg)
}

// termChildren returns the terms directly inside t.
func termChildren(t grammar.Term) []grammar.Term {
	switch t := t.(type) {
	case *grammar.TermGroup:
		return []grammar.Term{t.Term}
	case *grammar.Block:
		return t.Statements
	case *grammar.Member:
		return []grammar.Term{t.X}
	case *grammar.New:
		out := append([]grammar.Term(nil), t.Args...)
		for _, f := range t.Fields {
			out = append(out, f.Value)
		}
		return out
	case *grammar.TupleLit:
		return t.Elems
	case *grammar.ListConstructor:
		return t.Items
	case *grammar.Call:
		return t.Args
	case *grammar.Lambda:
		return []grammar.Term{t.Body}
	case *grammar.Binary:
		return []grammar.Term{t.L, t.R}
	case *grammar.Unary:
		return []grammar.Term{t.X}
	case *grammar.Cast:
		return []grammar.Term{t.X}
	case *grammar.Bind:
		return []grammar.Term{t.Value}
	}
	return nil
}

// maxDeg returns the largest of the degrees d.
func maxDeg(d [3]int) int { return max(d[0], d[1], d[2]) }
