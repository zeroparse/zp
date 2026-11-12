package check

import (
	"maps"
	"reflect"
	"slices"
	"sort"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/diag"
	"zeroparse.dev/zp/internal/types"

	"zeroparse.dev/zp/grammar"
)

// This file infers the types of rule values in the value model, in which a rule produces an
// ordinary typed value and the syntax tree is built from one generic node[T, C]. These types are
// the types the checker checks the grammar with (checkTypes).

// ValueElem is an element of the tuple an expression contributes to its scope.
type ValueElem struct {
	t    Ty
	caps []CapIndex // the captures in this element, with paths relative to it
}

// ValueInference computes value types. Rules are computed on demand, so that the type of a rule
// referred to from an action is known when the action is checked. The rules of a cycle of
// references are computed together as the least fixed point of their values.
type ValueInference struct {
	Prog    *Checker
	control *cancel.Control
	Rules   map[string]*grammar.RuleDef
	Out     map[string]Ty
	// cycle maps each rule that refers to itself, directly or through other rules, to the rules
	// of its cycle by name, and approx holds the values of the rules of the cycles being
	// computed.
	cycle  map[string][]string
	Approx map[string]Ty
	// Computing holds the rules outside cycles that are being computed.
	Computing map[string]bool
	// errs holds the type errors of the rule being computed, and final those of computed rules.
	errs, final diag.ErrorList
	Quiet       bool // while a lambda body is typed again with a wider parameter type
	Rule        *grammar.RuleDef
	// spliced holds the splice rules whose bodies are being expanded; a call inside them is an
	// element of its own (Rule.Splice).
	spliced map[string]bool
	// bottoms, while not nil, are the rules of type _|_ as far as Program.bottomRules has found
	// them.
	bottoms map[string]bool
	// nodeMembers, if not nil, receives the member accesses that read only node members.
	nodeMembers map[*grammar.Member]bool
	// rec, if not nil, receives the types of terms, scopes and rules.
	rec *ValueRecord
	// memo, if not nil, holds the elements of the expressions met outside the bodies of splice
	// rules being expanded, which depend on nothing else (ShapeCache). Its slices are shared:
	// nothing may change them.
	memo map[grammar.Expr][]ValueElem
}

// ValueRecord holds the types the value inference gives the parts of a program, for the backends
// (Core IR terms and capture slots). Each part has the type of its last typing: terms typed again
// quietly, with a wider lambda parameter, and the rules of a cycle, typed again in each round,
// keep the final one.
type ValueRecord struct {
	// Terms holds the type of each action, predicate and binding term and of their subterms.
	Terms map[grammar.Term]Ty
	// Proven reports, for each node type constructor that converts a value, that the conversion
	// is proven: every value of the operand converts, without a check at run time.
	Proven map[*grammar.New]bool
	// Scopes holds, for the body of each capture scope (a rule body, a repetition element with
	// captures of its own, an Operators Rule line), the type of each capture a term reads.
	Scopes map[grammar.Expr]map[string]Ty
	// Rules holds the value type of every rule, splice rules included.
	Rules map[string]Ty
}

func newValueRecord() *ValueRecord {
	return &ValueRecord{Terms: map[grammar.Term]Ty{}, Proven: map[*grammar.New]bool{}, Scopes: map[grammar.Expr]map[string]Ty{}}
}

// scope records the capture types of the scope with the body e and the captures caps.
func (v *ValueInference) scope(e grammar.Expr, caps []CapIndex) {
	if v.rec == nil || v.Quiet {
		return
	}
	types := map[string]Ty{}
	for _, c := range caps {
		if _, ok := types[c.Name]; !ok {
			types[c.Name], _ = CapType(caps, c.Name)
		}
	}
	v.rec.Scopes[e] = types
}

// ValueTypes returns the value type of each rule of a type-checked program. It is computed on
// demand: nothing uses it for compilation yet.
func (p *Checker) ValueTypes() map[string]Ty {
	if p.Typed == nil {
		return nil
	}
	p.Typed.valuesOnce.Do(func() {
		p.Typed.Values, p.Typed.valueErrs = ValueTypes(p, nil)
	})
	return p.Typed.Values
}

// ValueType is ValueType.
func (p *Checker) ValueType(name string) (string, bool) {
	t, ok := p.ValueTypes()[name]
	if !ok {
		return "", false
	}
	return t.String(), true
}

// valueErrors returns the errors of actions in the value model of a type-checked program.
func (p *Checker) ValueErrors() diag.ErrorList {
	if p.ValueTypes() == nil {
		return nil
	}
	return p.Typed.valueErrs
}

// ValueTypes returns the value type of each rule, and the errors of the actions.
func ValueTypes(p *Checker, control *cancel.Control) (map[string]Ty, diag.ErrorList) {
	return valueTypesChecked(p, control, nil, nil)
}

// valueTypesChecked is valueTypes recording the node member accesses in nodeMembers and the
// types of the program's parts in rec, each if not nil.
func valueTypesChecked(p *Checker, control *cancel.Control, nodeMembers map[*grammar.Member]bool, rec *ValueRecord) (map[string]Ty, diag.ErrorList) {
	v := &ValueInference{Prog: p, control: control, Rules: map[string]*grammar.RuleDef{}, Out: map[string]Ty{}, Approx: map[string]Ty{}, Computing: map[string]bool{}, nodeMembers: nodeMembers, rec: rec}
	rules := p.Grammar().Rules()
	for _, rd := range rules {
		v.Rules[rd.Name] = rd
	}
	v.cycle = RuleCycles(rules, v.Rules, control)
	for _, rd := range rules {
		control.Poll()
		v.ruleValue(rd.Name)
	}
	if rec != nil {
		rec.Rules = maps.Clone(v.Out)
	}
	return PruneSpliceTypes(p, rules, v.Out), v.final
}

// PruneSpliceTypes returns the value types of the rules that have a type of their own: a splice
// rule makes no node, so it has a type only where a recursive call of it is an element of its own,
// and then only because another type refers to it.
func PruneSpliceTypes(p *Checker, rules []*grammar.RuleDef, out map[string]Ty) map[string]Ty {
	var queue []string
	kept := map[string]bool{}
	for _, rd := range rules {
		if _, splice := p.SpliceBody(rd.Name); !splice {
			kept[rd.Name] = true
			queue = append(queue, rd.Name)
		}
	}
	if len(kept) == len(rules) {
		return out
	}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		t, ok := out[name]
		if !ok {
			continue
		}
		types.WalkTy(t, func(u Ty) {
			if r, ok := u.(RuleTy); ok && !kept[r.Name] {
				kept[r.Name] = true
				queue = append(queue, r.Name)
			}
		})
	}
	pruned := make(map[string]Ty, len(kept))
	for name, t := range out {
		if kept[name] {
			pruned[name] = t
		}
	}
	return pruned
}

// RuleCycles returns the strongly connected components of the references between rules that
// have a cycle, for each of their rules. The value of a rule can only depend on the rules it
// refers to, directly or through other rules, since a rule type comes from a reference.
func RuleCycles(rules []*grammar.RuleDef, byName map[string]*grammar.RuleDef, control *cancel.Control) map[string][]string {
	refs := map[string][]string{}
	for _, rd := range rules {
		control.Poll()
		walkExprControlled(rd.Expr, func(e grammar.Expr) {
			control.Poll()
			if r, ok := e.(*grammar.Ref); ok && byName[r.Name] != nil {
				refs[rd.Name] = append(refs[rd.Name], r.Name)
			}
		}, control)
	}
	// Tarjan's algorithm.
	index, low := map[string]int{}, map[string]int{}
	onStack := map[string]bool{}
	var stack []string
	out := map[string][]string{}
	var visit func(n string)
	visit = func(n string) {
		control.Poll()
		index[n], low[n] = len(index), len(index)
		stack = append(stack, n)
		onStack[n] = true
		self := false
		for _, m := range refs[n] {
			self = self || m == n
			if _, ok := index[m]; !ok {
				visit(m)
				low[n] = min(low[n], low[m])
			} else if onStack[m] {
				low[n] = min(low[n], index[m])
			}
		}
		if low[n] != index[n] {
			return
		}
		i := len(stack) - 1
		for stack[i] != n {
			i--
		}
		scc := slices.Clone(stack[i:])
		stack = stack[:i]
		for _, m := range scc {
			onStack[m] = false
		}
		if len(scc) == 1 && !self {
			return
		}
		slices.Sort(scc)
		for _, m := range scc {
			out[m] = scc
		}
	}
	for _, rd := range rules {
		if _, ok := index[rd.Name]; !ok {
			visit(rd.Name)
		}
	}
	return out
}

// maxCycleRounds bounds the rounds of a cycle; the values that still change after it are not
// known.
const maxCycleRounds = 8

// ruleValue returns the value type of the rule name.
func (v *ValueInference) ruleValue(name string) Ty {
	if t, ok := v.Out[name]; ok {
		return t
	}
	if t, ok := v.Approx[name]; ok {
		return t
	}
	rd := v.Rules[name]
	if rd == nil || v.Computing[name] {
		return TyAny
	}
	rule, quiet, errs := v.Rule, v.Quiet, v.errs
	defer func() { v.Rule, v.Quiet, v.errs = rule, quiet, errs }()
	v.Quiet = false
	cycle := v.cycle[name]
	if cycle == nil {
		v.Computing[name] = true
		v.Rule, v.errs = rd, nil
		t := v.ruleDef(rd)
		delete(v.Computing, name)
		v.final = append(v.final, v.errs...)
		v.Out[name] = t
		return t
	}
	// The rules of the cycle start with no value, _|_, and each round computes every rule from
	// the values of the previous round, so that the result does not depend on the order of the
	// rules. The errors are those of the last round, which sees the final values.
	for _, m := range cycle {
		v.Approx[m] = TyBottom
	}
	var mine diag.ErrorList
	for round := 0; ; round++ {
		next := make(map[string]Ty, len(cycle))
		mine = nil
		for _, m := range cycle {
			v.Rule, v.errs = v.Rules[m], nil
			next[m] = Vjoin(v.Approx[m], v.ruleDef(v.Rules[m]))
			mine = append(mine, v.errs...)
		}
		changed := false
		for _, m := range cycle {
			if !reflect.DeepEqual(next[m], v.Approx[m]) {
				changed = true
				if round >= maxCycleRounds {
					next[m] = TyAny
				}
			}
		}
		if !changed || round > maxCycleRounds {
			break
		}
		for _, m := range cycle {
			v.Approx[m] = next[m]
		}
	}
	for _, m := range cycle {
		v.Out[m] = v.Approx[m]
		delete(v.Approx, m)
	}
	v.final = append(v.final, mine...)
	return v.Out[name]
}

// spliceElems returns the elements a call of the splice rule name contributes: those of its body,
// without the captures, which end with the rule.
func (v *ValueInference) spliceElems(name string, body grammar.Expr) []ValueElem {
	if v.spliced == nil {
		v.spliced = map[string]bool{}
	}
	v.spliced[name] = true
	defer delete(v.spliced, name)
	var out []ValueElem
	for _, el := range v.elems(body) {
		if el.t != nil {
			out = append(out, ValueElem{t: el.t})
		}
	}
	return out
}

func (v *ValueInference) ruleDef(rd *grammar.RuleDef) Ty {
	if pr, ok := rd.Expr.(*grammar.OperatorsRule); ok {
		return v.operatorsRule(rd, pr)
	}
	env := &valueEnv{caps: tupleNode(v.elems(rd.Expr)).Caps}
	v.scope(rd.Expr, env.caps)
	v.predicates(rd.Expr, env)
	if _, ok := v.Prog.SpliceBody(rd.Name); ok {
		// The value of a rule that makes no node of its own is its one element, as a call takes it.
		if v.spliced == nil {
			v.spliced = map[string]bool{}
		}
		v.spliced[rd.Name] = true
		defer delete(v.spliced, rd.Name)
		if v.bottom(rd.Expr) {
			return TyBottom // a body that never matches gives no value
		}
		el, ok := v.single(rd.Expr)
		if !ok {
			return NodeTy{Value: TupleTy{Elems: []Ty{}}}
		}
		return el.t
	}
	if rd.Action == nil {
		if rd.Type == nil && v.bottomRule(rd.Name) {
			return TyBottom // a body that never matches gives no value
		}
		return v.declared(rd, v.ScopeValue(rd.Expr))
	}
	return v.declared(rd, v.term(rd.Action, env))
}

// predicates types the terms of the predicates in e, which see the captures of env. A repetition
// element with a scope of its own, and the body of an inline action, see their own captures before
// those of the enclosing scope.
func (v *ValueInference) predicates(e grammar.Expr, env *valueEnv) {
	v.control.Poll()
	inner := func(body grammar.Expr) *valueEnv {
		caps := tupleNode(v.elems(body)).Caps
		v.scope(body, caps)
		return &valueEnv{caps: append(slices.Clone(caps), env.caps...), locals: env.locals}
	}
	switch e := e.(type) {
	case *grammar.Predicate:
		if a, ok := e.Term.(*grammar.Bind); ok {
			t := v.term(a.Value, env)
			if d := v.Prog.Vars().Lookup(a.Name); d != nil {
				switch {
				case !v.assignable(t, d.Typ):
					v.errorf(a.Diagnostic(), "cannot use %s as %s in the binding of variable %s%s", v.display(t), d.Typ, a.Name, v.castHint(t, d.Typ))
				case v.needsCast(t, d.Typ):
					v.errorf(a.Diagnostic(), "cannot use %s as %s in the binding of variable %s: the type of the value is not known; convert it with a cast, as in value.(%s)", v.display(t), d.Typ, a.Name, d.Typ)
				}
			}
		} else {
			v.term(e.Term, env)
		}
	case *grammar.Repeat:
		if v.Prog.ElementScoped(e.Expr, v.control) {
			v.predicates(e.Expr, inner(e.Expr))
		} else {
			v.predicates(e.Expr, env)
		}
	case *grammar.InlineAction:
		v.predicates(e.Expr, inner(e.Expr))
	case *grammar.Seq:
		for _, it := range e.Items {
			v.predicates(it, env)
		}
	case *grammar.Choice:
		for _, alt := range e.Alts {
			v.predicates(alt, env)
		}
	case *grammar.Recovery:
		v.predicates(e.Expr, env)
		v.predicates(e.Skip, env)
	case *grammar.Group:
		v.predicates(e.Expr, env)
	case *grammar.Optional:
		v.predicates(e.Expr, env)
	case *grammar.And:
		v.predicates(e.Expr, env)
	case *grammar.Not:
		v.predicates(e.Expr, env)
	case *grammar.Atomic:
		v.predicates(e.Expr, env)
	case *grammar.Stream:
		v.predicates(e.Expr, env)
	case *grammar.Discard:
		v.predicates(e.Expr, env)
	case *grammar.Capture:
		v.predicates(e.Expr, env)
	case *grammar.Attributed:
		v.predicates(e.Expr, env)
		// An expression argument, such as the skip expression of #recover, is a scope of its own.
		for _, a := range e.Attrs {
			v.control.Poll()
			for _, arg := range expressionArgs(v.control, a) {
				v.predicates(arg, &valueEnv{caps: tupleNode(v.elems(arg)).Caps})
			}
		}
	}
}

// ScopeValue is the value of e as the body of a scope: a rule body, an operand or a repetition
// element. A scope whose body is a sequence, or that has captures, is a node of its
// tuple; otherwise its one element is its value, and a scope without elements is node[()].
func (v *ValueInference) ScopeValue(e grammar.Expr) Ty {
	elems := v.elems(e)
	seq := v.isSeq(e)
	if !seq && len(elems) == 1 && len(elems[0].caps) == 0 {
		return elems[0].t
	}
	return tupleNode(elems)
}

// tupleNode is the node of the tuple of elems, with their captures. An element without a type
// holds only captures kept beside the tuple.
func tupleNode(elems []ValueElem) NodeTy {
	n := NodeTy{Value: TupleTy{Elems: []Ty{}}}
	var ts []Ty
	for _, el := range elems {
		if el.t == nil {
			n.Caps = append(n.Caps, el.caps...)
			continue
		}
		for _, c := range el.caps {
			var paths [][]int
			for _, path := range c.Paths {
				paths = append(paths, append([]int{len(ts)}, path...))
			}
			n.Caps = append(n.Caps, CapIndex{Name: c.Name, Paths: paths, Side: c.Side, T: c.T, Maybe: c.Maybe, Set: c.Set})
		}
		ts = append(ts, el.t)
	}
	if len(ts) > 0 {
		n.Value = TupleTy{Elems: ts}
	}
	sort.SliceStable(n.Caps, func(i, j int) bool { return n.Caps[i].Name < n.Caps[j].Name })
	return n
}

// single is the value of e where it takes one element: an alternative, the operand of ? or a
// capture. Several elements become the node of their tuple. ok is false when e has no value;
// the result then holds the captures kept beside the tuple in e. The captures of the result may be
// changed without changing its type.
func (v *ValueInference) single(e grammar.Expr) (ValueElem, bool) {
	return oneElem(v.elems(e), v.isSeq(e))
}

func oneElem(elems []ValueElem, seq bool) (ValueElem, bool) {
	var vals []ValueElem
	var side []CapIndex
	for _, el := range elems {
		if el.t == nil {
			side = append(side, el.caps...)
		} else {
			vals = append(vals, el)
		}
	}
	switch {
	case len(vals) == 0:
		return ValueElem{caps: side}, false
	case len(vals) == 1 && !seq:
		el := vals[0]
		el.caps = append(slices.Clone(el.caps), side...)
		return el, true
	}
	n := tupleNode(elems)
	return ValueElem{t: n, caps: slices.Clone(n.Caps)}, true
}

// isSeq is isSeq (value_shape.go) for the elements elems returns: a call of a splice rule whose
// body is being expanded is one element, the rule's own node, not the sequence of its body.
func (v *ValueInference) isSeq(e grammar.Expr) bool {
	var seen map[string]bool
	for {
		switch x := UngroupExpr(e).(type) {
		case *grammar.Seq:
			return true
		case *grammar.Attributed:
			// An attribute other than recovery only labels e; recovery makes it one element.
			if Recovers(x) {
				return false
			}
			e = x.Expr
		case *grammar.Ref:
			body, ok := v.Prog.SpliceBody(x.Name)
			if !ok || v.spliced[x.Name] || seen[x.Name] {
				return false
			}
			if seen == nil {
				seen = map[string]bool{}
			}
			seen[x.Name] = true
			e = body
		default:
			return false
		}
	}
}

// elems returns the elements e contributes to the tuple of its scope.
func (v *ValueInference) elems(e grammar.Expr) []ValueElem {
	if v.memo == nil || len(v.spliced) != 0 {
		return v.exprElems(e)
	}
	if out, ok := v.memo[e]; ok {
		return out
	}
	out := v.exprElems(e)
	v.memo[e] = out
	return out
}

// exprElems computes elems(e).
func (v *ValueInference) exprElems(e grammar.Expr) []ValueElem {
	v.control.Poll()
	switch e := e.(type) {
	case *grammar.Group:
		return v.elems(e.Expr)
	case *grammar.Seq:
		var out []ValueElem
		for _, it := range e.Items {
			out = append(out, v.elems(it)...)
		}
		return out
	case *grammar.Literal, *grammar.CharClass, *grammar.Any, *grammar.Atomic:
		return []ValueElem{{t: TyStringNode}}
	case *grammar.Ref:
		if body, ok := v.Prog.SpliceBody(e.Name); ok && !v.spliced[e.Name] {
			return v.spliceElems(e.Name, body)
		}
		return []ValueElem{{t: RuleTy{Name: e.Name}}}
	case *grammar.Choice:
		var ts []Ty
		valueless := false
		count := map[string]int{}
		var all, never []CapIndex
		alts := 0
		for _, alt := range e.Alts {
			if v.bottom(alt) {
				// _|_ | T is T. The captures of an alternative that never matches are never made.
				el, _ := v.single(alt)
				never = append(never, el.caps...)
				continue
			}
			alts++
			el, ok := v.single(alt)
			if ok {
				ts = append(ts, el.t)
			} else {
				valueless = true
			}
			for _, c := range el.caps {
				count[c.Name]++
				all = append(all, c)
			}
		}
		// A name that only alternatives that never match capture is nil.
		for _, c := range neverMade(never) {
			if count[c.Name] == 0 {
				all = append(all, c)
			}
		}
		var t Ty // no element when no alternative has a value
		if len(ts) > 0 {
			t = Vjoin(ts...)
			if valueless {
				t = types.Vopt(t)
			}
		} else if len(all) == 0 {
			return nil
		}
		// A name captured in only some alternatives is nil when another matched.
		var caps []CapIndex
		for _, c := range all {
			if count[c.Name] < alts {
				c = maybeCap(c)
			}
			caps = MergeCap(caps, c)
		}
		return []ValueElem{{t: t, caps: caps}}
	case *grammar.Optional:
		el, ok := v.single(e.Expr)
		if v.bottom(e.Expr) {
			// An operand of type _|_ never matches: a? matches the empty input and is nil, of type
			// *_|_, the optional type whose only value is nil. Its captures are never made.
			el, ok = ValueElem{t: TyBottom, caps: neverMade(el.caps)}, true
		}
		if !ok && len(el.caps) == 0 {
			return nil
		}
		el.caps = slices.Clone(el.caps)
		for i := range el.caps {
			el.caps[i] = maybeCap(el.caps[i])
		}
		if ok {
			el.t = types.Vopt(el.t)
		}
		return []ValueElem{el}
	case *grammar.Repeat:
		if v.bottom(e.Expr) {
			// An element of type _|_ never matches: a* is the empty list, of type []_|_, the list
			// type whose only value is the empty list (a+ has the type _|_, see bottom).
			return []ValueElem{{t: NodeTy{Value: ListTy{Elem: TyBottom}}}}
		}
		return []ValueElem{{t: NodeTy{Value: ListTy{Elem: v.ScopeValue(e.Expr)}}}}
	case *grammar.Capture:
		el, ok := v.single(e.Expr)
		if !ok {
			el.t = NodeTy{Value: TupleTy{Elems: []Ty{}}}
		}
		el.caps = append([]CapIndex{{Name: e.Name, Paths: [][]int{{}}, T: el.t}}, el.caps...)
		return []ValueElem{el}
	case *grammar.Attributed:
		elems := v.elems(e.Expr)
		for _, a := range e.Attrs {
			if a.Name != "recover" && a.Name != "missing" {
				continue
			}
			// What recovers is one element: its value or an error, which #missing makes a missing
			// token. An expression without a value stays without one: its error is only recorded.
			// An expression of type _|_ is no exception to the join: its recovery is only the
			// error, since join(_|_, error) is error, whatever elements it would have if it
			// matched. Its captures are never made.
			el, ok := oneElem(elems, v.isSeq(e.Expr))
			switch {
			case v.bottom(e.Expr):
				el = ValueElem{t: TyBottom, caps: neverMade(el.caps)}
			case !ok:
				continue
			}
			if a.Name == "missing" {
				el.t = Vjoin(el.t, TyMissing)
			} else {
				el.t = Vjoin(el.t, TyErr)
			}
			elems = []ValueElem{el}
		}
		return elems
	case *grammar.And:
		// The captures of a positive lookahead have no element; their values are kept beside the
		// tuple.
		var caps []CapIndex
		for _, c := range tupleNode(v.elems(e.Expr)).Caps {
			caps = append(caps, CapIndex{Name: c.Name, Side: true, T: c.T})
		}
		if len(caps) == 0 {
			return nil
		}
		return []ValueElem{{caps: caps}}
	case *grammar.InlineAction:
		// Inline actions are lowered to rules (inline_actions.go) before value types are computed.
		return []ValueElem{{t: TyAny}}
	}
	// Top, bottom, anchors, negative lookaheads, discards, the cut and predicates have no value.
	return nil
}

// bottom reports whether e has the type _|_, the type without values, which an expression has when
// it never matches. The rules are those of the type system (spec/language/types.md): _|_ has it; a
// sequence with an element of type _|_ has it, as a tuple of _|_ is _|_; a choice has the join of
// its alternatives, _|_ only when each has it; a repetition of at least one element of type _|_
// has it; a group and a label have the type of the expression they stand for, as in elems. A
// capture has the type node[T] of the type T of its operand, and node[_|_] is _|_. An #atomic,
// otherwise of type node[string], matches only where its operand does, so an #atomic of an
// expression that never matches never matches and has no value: its type is _|_. A call of another
// rule has the rule's type, which is the type of its body for a rule without an action or a
// declared type (ruleDef), as for a call of a splice rule (Program.bottomRules). A recovery joins
// the type with error, which is not _|_. Every other expression has a type of its own whatever its
// operand is: no value (a lookahead, a discard, a predicate), *_|_ (an optional, which matches the
// empty input), []_|_ (a repetition that may match no element), a match.
func (v *ValueInference) bottom(e grammar.Expr) bool {
	v.control.Poll()
	switch e := e.(type) {
	case *grammar.Bottom:
		return true
	case *grammar.Group:
		return v.bottom(e.Expr)
	case *grammar.Seq:
		return slices.ContainsFunc(e.Items, v.bottom)
	case *grammar.Choice:
		for _, alt := range e.Alts {
			if !v.bottom(alt) {
				return false
			}
		}
		return true
	case *grammar.Repeat:
		return e.Min > 0 && v.bottom(e.Expr)
	case *grammar.Attributed:
		return !Recovers(e) && v.bottom(e.Expr)
	case *grammar.Capture:
		return v.bottom(e.Expr)
	case *grammar.Atomic:
		return v.bottom(e.Expr)
	case *grammar.Ref:
		return v.bottomRule(e.Name)
	}
	return false
}

// bottomRule reports whether the rule name has the type _|_ (Program.bottomRules).
func (v *ValueInference) bottomRule(name string) bool {
	if v.bottoms != nil {
		return v.bottoms[name]
	}
	return v.Prog.bottomRules(v.control)[name]
}

// bottomRules returns the rules of type _|_: those without an action or a declared type, whose
// type is that of their body, with a body of type _|_. A body that calls such rules is decided
// with the others, as the greatest set of rules whose bodies have the type _|_ when the calls of
// the rules of the set do: a rule that only calls itself, as r = "a" r, never matches. The set is
// computed once, from the rules that the program has when it is first asked.
func (p *Checker) bottomRules(control *cancel.Control) map[string]bool {
	if p == nil {
		return nil
	}
	p.bottomsOnce.Do(func() {
		set := map[string]bool{}
		var names []string
		p.EachRule(func(name string, r RuleInfo) {
			if r.Def != nil && r.Def.Action == nil && r.Def.Type == nil {
				set[name] = true
				names = append(names, name)
			}
		})
		sort.Strings(names)
		w := &ValueInference{Prog: p, control: control, bottoms: set}
		for changed := true; changed; {
			changed = false
			for _, name := range names {
				if set[name] && !w.bottom(p.Rule(name).Def.Expr) {
					set[name], changed = false, true
				}
			}
		}
		for name, b := range set {
			if !b {
				delete(set, name)
			}
		}
		p.bottoms = set
	})
	return p.bottoms
}

// neverMade returns the captures caps of an expression that never matches: they are never made, so
// they are nil, and have no element, as the captures kept beside the tuple have none.
func neverMade(caps []CapIndex) []CapIndex {
	var out []CapIndex
	for _, c := range caps {
		out = MergeCap(out, maybeCap(CapIndex{Name: c.Name, Side: true, T: c.T, Maybe: c.Maybe, Set: c.Set}))
	}
	return out
}

// maybeCap returns c for a capture that may not be made.
func maybeCap(c CapIndex) CapIndex {
	if !c.Maybe {
		c.Maybe, c.Set = true, c.T
	}
	c.T = types.Vopt(c.T)
	return c
}

// MergeCap adds c to caps, joining the types and locations of captures of the same name.
func MergeCap(caps []CapIndex, c CapIndex) []CapIndex {
	for i, o := range caps {
		if o.Name != c.Name {
			continue
		}
		caps[i].T = Vjoin(o.T, c.T)
		if o.Maybe || c.Maybe {
			caps[i].Maybe, caps[i].Set = true, Vjoin(setType(o), setType(c))
		}
		paths := slices.Clone(o.Paths)
		for _, p := range c.Paths {
			if !slices.ContainsFunc(paths, func(q []int) bool { return slices.Equal(p, q) }) {
				paths = append(paths, p)
			}
		}
		caps[i].Paths = paths
		caps[i].Side = o.Side || c.Side
		return caps
	}
	return append(caps, c)
}

// setType is the type of the capture c when it is made.
func setType(c CapIndex) Ty {
	if c.Maybe {
		return c.Set
	}
	return c.T
}

// operatorsRule is the join of the operand values and of the values of the operator lines: an
// infix line without an action is node[(L, Op, R)], a prefix line node[(Op, R)] and a postfix line
// node[(L, Op)], with the captures lhs, op and rhs. With a whole-rule action, it is the type of the
// action instead, which sees the folded value as $value (operatorsFold).
func (v *ValueInference) operatorsRule(rd *grammar.RuleDef, pr *grammar.OperatorsRule) Ty {
	if rd.Action != nil {
		fold := v.operatorsFold(rd, pr)
		env := &valueEnv{locals: map[string]Ty{OperatorsRuleValue: fold}}
		return v.declared(rd, v.term(rd.Action, env))
	}
	var self Ty = RuleTy{Name: rd.Name}
	if rd.Type != nil {
		// The lhs and rhs captures are values of the rule, each of which is checked against the
		// rule's declared type.
		self = v.resolve(rd.Type)
	}
	ts, tuples, operators := v.operatorsLines(rd, pr, self, true)
	if rd.Type != nil {
		// Each line that does not fit is reported on its own.
		d, bad := v.resolve(rd.Type), false
		dn, converts := v.Prog.OperatorsNodeType(rd)
		for i, t := range ts {
			if converts && operators[i] {
				// The node of an operator is converted to the named node type.
				if v.convertible(t, dn) {
					continue
				}
				bad = true
				if tuples[i] {
					v.errorf(rd.Diagnostic(), "operators of %s without an action produce a tuple node, which cannot be converted to %s", rd.Name, d)
				} else {
					v.errorf(rd.Diagnostic(), "operatorsRule %s produces %s, which cannot be converted to %s", rd.Name, v.display(t), d)
				}
				continue
			}
			if v.assignable(t, d) {
				continue
			}
			bad = true
			if tuples[i] {
				v.errorf(rd.Diagnostic(), "operators of %s without an action produce a tuple node, which is not assignable to %s", rd.Name, d)
			} else if converts && !operators[i] {
				v.errorf(rd.Diagnostic(), "operand of %s produces %s, which is not assignable to %s; an operand is not converted: declare the operand rule as %s, or declare %s as a join that includes %s", rd.Name, v.display(t), d, d, rd.Name, v.display(t))
			} else {
				v.errorf(rd.Diagnostic(), "operatorsRule %s produces %s, which is not assignable to %s", rd.Name, v.display(t), d)
			}
		}
		if bad || converts {
			return d
		}
	}
	return v.declared(rd, Vjoin(ts...))
}

// operatorsFold returns the type of the folded value of the Operators Rule rd, which has a
// whole-rule action: the join of its line values, where $lhs and $rhs have that join type. The
// declared type of the rule is the type of the action's result, so the fold has no declared type
// and no name to refer to itself by; its type is the least fixed point of the lines, computed from
// _|_ round by round. A fold that is still growing after maxCycleRounds (the tuple nodes of
// operators without actions nest without bound) gives $lhs and $rhs the widest type its values can
// have: node[_] when every line makes a node, and any otherwise.
func (v *ValueInference) operatorsFold(rd *grammar.RuleDef, pr *grammar.OperatorsRule) Ty {
	quiet := v.Quiet
	v.Quiet = true
	var self Ty = TyBottom
	stable := false
	for round := 0; round <= maxCycleRounds; round++ {
		ts, _, _ := v.operatorsLines(rd, pr, self, false)
		next := Vjoin(ts...)
		if reflect.DeepEqual(next, self) {
			stable = true
			break
		}
		self = next
	}
	if !stable {
		ts, _, _ := v.operatorsLines(rd, pr, TyBottom, false)
		self = NodeTy{Value: TyTop}
		for _, t := range ts {
			if !v.assignable(t, self) {
				self = TyAny
				break
			}
		}
	}
	v.Quiet = quiet
	// The last typing reports the errors of the lines and records their types.
	ts, _, _ := v.operatorsLines(rd, pr, self, false)
	return Vjoin(ts...)
}

// operatorsLines types the skip and the lines of the Operators Rule rd, with $lhs and $rhs of type
// self, and returns the value of each line, with the indexes of the lines that make the tuple node
// of an operator without an action (tuples) and of the operator lines (operators). If checked, the
// value of each line is checked against the rule's declared type.
func (v *ValueInference) operatorsLines(rd *grammar.RuleDef, pr *grammar.OperatorsRule, self Ty, checked bool) (ts []Ty, tuples, operators map[int]bool) {
	tuples = map[int]bool{}    // the lines whose value is the tuple node of an operator without an action
	operators = map[int]bool{} // the lines of operators, whose nodes a named node type converts
	if pr.Skip != nil {
		v.scope(pr.Skip, nil)
		v.predicates(pr.Skip, &valueEnv{})
	}
	for _, o := range pr.Operands {
		env := &valueEnv{caps: tupleNode(v.elems(o.Expr)).Caps}
		v.scope(o.Expr, env.caps)
		v.predicates(o.Expr, env)
		if o.Action != nil {
			ts = append(ts, v.term(o.Action, env))
		} else {
			ts = append(ts, v.ScopeValue(o.Expr))
		}
		if checked {
			v.checkResult(o, rd, ts[len(ts)-1])
		}
	}
	for _, l := range pr.Levels {
		for _, o := range l.Operators {
			v.control.Poll()
			op, ok := v.single(o.Expr)
			if !ok {
				op = ValueElem{t: NodeTy{Value: TupleTy{Elems: []Ty{}}}}
			}
			v.scope(o.Expr, op.caps)
			v.predicates(o.Expr, &valueEnv{caps: op.caps})
			if o.Action != nil {
				locals := map[string]Ty{"op": op.t}
				if o.Kind != grammar.Prefix {
					locals["lhs"] = self
				}
				if o.Kind != grammar.Postfix {
					locals["rhs"] = self
				}
				operators[len(ts)] = true
				ts = append(ts, v.term(o.Action, &valueEnv{caps: op.caps, locals: locals}))
				if checked {
					v.checkResult(o, rd, ts[len(ts)-1])
				}
				continue
			}
			var elems []ValueElem
			switch o.Kind {
			case "prefix":
				elems = []ValueElem{{t: op.t, caps: []CapIndex{{Name: "op", Paths: [][]int{{}}, T: op.t}}}, {t: self, caps: []CapIndex{{Name: "rhs", Paths: [][]int{{}}, T: self}}}}
			case "postfix":
				elems = []ValueElem{{t: self, caps: []CapIndex{{Name: "lhs", Paths: [][]int{{}}, T: self}}}, {t: op.t, caps: []CapIndex{{Name: "op", Paths: [][]int{{}}, T: op.t}}}}
			default:
				elems = []ValueElem{{t: self, caps: []CapIndex{{Name: "lhs", Paths: [][]int{{}}, T: self}}}, {t: op.t, caps: []CapIndex{{Name: "op", Paths: [][]int{{}}, T: op.t}}}, {t: self, caps: []CapIndex{{Name: "rhs", Paths: [][]int{{}}, T: self}}}}
			}
			tuples[len(ts)] = true
			operators[len(ts)] = true
			ts = append(ts, tupleNode(elems))
		}
	}
	return ts, tuples, operators
}

// UngroupExpr returns e without its enclosing parentheses.
func UngroupExpr(e grammar.Expr) grammar.Expr {
	for {
		g, ok := e.(*grammar.Group)
		if !ok {
			return e
		}
		e = g.Expr
	}
}

// Type returns the type of the element.
func (e ValueElem) Type() Ty { return e.t }

// Captures returns the captures in the element, with paths relative to it.
func (e ValueElem) Captures() []CapIndex { return e.caps }
