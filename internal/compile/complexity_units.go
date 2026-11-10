package compile

import (
	"fmt"
	"maps"
	"slices"

	"zeroparse.dev/zp/grammar"
)

// endKind says what an evaluation examines after the last item of a continuation.
type endKind uint8

const (
	// endNone: nothing more, as at the end of a lookahead or of a unit's own extent.
	endNone endKind = iota
	// endRule: what follows the calls of the rule (its FOLLOW).
	endRule
	// endOps: the operator parts of the Operators Rule of the rule, then what follows its calls.
	endOps
)

// cont is a continuation: the expressions that an evaluation evaluates next, in order. After its
// items it continues with next, or, when next is nil, as end says.
type cont struct {
	items []grammar.Expr
	next  *cont
	end   endKind
	rule  string
	// inner marks the end of a body that a path entered by looking through a call of the rule,
	// after which the path continues after that call: it does not leave the body in which the
	// path started.
	inner bool
}

// then returns the continuation that evaluates items and then c.
func (c *cont) then(items ...grammar.Expr) *cont { return &cont{items: items, next: c} }

// rest returns the continuation after the first item of c.
func (c *cont) rest() *cont {
	return &cont{items: c.items[1:], next: c.next, end: c.end, rule: c.rule, inner: c.inner}
}

// source is a scanning source.
type source struct {
	rule string
	pos  grammar.Pos
	what string
	scan *grammar.Repeat // the repetition that is the source, if one is
}

func (s *source) String() string { return fmt.Sprintf("%s at %s", s.what, at(s.rule, s.pos)) }

// site is a call of a rule.
type site struct {
	caller, callee string
	ref            *grammar.Ref
	cont           *cont // what follows the call
	// mayStart: the call can run before its caller's body consumes input; mustStart: it always
	// does.
	mayStart, mustStart bool
	repeated            string     // why the call is evaluated repeatedly within its caller, or ""
	seqs                []seqFrame // the sequences around the call
	units               []*unit    // the units around the call in its caller, outermost first
}

// unitKind is the kind of a unit (an expression after whose failure the parser continues where
// it started).
type unitKind uint8

const (
	unitAlt unitKind = iota
	unitIteration
	unitOption
	unitLookahead
	unitRecover
	unitOperand
	unitOperator
)

func (u unitKind) String() string {
	return [...]string{"the alternative", "an iteration of the repetition", "the option", "the lookahead",
		"the base of the #recover", "the operand", "the operator part"}[u]
}

// contSpec is one continuation of a unit, with a description for witnesses.
type contSpec struct {
	paths []*cont
	what  string
}

type unit struct {
	id    int
	kind  unitKind
	rule  string
	expr  grammar.Expr // the expression that holds the unit, for its position
	pos   grammar.Pos
	self  []*cont
	conts []contSpec
	// repeated says why the construct that holds the unit is evaluated more than once in a parse,
	// or is "".
	repeated string
	// growing marks an alternative that begins with a left call; choice and index locate it.
	growing bool
	// afterInput marks a growing unit that the expressions before it can reach after consuming
	// input, where its left call is an ordinary call and nothing grows.
	afterInput bool
	choice     *grammar.Choice
	index      int
	after      *cont // what follows the choice
	cycle      int
	// unsupported is set for a unit whose shape the classifier does not analyze.
	unsupported string

	src   *source         // the first scanning source inside the unit itself
	srcs  []*source       // the scanning sources that the walk found inside the unit itself
	calls map[string]bool // the rules that the unit calls
	inner []*unit         // the units inside it, in the same rule

	hasSource bool
	nc        bool // contains a scanning source and is not committing
	witness   string
}

type wctx struct {
	rule                string
	cont                *cont
	mayStart, mustStart bool
	repeated            string
	skipSites           bool // inside a skip expression whose unbounded repetitions are reused skip-run sites
	open                []*unit
	inNot               bool
	seqs                []seqFrame // the sequences around the expression, for bindings
	inRepeat            bool       // inside the element of a repetition of the rule
	ops                 bool       // inside an Operators Rule
	// strictFrom is the first of seqs that the expression is in through groups, options,
	// choices, captures, discards and atomic expressions only, for the growing-variable check.
	strictFrom int
}

type seqFrame struct {
	seq   *grammar.Seq
	index int
}

func (c wctx) with(cn *cont) wctx { c.cont = cn; return c }

// againOf returns the repetition that stands, in a path, for the iterations of e after one: e
// again, without a minimum. It is a scanning source when e is one, so a path that has entered an
// iteration of a scanning repetition still contains the source. Each repetition has one. The
// walk marks e as a source before it first calls againOf(e), and leads and wordExcludes run after
// every rule is walked, so the mark is never missed.
func (k *classifier) againOf(e *grammar.Repeat) *grammar.Repeat {
	a := k.again[e]
	if a == nil {
		a = &grammar.Repeat{SourceInfo: e.SourceInfo, Expr: e.Expr, Min: 0, Max: -1}
		k.again[e] = a
	}
	if k.srcExprs[e] {
		k.srcExprs[a] = true
	}
	return a
}

// walkRule walks the body and the action of a reachable rule, collecting call sites, scanning
// sources, units, bindings and copying operations.
func (k *classifier) walkRule(r *grammar.RuleDef) {
	c := wctx{rule: r.Name, cont: &cont{end: endRule, rule: r.Name}, mayStart: true, mustStart: true}
	k.walk(r.Expr, c)
	if r.Action != nil {
		k.walkTerm(r.Action, termCtx{rule: r.Name, c: c, action: true, value: true})
	}
}

func (k *classifier) collectCaptures(r *grammar.RuleDef) {
	m := map[string][]grammar.Expr{}
	WalkExpr(r.Expr, func(e grammar.Expr) {
		if cp, ok := e.(*grammar.Capture); ok {
			m[cp.Name] = append(m[cp.Name], cp.Expr)
		}
	})
	k.captures[r.Name] = m
}

// addSource records a scanning source found in the walk.
func (k *classifier) addSource(c wctx, pos grammar.Pos, what string) *source {
	s := &source{rule: c.rule, pos: pos, what: what}
	k.walked[c.rule] = append(k.walked[c.rule], s)
	for _, u := range c.open {
		u.srcs = append(u.srcs, s)
	}
	return s
}

// settleSources keeps the first of the sources that the walk found in each rule and unit, among
// those that remain scanning sources.
func (k *classifier) settleSources() {
	live := func(ss []*source) *source {
		for _, s := range ss {
			if s.scan == nil || k.srcExprs[s.scan] {
				return s
			}
		}
		return nil
	}
	for r, ss := range k.walked {
		if s := live(ss); s != nil {
			k.local[r] = s
		}
	}
	for _, u := range k.units {
		if u.src == nil {
			u.src = live(u.srcs)
		}
	}
}

func (k *classifier) newUnit(kind unitKind, c wctx, e grammar.Expr, self []*cont, conts []contSpec) *unit {
	u := &unit{id: len(k.units), kind: kind, rule: c.rule, expr: e, pos: exprPos(e), self: self, conts: conts,
		repeated: c.repeated, calls: map[string]bool{}}
	for _, o := range c.open {
		o.inner = append(o.inner, u)
	}
	k.units = append(k.units, u)
	return u
}

func (c wctx) push(u *unit) wctx {
	c.open = append(c.open[:len(c.open):len(c.open)], u)
	return c
}

// consumesNothing reports whether e never consumes input.
func (k *classifier) consumesNothing(e grammar.Expr) bool {
	switch e := e.(type) {
	case *grammar.Predicate, *grammar.Cut, *grammar.Top, *grammar.Bottom, *grammar.BeginInput, *grammar.EndInput,
		*grammar.BeginLine, *grammar.EndLine, *grammar.And, *grammar.Not:
		return true
	case *grammar.Literal:
		return e.Value == ""
	case *grammar.Group:
		return k.consumesNothing(e.Expr)
	case *grammar.Seq:
		for _, it := range e.Items {
			if !k.consumesNothing(it) {
				return false
			}
		}
		return true
	}
	return false
}

func (k *classifier) walk(e grammar.Expr, c wctx) {
	k.control.Poll()
	switch e.(type) {
	case *grammar.Seq, *grammar.Choice, *grammar.Group, *grammar.Optional, *grammar.Capture, *grammar.Discard,
		*grammar.Atomic, *grammar.Ref, *grammar.Predicate:
	default:
		c.strictFrom = len(c.seqs)
	}
	switch e := e.(type) {
	case *grammar.Ref:
		k.walkRef(e, c)
	case *grammar.Seq:
		may, must := c.mayStart, c.mustStart
		for i, it := range e.Items {
			cc := c.with(c.cont.then(e.Items[i+1:]...))
			cc.mayStart, cc.mustStart = may, must
			cc.seqs = append(c.seqs[:len(c.seqs):len(c.seqs)], seqFrame{e, i})
			k.walk(it, cc)
			may = may && k.nullable(it)
			must = must && k.consumesNothing(it)
		}
	case *grammar.Choice:
		for i, alt := range e.Alts {
			cc := c
			if i < len(e.Alts)-1 {
				var conts []contSpec
				if !k.cutCommits(alt) {
					for _, later := range e.Alts[i+1:] {
						conts = append(conts, contSpec{paths: []*cont{c.cont.then(later)},
							what: fmt.Sprintf("the later alternative %s at %s", k.show(later), at(c.rule, exprPos(later)))})
					}
				}
				growing := false
				if k.an.cyclic[c.rule] && c.mayStart {
					if first := k.firstEvaluated(alt); first != nil && k.sameCycle(c.rule, first.Name) && first.Level == "" {
						growing = true
					}
				}
				if s, cut := topCut(alt); cutPrefixesOn && !growing && len(conts) > 0 && cut > 0 {
					prefix := s.Items[:cut]
					u := k.newUnit(unitAlt, c, &grammar.Seq{SourceInfo: s.SourceInfo, Items: prefix},
						[]*cont{{items: prefix}}, conts)
					u.choice, u.index, u.after = e, i, c.cont
					k.walkCutSeq(s, c, u, cut)
					continue
				}
				u := k.newUnit(unitAlt, c, alt, []*cont{{items: []grammar.Expr{alt}}}, conts)
				u.choice, u.index, u.after = e, i, c.cont
				if growing {
					u.growing, u.cycle = true, k.an.cycle[c.rule]
					u.afterInput = !c.mustStart
				}
				cc = c.push(u)
			}
			k.walk(alt, cc)
		}
	case *grammar.Repeat:
		if e.Max == 0 {
			return
		}
		if e.Max < 0 && !c.skipSites && !k.runSite(e) {
			k.addSource(c, exprPos(e), "the repetition "+k.show(e)).scan = e
			k.srcExprs[e] = true
		}
		u := k.newUnit(unitIteration, c, e, []*cont{{items: []grammar.Expr{e.Expr}}},
			[]contSpec{{paths: []*cont{c.cont}, what: "what follows the repetition " + k.show(e)}})
		k.leftUnit(u, c, e.Expr)
		again := k.againOf(e)
		cc := c.push(u).with(c.cont.then(again))
		cc.mustStart = false
		cc.inRepeat = true
		if e.Max < 0 || e.Max > 1 {
			cc.repeated = "the repetition " + k.show(e) + " at " + at(c.rule, exprPos(e))
		}
		k.walk(e.Expr, cc)
	case *grammar.Optional:
		u := k.newUnit(unitOption, c, e, []*cont{{items: []grammar.Expr{e.Expr}}},
			[]contSpec{{paths: []*cont{c.cont}, what: "what follows the option " + k.show(e)}})
		k.leftUnit(u, c, e.Expr)
		k.walk(e.Expr, c.push(u))
	case *grammar.And:
		k.walkLookahead(e, e.Expr, c, false)
	case *grammar.Not:
		k.walkLookahead(e, e.Expr, c, true)
	case *grammar.Recovery:
		k.walkRecovery(e, e.Expr, e.Skip, c)
	case *grammar.Attributed:
		if skip := recoverSkip(e); skip != nil {
			k.walkRecovery(e, e.Expr, skip, c)
			return
		}
		k.walk(e.Expr, c)
	case *grammar.Group:
		k.walk(e.Expr, c)
	case *grammar.Capture:
		k.walk(e.Expr, c)
	case *grammar.Discard:
		k.walk(e.Expr, c)
	case *grammar.Atomic:
		k.walk(e.Expr, c)
	case *grammar.Stream:
		k.walk(e.Expr, c)
	case *grammar.InlineAction:
		k.walk(e.Expr, c)
		if e.Action != nil {
			k.walkTerm(e.Action, termCtx{rule: c.rule, c: c, action: true, holder: e, value: k.ruleValue(c.rule, e)})
		}
	case *grammar.Predicate:
		k.walkTerm(e.Term, termCtx{rule: c.rule, c: c, pred: e})
	case *grammar.OperatorsRule:
		k.walkOperators(e, c)
	}
}

// leftUnit marks a unit other than an alternative that begins with a left call: the classifier
// analyzes growing units only as alternatives.
func (k *classifier) leftUnit(u *unit, c wctx, e grammar.Expr) {
	if k.an.cyclic[c.rule] && c.mayStart {
		if first := k.firstEvaluated(e); first != nil && k.sameCycle(c.rule, first.Name) {
			u.unsupported = u.kind.String() + " begins with a left call"
		}
	}
}

func (k *classifier) walkLookahead(e, body grammar.Expr, c wctx, not bool) {
	u := k.newUnit(unitLookahead, c, e, []*cont{{items: []grammar.Expr{body}}},
		[]contSpec{{paths: []*cont{c.cont}, what: "what follows the lookahead " + k.show(e)}})
	k.leftUnit(u, c, body)
	cc := c.push(u).with(&cont{})
	cc.inNot = c.inNot || not
	cc.seqs, cc.strictFrom = nil, 0
	k.walk(body, cc)
}

func (k *classifier) walkRecovery(e, base, skip grammar.Expr, c wctx) {
	shared := reused(skip)
	if !shared {
		k.addSource(c, exprPos(e), "the skip expression "+k.show(skip)+" of #recover")
		k.srcExprs[e] = true
	}
	u := k.newUnit(unitRecover, c, e, []*cont{{items: []grammar.Expr{base}}},
		[]contSpec{{paths: []*cont{c.cont.then(skip)}, what: "the skip expression " + k.show(skip)}})
	k.leftUnit(u, c, base)
	k.walk(base, c.push(u))
	if shared {
		c.skipSites = true
	}
	k.walk(skip, c)
}

func (k *classifier) walkRef(e *grammar.Ref, c wctx) {
	if k.byName[e.Name] == nil {
		return
	}
	for _, u := range c.open {
		u.calls[e.Name] = true
	}
	k.sites = append(k.sites, &site{caller: c.rule, callee: e.Name, ref: e, cont: c.cont, mayStart: c.mayStart,
		mustStart: c.mustStart, repeated: c.repeated, seqs: c.seqs, units: c.open})
	// A growth of the callee's cycle at the position of the call encloses the evaluation of the
	// caller, so the caller is on the cycle and the call is a left call of its body.
	if !k.sameCycle(c.rule, e.Name) || !c.mayStart {
		k.outside[e] = true
	}
	if k.an.cyclic[e.Name] && (!k.sameCycle(c.rule, e.Name) || !c.mustStart) {
		k.addSource(c, exprPos(e), "the growth of the left-recursive rule "+e.Name+" at a new position")
		k.srcExprs[e] = true
	}
}

// topCut returns the top-level sequence of an alternative and the index of the first #cut in it,
// or 0.
func topCut(alt grammar.Expr) (*grammar.Seq, int) {
	s, ok := ungroup(alt).(*grammar.Seq)
	if !ok {
		return nil, 0
	}
	for i, it := range s.Items {
		if _, ok := it.(*grammar.Cut); ok {
			return s, i
		}
	}
	return nil, 0
}

// walkCutSeq walks the top-level sequence of an alternative with a #cut at index cut: the items
// before the cut are in the unit u, and the items after it are not, as a failure after the cut
// fails the choice.
func (k *classifier) walkCutSeq(e *grammar.Seq, c wctx, u *unit, cut int) {
	may, must := c.mayStart, c.mustStart
	inside := c.push(u)
	for i, it := range e.Items {
		base := c
		if i < cut {
			base = inside
		}
		cc := base.with(c.cont.then(e.Items[i+1:]...))
		cc.mayStart, cc.mustStart = may, must
		cc.seqs = append(c.seqs[:len(c.seqs):len(c.seqs)], seqFrame{e, i})
		k.walk(it, cc)
		may = may && k.nullable(it)
		must = must && k.consumesNothing(it)
	}
}

// cutCommits reports whether every way in which the alternative can fail lies after a #cut in it,
// so that the later alternatives are not tried after it fails.
func (k *classifier) cutCommits(alt grammar.Expr) bool {
	s, ok := ungroup(alt).(*grammar.Seq)
	if !ok {
		return false
	}
	for _, it := range s.Items {
		if _, ok := it.(*grammar.Cut); ok {
			return true
		}
		if !k.an.AlwaysSucceeds(it) {
			return false
		}
	}
	return false
}

// firstEvaluated returns the call that e evaluates before anything else, or nil.
func (k *classifier) firstEvaluated(e grammar.Expr) *grammar.Ref {
	for {
		switch x := e.(type) {
		case *grammar.Ref:
			return x
		case *grammar.Group:
			e = x.Expr
		case *grammar.Capture:
			e = x.Expr
		case *grammar.Discard:
			e = x.Expr
		case *grammar.Atomic:
			e = x.Expr
		case *grammar.Stream:
			e = x.Expr
		case *grammar.Seq:
			if len(x.Items) == 0 {
				return nil
			}
			e = x.Items[0]
		default:
			return nil
		}
	}
}

// walkOperators walks an Operators Rule. The rule applies operators, which is a scanning source;
// each operand item other than the last, and each operator part with its right operand, is a
// unit. Its continuations are approximated by the other operand items or operator parts that the
// rule can try at the same position, and by what follows the Operators Rule expression.
func (k *classifier) walkOperators(pr *grammar.OperatorsRule, c wctx) {
	hasOps := false
	for _, l := range pr.Levels {
		hasOps = hasOps || len(l.Operators) > 0
	}
	if hasOps {
		k.addSource(c, exprPos(pr), "the operators that the Operators Rule applies")
		k.srcExprs[pr] = true
	}
	end := &cont{end: endOps, rule: c.rule}
	inside := c.with(end)
	inside.mustStart = false
	inside.ops = true
	inside.seqs, inside.strictFrom = nil, 0
	if hasOps {
		inside.repeated = "the Operators Rule at " + at(c.rule, exprPos(pr))
	}
	withSkip := func(e grammar.Expr, more ...grammar.Expr) *cont {
		items := []grammar.Expr{}
		if pr.Skip != nil {
			items = append(items, k.skipItem(pr))
		}
		return &cont{items: append(append(items, e), more...)}
	}
	// An Operators Rule is a whole rule body: what follows it is what follows the rule. The other
	// operator parts that the rule can try after an operand are continuations of their own.
	follow := contSpec{paths: []*cont{{end: endRule, rule: c.rule}}, what: "what follows the Operators Rule"}
	rhs := &grammar.Ref{Name: c.rule, Level: rhsLevel}
	skipCalls := map[string]bool{}
	if pr.Skip != nil {
		for _, n := range AllCalls(pr.Skip, nil) {
			skipCalls[n] = true
		}
		k.walk(pr.Skip, inside)
	}
	enter := func(u *unit) wctx {
		for n := range skipCalls {
			u.calls[n] = true
		}
		return inside.push(u)
	}
	skipSource := pr.Skip != nil && (k.containsSourceSyntactic(pr.Skip))
	var operandUnits []*unit
	operandCalls := map[string]bool{}
	for i, o := range pr.Operands {
		for _, n := range AllCalls(o.Expr, nil) {
			operandCalls[n] = true
		}
		cc := inside
		if i < len(pr.Operands)-1 {
			var conts []contSpec
			for _, later := range pr.Operands[i+1:] {
				conts = append(conts, contSpec{paths: []*cont{withSkip(later.Expr)},
					what: fmt.Sprintf("the later operand %s at %s", k.show(later.Expr), at(c.rule, exprPos(later.Expr)))})
			}
			u := k.newUnit(unitOperand, c, o.Expr, []*cont{withSkip(o.Expr)}, append(conts, follow))
			if skipSource {
				u.src = &source{rule: c.rule, pos: exprPos(pr.Skip), what: "the skip expression " + k.show(pr.Skip)}
			}
			cc = enter(u)
		}
		cc.mayStart = c.mayStart
		before := len(k.units)
		k.walk(o.Expr, cc)
		for _, v := range k.units[before:] {
			if v.rule == c.rule {
				operandUnits = append(operandUnits, v)
			}
		}
		if i < len(pr.Operands)-1 {
			operandUnits = append(operandUnits, k.units[before-1])
		}
		if o.Action != nil {
			k.walkTerm(o.Action, termCtx{rule: c.rule, c: inside, action: true, value: true})
		}
	}
	type part struct {
		level int
		op    *grammar.OperatorsRuleOperator
	}
	var parts []part
	prefixes := false
	for li, l := range pr.Levels {
		for _, op := range l.Operators {
			parts = append(parts, part{li, op})
			prefixes = prefixes || op.Kind == grammar.Prefix
		}
	}
	for _, p := range parts {
		var self *cont
		if p.op.Kind == grammar.Postfix {
			self = withSkip(p.op.Expr)
		} else {
			self = withSkip(p.op.Expr, rhs)
		}
		var conts []contSpec
		for _, q := range parts {
			if q.op == p.op || (q.op.Kind == grammar.Prefix) != (p.op.Kind == grammar.Prefix) {
				continue
			}
			conts = append(conts, contSpec{paths: []*cont{withSkip(q.op.Expr)},
				what: fmt.Sprintf("the operator part %s at %s", k.show(q.op.Expr), at(c.rule, exprPos(q.op.Expr)))})
		}
		if p.op.Kind == grammar.Prefix {
			for _, o := range pr.Operands {
				conts = append(conts, contSpec{paths: []*cont{withSkip(o.Expr)},
					what: fmt.Sprintf("the operand %s at %s", k.show(o.Expr), at(c.rule, exprPos(o.Expr)))})
			}
		}
		u := k.newUnit(unitOperator, c, p.op.Expr, []*cont{self}, append(conts, follow))
		if p.op.Kind != grammar.Prefix {
			k.retried = append(k.retried, retriedPart{u, c.rule, end})
		}
		applies := false
		switch p.op.Kind {
		case grammar.Prefix:
			applies = true
		case grammar.Infix:
			applies = prefixes
			for _, q := range parts {
				if q.op.Kind != grammar.Prefix && (q.level > p.level || q.level == p.level && p.op.Assoc == grammar.AssocRight) {
					applies = true
				}
			}
		}
		if p.op.Kind != grammar.Postfix {
			// The right operand parses operands again, and operators too when it can apply them.
			if applies {
				u.calls[c.rule] = true
			} else {
				for n := range operandCalls {
					u.calls[n] = true
				}
				u.inner = append(u.inner, operandUnits...)
			}
		}
		if applies {
			u.src = &source{rule: c.rule, pos: exprPos(p.op.Expr), what: "the right operand of the operator part " + k.show(p.op.Expr) + ", which can apply operators"}
		} else if skipSource {
			u.src = &source{rule: c.rule, pos: exprPos(pr.Skip), what: "the skip expression " + k.show(pr.Skip)}
		}
		cc := enter(u)
		cc.mayStart = c.mayStart && (p.op.Kind == grammar.Prefix || k.operandsNullable(c.rule))
		if p.op.Kind == grammar.Postfix {
			cc.cont = end
		} else {
			cc.cont = end.then(rhs)
		}
		k.walk(p.op.Expr, cc)
		if p.op.Action != nil {
			k.walkTerm(p.op.Action, termCtx{rule: c.rule, c: inside, action: true, value: true})
		}
	}
}

// containsSourceSyntactic reports whether e has a repetition without an upper bound, a #recover
// or an Operators Rule in it, without following calls.
func (k *classifier) containsSourceSyntactic(e grammar.Expr) bool {
	found := false
	WalkExpr(e, func(x grammar.Expr) {
		switch x := x.(type) {
		case *grammar.Repeat:
			found = found || x.Max < 0
		case *grammar.Recovery, *grammar.OperatorsRule:
			found = true
		}
	})
	return found
}

// computeRuleSources finds, for each rule, a scanning source that it contains through calls.
func (k *classifier) computeRuleSources() {
	for n, s := range k.local {
		k.ruleSrc[n] = s
	}
	for changed := true; changed; {
		changed = false
		for _, r := range k.rules {
			if !k.reach[r.Name] || k.ruleSrc[r.Name] != nil {
				continue
			}
			for _, c := range k.calls[r.Name] {
				if s := k.ruleSrc[c]; s != nil {
					k.ruleSrc[r.Name] = s
					changed = true
					break
				}
			}
		}
	}
	for _, u := range k.units {
		if u.src != nil {
			u.hasSource = true
			continue
		}
		for _, c := range slices.Sorted(maps.Keys(u.calls)) {
			if s := k.ruleSrc[c]; s != nil {
				u.src, u.hasSource = s, true
				break
			}
		}
	}
}

// --- First characters of continuations and FOLLOW ---

// opsAfter is the first characters of the operator parts of the Operators Rule of rule, which
// can follow an operand.
func (k *classifier) opsAfter(rule string) charset {
	pr := k.operators(rule)
	if pr == nil {
		return nil
	}
	var s charset
	for _, l := range pr.Levels {
		for _, op := range l.Operators {
			if op.Kind == grammar.Prefix {
				continue
			}
			items := []grammar.Expr{k.skipItem(pr), op.Expr}
			if op.Kind == grammar.Infix {
				items = append(items, &grammar.Ref{Name: rule, Level: rhsLevel})
			}
			t, _ := k.firstItems(items)
			s = s.union(t)
		}
	}
	return s
}

// firstItems returns the first characters of the items in sequence and whether all can match
// without consuming input.
func (k *classifier) firstItems(items []grammar.Expr) (charset, bool) {
	var s charset
	for _, it := range items {
		if it == nil {
			continue
		}
		s = s.union(k.first(it))
		if !k.nullable(it) {
			return s, false
		}
	}
	return s, true
}

// firstCont returns the first characters of a continuation, with follow giving the FOLLOW set of a
// rule. ends receives the rules whose FOLLOW it reaches, if not nil.
func (k *classifier) firstCont(c *cont, follow func(string) charset, ends map[string]bool) charset {
	var s charset
	for c != nil {
		k.control.Poll()
		t, null := k.firstItems(c.items)
		s = s.union(t)
		if !null {
			return s
		}
		if c.next != nil {
			c = c.next
			continue
		}
		switch c.end {
		case endOps:
			s = s.union(k.opsAfter(c.rule))
			fallthrough
		case endRule:
			if ends != nil {
				ends[c.rule] = true
			}
			if follow != nil {
				s = s.union(follow(c.rule))
			}
		}
		return s
	}
	return s
}

func (k *classifier) firstPaths(paths []*cont, follow func(string) charset) charset {
	var s charset
	for _, p := range paths {
		s = s.union(k.firstCont(p, follow, nil))
	}
	return s
}

// computeFollow computes FOLLOW(r) for every reachable rule r: the first characters of what the
// parser can evaluate after a call of r, through the ends of the callers' bodies. The start rule
// is followed by the end of the input, which examines nothing.
func (k *classifier) computeFollow() {
	k.follow = k.followSets(func(*site) bool { return true })
}

// followSets computes FOLLOW over the call sites that keep accepts.
func (k *classifier) followSets(keep func(*site) bool) map[string]charset {
	type contrib struct {
		static charset
		ends   map[string]bool
	}
	byCallee := map[string][]contrib{}
	for _, s := range k.sites {
		if !keep(s) {
			continue
		}
		ends := map[string]bool{}
		st := k.firstCont(s.cont, nil, ends)
		byCallee[s.callee] = append(byCallee[s.callee], contrib{st, ends})
	}
	out := map[string]charset{}
	for changed := true; changed; {
		changed = false
		for callee, cs := range byCallee {
			k.control.Poll()
			s := out[callee]
			for _, c := range cs {
				s = s.union(c.static)
				for e := range c.ends {
					s = s.union(out[e])
				}
			}
			if !s.equal(out[callee]) {
				out[callee] = s
				changed = true
			}
		}
	}
	return out
}

// keepFor returns the call sites through which a later step of a growth of the cycle continues at
// the end of a rule body (growthGoesOn): every site, or, with growth, the sites other than the
// seed reads of the cycle. A call of a leader of the cycle at the position where it grows reads
// the seed and is part of the growth; the step goes on after the call that started it, which is
// such a call in the body of a rule of the cycle other than a leader that starts growths
// (startsGrowth).
//
// The continuations of units do not drop the seed reads: after a growth that a call in the body
// of a leader of the cycle started, the end of that body is followed by the next step of the
// leader's own growth, which reads its seed there.
func (k *classifier) keepFor(cycle int, growth bool) func(*site) bool {
	if !growth {
		return nil
	}
	return func(s *site) bool {
		return !(k.an.cyclic[s.caller] && k.an.cycle[s.caller] == cycle && k.an.leaders[s.callee] &&
			k.an.cycle[s.callee] == cycle && s.mustStart) || k.startsGrowth(s.caller)
	}
}

// followFor returns the FOLLOW function of the sites that keepFor keeps.
func (k *classifier) followFor(cycle int, growth bool) func(string) charset {
	if !growth {
		return func(r string) charset { return k.follow[r] }
	}
	m := k.growing[cycle]
	if m == nil {
		m = k.followSets(k.keepFor(cycle, true))
		k.growing[cycle] = m
	}
	return func(r string) charset { return m[r] }
}

// --- Shared calls ---

// lead is the first call that a path evaluates before it examines input, with the rest of the path
// after the call. A lead without a call is a path that examines input first or cannot be followed;
// a lead that is done examines nothing more.
type lead struct {
	call *grammar.Ref
	lit  *grammar.Literal
	rest *cont
	done bool
	// up: the path reaches the lead after the end of a rule body, in what follows a call whose
	// evaluations can nest without a bound and end at one position (nestsEnd, nestsOps).
	up bool
	// left: the path reaches the lead after the end of the body in which it started.
	left bool
	// via: the calls whose bodies the path looked through to reach the lead.
	via *refList
}

// refList is a list of calls, the last one looked through first.
type refList struct {
	ref  *grammar.Ref
	next *refList
}

type endKey struct {
	end  endKind
	rule string
}

const (
	leadBudget = 4096 // steps to find the leads of one set of paths
	leadMax    = 256  // the most leads of one set of paths
)

// leads finds the first calls of the paths. It looks through calls that are not shared, by
// evaluating their bodies, through expressions that examine no input, and through the ends of rule
// bodies, by every call site of the rule. With only set, it looks through every call but one
// of that rule, and records in seen the shareable calls that it looks through. When from is set,
// each path is the rest after the lead in from at the same index, and starts with what that lead
// carries: whether the path left a body through a call that nests (see lead.up), whether it left
// the body in which it started, and the calls that it looked through, before the shared calls
// that strip removed from it.
func (k *classifier) leads(paths []*cont, from []lead, only *string, seen map[string]bool) []lead {
	budget := leadBudget
	var out []lead
	// ended are the rule ends already followed: the paths after them are the same each time. The
	// end of a body that the path looked into is followed once for each list of calls through
	// which the path entered it, which the leads after it carry.
	type upKey struct {
		endKey
		up, left bool
		via      *refList
	}
	ended := map[upKey]bool{}
	type state struct {
		up, left bool
		via      *refList
	}
	var visit func(p *cont, st state)
	visit = func(p *cont, st state) {
		up := st.up
		for {
			k.control.Poll()
			budget--
			if budget < 0 || len(out) > leadMax {
				out = append(out, lead{})
				return
			}
			if p == nil {
				out = append(out, lead{done: true, up: up, left: st.left, via: st.via})
				return
			}
			if len(p.items) == 0 {
				if p.next != nil {
					p = p.next
					continue
				}
				if p.end != endNone {
					key := upKey{endKey: endKey{p.end, p.rule}, up: up, left: st.left}
					if p.inner {
						key.via = st.via
					}
					if ended[key] {
						return
					}
					ended[key] = true
				}
				// Past the end of a body that the path looked into, it is still in the body in
				// which it started.
				left := st.left || !p.inner
				switch p.end {
				case endNone:
					out = append(out, lead{done: true, up: up, left: st.left, via: st.via})
				case endOps:
					k.opsLeads(p.rule, p.inner, func(c *cont) { visit(c, state{k.nestsOps(p.rule), left, st.via}) })
					k.endLeads(p.rule, func(c *cont, s *site) { visit(c, state{k.nestsEnd(s), left, st.via}) })
				case endRule:
					k.endLeads(p.rule, func(c *cont, s *site) { visit(c, state{k.nestsEnd(s), left, st.via}) })
				}
				return
			}
			rest := p.rest()
			switch e := p.items[0].(type) {
			case nil:
				p = rest
			case *grammar.Group:
				p = rest.then(e.Expr)
			case *grammar.Capture:
				p = rest.then(e.Expr)
			case *grammar.Discard:
				p = rest.then(e.Expr)
			case *grammar.Atomic:
				p = rest.then(e.Expr)
			case *grammar.Stream:
				p = rest.then(e.Expr)
			case *grammar.InlineAction:
				p = rest.then(e.Expr)
			case *grammar.Seq:
				p = rest.then(e.Items...)
			case *grammar.Attributed:
				if recoverSkip(e) != nil || hasPragma(e, "missing") {
					out = append(out, lead{})
					return
				}
				p = rest.then(e.Expr)
			case *grammar.Predicate, *grammar.Cut, *grammar.Top, *grammar.BeginInput, *grammar.BeginLine:
				p = rest
			case *grammar.EndInput:
				// At the end of the input nothing more is examined.
				out = append(out, lead{done: true, up: up, left: st.left, via: st.via})
				return
			case *grammar.Literal:
				if e.Value != "" {
					if only == nil {
						out = append(out, lead{lit: e, rest: rest, up: up, left: st.left, via: st.via})
					} else {
						out = append(out, lead{})
					}
					return
				}
				p = rest
			case *grammar.Choice:
				for _, alt := range e.Alts {
					visit(rest.then(alt), st)
				}
				return
			case *grammar.Optional:
				visit(rest.then(e.Expr), st)
				visit(rest, st)
				return
			case *grammar.Repeat:
				if e.Max == 0 {
					p = rest
					continue
				}
				again := k.againOf(e)
				visit(rest.then(e.Expr, again), st)
				if e.Min == 0 {
					visit(rest, st)
				}
				return
			case *grammar.Ref:
				if k.byName[e.Name] == nil || e.Level == rhsLevel {
					out = append(out, lead{})
					return
				}
				if k.shareable(e) && (only == nil || *only == e.Name) {
					out = append(out, lead{call: e, rest: rest, up: up, left: st.left, via: st.via})
					return
				}
				if k.shareable(e) && seen != nil {
					seen[e.Name] = true
				}
				st.via = &refList{e, st.via}
				if pr := k.operatorsBody(e.Name); pr != nil {
					k.operatorsLeads(pr, e.Name, func(c *cont) { visit(c, st) })
					return
				}
				p = rest.then(k.byName[e.Name].Expr)
			default:
				out = append(out, lead{})
				return
			}
		}
	}
	for i, p := range paths {
		var st state
		if from != nil {
			st = state{from[i].up, from[i].left, from[i].via}
		}
		visit(p, st)
	}
	return out
}

// endLeads continues a path at the end of the body of rule, after each call of the rule, with the
// call site; at the end of the start rule the parse ends, without a site.
func (k *classifier) endLeads(rule string, visit func(*cont, *site)) {
	if rule == k.start {
		visit(nil, nil)
	}
	for _, s := range k.sites {
		if s.callee == rule {
			visit(s.cont, s)
		}
	}
}

// endsEmpty reports whether the path c can reach its end without consuming input.
func (k *classifier) endsEmpty(c *cont) bool {
	for ; c != nil; c = c.next {
		k.control.Poll()
		if _, null := k.firstItems(c.items); !null {
			return false
		}
	}
	return true
}

// A retriedPart is an operator part after an operand, which each enclosing level of a nesting
// Operators Rule tries again at the same position.
type retriedPart struct {
	u    *unit
	rule string
	end  *cont // the end of the Operators Rule, after which the operator parts follow
}

// addRetried gives each operator part after an operand of a nesting Operators Rule the operator
// parts that the enclosing levels try again as a continuation. It runs after every rule is
// walked, since nestsOps reads every call site.
func (k *classifier) addRetried() {
	for _, r := range k.retried {
		if k.nestsOps(r.rule) {
			// Before what follows the Operators Rule, the last continuation.
			n := len(r.u.conts) - 1
			r.u.conts = slices.Insert(r.u.conts, n, contSpec{paths: []*cont{r.end},
				what: "the operator parts that the enclosing operator parts of " + r.rule + " try again"})
		}
	}
}

// nestsOps reports whether the evaluations of the Operators Rule of rule nest without a bound that
// the grammar sets and end at one position, so that each enclosing level tries the operator parts
// after an operand again where the inner ones end: a prefix operator part, or a right-associative
// infix one, parses operators of its own level in its right operand, and an operand or an
// operator part that ends with a call that nests at one end (nestsAt), directly or through other
// rules, parses the rule again at its end.
func (k *classifier) nestsOps(rule string) bool {
	pr := k.operators(rule)
	if pr == nil {
		return false
	}
	for _, l := range pr.Levels {
		for _, op := range l.Operators {
			if op.Kind == grammar.Prefix || op.Kind == grammar.Infix && op.Assoc == grammar.AssocRight {
				return true
			}
		}
	}
	for _, s := range k.sites {
		if s.caller == rule && k.nestsAt(s) {
			return true
		}
	}
	return false
}

// opsLeads continues a path after an operand of the Operators Rule of rule, with its operator
// parts. inner tells that the path looked into the rule through a call (cont.inner).
func (k *classifier) opsLeads(rule string, inner bool, visit func(*cont)) {
	pr := k.operators(rule)
	if pr == nil {
		return
	}
	end := &cont{end: endOps, rule: rule, inner: inner}
	for _, l := range pr.Levels {
		for _, op := range l.Operators {
			if op.Kind == grammar.Prefix {
				continue
			}
			items := []grammar.Expr{k.skipItem(pr), op.Expr}
			if op.Kind == grammar.Infix {
				items = append(items, &grammar.Ref{Name: rule, Level: rhsLevel})
			}
			visit(&cont{items: items, next: end})
		}
	}
}

// operatorsBody returns the Operators Rule that is the body of rule, alone or under attributes
// that change neither what it evaluates first nor where the parser continues after it fails, or
// nil.
func (k *classifier) operatorsBody(rule string) *grammar.OperatorsRule {
	body := ungroup(k.byName[rule].Expr)
	if a, ok := body.(*grammar.Attributed); ok {
		if recoverSkip(a) != nil || hasPragma(a, "missing") {
			return nil
		}
		body = ungroup(a.Expr)
	}
	pr, _ := body.(*grammar.OperatorsRule)
	return pr
}

// skipItem is the skip of pr as an item of a path. The rule evaluates its skip before each operand
// and operator part, and continues at the same position when the skip does not match, so a skip
// that can match nothing stands for itself, and the first call that it evaluates is a call that the
// path evaluates first; any other skip is optional.
func (k *classifier) skipItem(pr *grammar.OperatorsRule) grammar.Expr {
	if pr.Skip == nil || k.nullable(pr.Skip) {
		return pr.Skip
	}
	return &grammar.Optional{Expr: pr.Skip}
}

// operatorsLeads continues a path into the body of rule, the Operators Rule pr. The rule begins
// with its skip, then an operand or a prefix operator part, so a skip that is a shareable call is
// what every path through it evaluates first. After its operand the rule tries its operator
// parts, then continues as after its calls: the path continues at the end of the rule body, which
// includes the path that called it.
func (k *classifier) operatorsLeads(pr *grammar.OperatorsRule, rule string, visit func(*cont)) {
	end := &cont{end: endOps, rule: rule, inner: true}
	start := func(e grammar.Expr, more ...grammar.Expr) {
		items := []grammar.Expr{}
		if pr.Skip != nil {
			items = append(items, k.skipItem(pr))
		}
		visit(&cont{items: append(append(items, e), more...), next: end})
	}
	for _, o := range pr.Operands {
		start(o.Expr)
	}
	for _, l := range pr.Levels {
		for _, op := range l.Operators {
			if op.Kind == grammar.Prefix {
				start(op.Expr, &grammar.Ref{Name: rule, Level: rhsLevel})
			}
		}
	}
}

// strip removes the calls that a unit and a continuation share from their starts, one after
// another, and returns what remains of both. Paths that examine nothing more take no part. It
// records the shared calls of owner, the unit, for the restart clause of nestsAt
// (recordShared).
//
// A shared call can be the first thing that a call of another shareable rule evaluates, as ws is
// in primary = ws number: when the first calls of the two differ, the classifier looks through
// them, by evaluating their bodies, for a call that both evaluate first.
func (k *classifier) strip(u, c []*cont, owner *unit) ([]*cont, []*cont) {
	var fu, fc []lead // the leads of which the paths of u and c are the rests
	for range 16 {
		lu, lc := k.leads(u, fu, nil, nil), k.leads(c, fc, nil, nil)
		if !commonLead(lu, lc) {
			lu, lc = k.deepLeads(u, c, fu, fc)
			if lu == nil {
				return u, c
			}
		}
		if k.rescans(lc) {
			return u, c
		}
		k.recordShared(owner, lu, lc)
		u, fu = rests(lu)
		c, fc = rests(lc)
	}
	return u, c
}

// rescans reports whether the continuation evaluates its shared call again with a cost that
// nothing bounds: the memo does not answer the call, the rule scans input, and the continuation
// reaches the call after the end of a rule body, in what follows a call whose evaluations can nest
// without a bound and end at one position. Each of the nested evaluations that ends at the
// position of the unit then evaluates the call there again, as the option in
// e = "(" e? (ws ")")? does, where ws scans the same spaces once for each level.
func (k *classifier) rescans(lc []lead) bool {
	for _, l := range lc {
		if l.call != nil && l.up && !k.memoAt(l.call) && k.ruleSrc[l.call.Name] != nil {
			return true
		}
	}
	return false
}

// deepLeads looks for a shareable call that the unit and the continuation both evaluate first,
// through the bodies of the shareable calls that they evaluate first, and returns the leads of
// both at it, or nil.
func (k *classifier) deepLeads(u, c []*cont, fu, fc []lead) ([]lead, []lead) {
	none := ""
	su, sc := map[string]bool{}, map[string]bool{}
	k.leads(u, fu, &none, su)
	k.leads(c, fc, &none, sc)
	var both []string
	for n := range su {
		if sc[n] {
			both = append(both, n)
		}
	}
	slices.Sort(both)
	for _, n := range both {
		lu, lc := k.leads(u, fu, &n, nil), k.leads(c, fc, &n, nil)
		if commonLead(lu, lc) {
			return lu, lc
		}
	}
	return nil, nil
}

// commonLead reports whether every lead of a and b that examines input is the same shared call,
// or the same literal, and both have one.
func commonLead(a, b []lead) bool {
	var first *lead
	seen := [2]bool{}
	for i, ls := range [2][]lead{a, b} {
		for j := range ls {
			l := &ls[j]
			if l.done {
				continue
			}
			if l.call == nil && l.lit == nil || first != nil && !sameLead(first, l) {
				return false
			}
			first = l
			seen[i] = true
		}
	}
	return seen[0] && seen[1]
}

func sameLead(a, b *lead) bool {
	if a.call != nil || b.call != nil {
		return a.call != nil && b.call != nil && a.call.Name == b.call.Name && a.call.Level == b.call.Level
	}
	return a.lit.Value == b.lit.Value && a.lit.IgnoreCase == b.lit.IgnoreCase
}

// rests returns the paths after the leads that examine input, and those leads.
func rests(ls []lead) ([]*cont, []lead) {
	var out []*cont
	var from []lead
	for _, l := range ls {
		if !l.done {
			out = append(out, l.rest)
			from = append(from, l)
		}
	}
	return out, from
}

// committing reports whether, after the shared calls are removed, the first characters of the
// unit paths and of each continuation are disjoint, or the words with which the unit paths begin
// exclude the continuation. It returns the first continuation that is neither.
func (k *classifier) committing(owner *unit, self []*cont, conts []contSpec) (bool, string) {
	for _, cs := range conts {
		u, c := k.strip(self, cs.paths, owner)
		// A unit whose sources all lie in the shared calls repeats nothing beyond their cost.
		if !k.pathsContainSource(u) {
			continue
		}
		fu := k.firstPaths(u, nil)
		fc := k.firstPaths(c, func(r string) charset { return k.follow[r] })
		if fu.intersects(fc) && !k.excludedByWord(u, c) {
			return false, cs.what
		}
	}
	return true, ""
}

// checkUnits checks condition 2: every unit that contains a scanning source is committing.
func (k *classifier) checkUnits() {
	for _, u := range k.units {
		if !u.hasSource {
			continue
		}
		if u.unsupported != "" {
			k.reason(u.rule, u.pos, "%s at %s contains %s, and %s, which the classifier does not analyze",
				u.kind, at(u.rule, u.pos), u.src, u.unsupported)
			continue
		}
		if u.growing {
			k.checkGrowingUnit(u)
			continue
		}
		if !k.unitFailsAfterSource(u) {
			continue
		}
		conts := k.withoutReentry(u)
		if ok, what := k.committing(u, u.self, conts); !ok {
			if decidedUnitsOn && k.unitDecided(u, conts) {
				continue
			}
			u.nc, u.witness = true, what
		}
	}
}

// --- Growth ---

// leftPaths follows the alternative alt of a rule of the cycle through the calls of rules of the
// cycle to the left calls of leaders, and returns what follows those calls. first are the paths on
// which the alternative evaluates something other than a left call first, from that point, so
// that the first step of the growth can continue the unit. With grower set, the alternative is
// one of the leader grower: in the first step of its growth its own left calls read the failed
// seed and end their paths, while a left call of another leader can match and is a path of first.
func (k *classifier) leftPaths(start *cont, cycle int, grower string) (paths, first []*cont, ok bool) {
	ok = true
	budget := leadBudget
	var visit func(p *cont, depth int)
	visit = func(p *cont, depth int) {
		for {
			k.control.Poll()
			budget--
			if budget < 0 || p == nil || len(p.items) == 0 {
				ok = false
				return
			}
			rest := p.rest()
			switch e := p.items[0].(type) {
			case *grammar.Group:
				p = rest.then(e.Expr)
			case *grammar.Capture:
				p = rest.then(e.Expr)
			case *grammar.Discard:
				p = rest.then(e.Expr)
			case *grammar.Atomic:
				p = rest.then(e.Expr)
			case *grammar.Stream:
				p = rest.then(e.Expr)
			case *grammar.Seq:
				if len(e.Items) == 0 {
					first = append(first, rest)
					return
				}
				p = rest.then(e.Items...)
			case *grammar.Choice:
				for _, alt := range e.Alts {
					visit(rest.then(alt), depth)
				}
				return
			case *grammar.Ref:
				if !k.an.cyclic[e.Name] || k.an.cycle[e.Name] != cycle || e.Level != "" {
					first = append(first, p)
					return
				}
				if k.an.leaders[e.Name] {
					paths = append(paths, rest)
					if grower != "" && e.Name != grower {
						first = append(first, p)
					}
					return
				}
				if depth > len(k.rules) {
					ok = false
					return
				}
				p = rest.then(k.byName[e.Name].Expr)
				depth++
			default:
				first = append(first, p)
				return
			}
		}
	}
	visit(start, 0)
	return paths, first, ok
}

// checkGrowingUnit checks a growing unit. In each step of a growth after the first its left call
// reads the seed, and it examines the input from the end of the seed; its continuations are the
// later growing alternatives and what follows the calls that start the growth.
func (k *classifier) checkGrowingUnit(u *unit) {
	grower := ""
	if k.an.leaders[u.rule] {
		grower = u.rule
	}
	self, first, ok := k.leftPaths(u.self[0], u.cycle, grower)
	if !ok {
		k.reason(u.rule, u.pos, "the growing alternative at %s contains %s, and the classifier cannot follow its left calls",
			at(u.rule, u.pos), u.src)
		return
	}
	var conts []contSpec
	for _, later := range u.choice.Alts[u.index+1:] {
		if f := k.firstEvaluated(later); f == nil || !k.sameCycle(u.rule, f.Name) || f.Level != "" {
			continue
		}
		paths, _, ok := k.leftPaths(u.after.then(later), u.cycle, "")
		if !ok {
			k.reason(u.rule, u.pos, "the growing alternative at %s contains %s, and the classifier cannot follow its left calls",
				at(u.rule, u.pos), u.src)
			return
		}
		// A later growing alternative that succeeds ends the step with a longer result, and the
		// next step reads it as the seed: past the end of the rule body it continues after the
		// seed reads too, so it reads the FOLLOW sets with them.
		conts = append(conts, contSpec{paths: paths,
			what: fmt.Sprintf("the later growing alternative %s at %s", k.show(later), at(u.rule, exprPos(later)))})
	}
	starts, ok := k.growthStarts(u.cycle)
	if !ok {
		k.reason(u.rule, u.pos, "the growing alternative at %s contains %s, and its cycle can grow from a call of a rule that is not a leader",
			at(u.rule, u.pos), u.src)
		return
	}
	conts = append(conts, starts...)
	if ok, what := k.committing(u, self, conts); !ok {
		u.nc, u.witness = true, what
		return
	}
	if u.afterInput {
		if ok, what := k.committing(u, u.self, u.conts); !ok {
			u.nc, u.witness = true, what
			return
		}
	}
	// The unit of a leader is evaluated only in steps of its growth; in the first step the paths
	// through its own left calls fail at them. The unit of another rule of the cycle is evaluated
	// in the steps of the growths that reach it, and, when the rule is called where its cycle does
	// not grow, as an alternative that is not growing, after the leaders that it calls have grown.
	switch {
	case grower != "" && len(first) > 0:
		if ok, what := k.committing(u, first, u.conts); !ok {
			u.nc, u.witness = true, what
		}
	case grower == "" && (len(first) > 0 || k.startsGrowth(u.rule)):
		if ok, what := k.committing(u, u.self, u.conts); !ok {
			u.nc, u.witness = true, what
		}
	}
}

// startsGrowth reports whether the rule, a rule of a left-recursion cycle other than a leader, can
// be evaluated where its cycle does not grow: when a call that is not a left call evaluates it, or
// one that can run after its caller's body consumed input, or it is the start rule, or a left call
// evaluates it in the body of such a rule. Its left calls of
// leaders then start the growths, and the parser continues after them when the growths end.
func (k *classifier) startsGrowth(rule string) bool {
	if k.starters == nil {
		k.starters = map[string]bool{}
		var add func(r string)
		add = func(r string) {
			if k.starters[r] || !k.an.cyclic[r] || k.an.leaders[r] {
				return
			}
			k.starters[r] = true
			for _, s := range k.sites {
				if s.caller == r && s.mayStart && k.sameCycle(r, s.callee) {
					add(s.callee)
				}
			}
		}
		add(k.start)
		for _, s := range k.sites {
			// A call after something that can match nothing is a left call only when that matches
			// nothing; after input it is outside the growths of its cycle at its position.
			if k.outside[s.ref] || !s.mustStart {
				add(s.callee)
			}
		}
	}
	return k.starters[rule]
}

// growthStarts returns what follows the calls that start a growth of the cycle: the calls of its
// rules other than left calls, and the start of the parse when the start rule is on the cycle. It
// fails when such a call is of a rule that is not a leader.
func (k *classifier) growthStarts(cycle int) ([]contSpec, bool) {
	var out []contSpec
	if k.an.cyclic[k.start] && k.an.cycle[k.start] == cycle && !k.an.leaders[k.start] {
		paths, ok := k.memberStarts(k.start, &cont{end: endRule, rule: k.start}, cycle)
		if !ok {
			return nil, false
		}
		out = append(out, contSpec{paths: paths, what: "what follows the start of the parse"})
	}
	for _, s := range k.sites {
		if !k.an.cyclic[s.callee] || k.an.cycle[s.callee] != cycle {
			continue
		}
		if k.sameCycle(s.caller, s.callee) && s.mustStart {
			continue
		}
		paths := []*cont{s.cont}
		if !k.an.leaders[s.callee] {
			var ok bool
			if paths, ok = k.memberStarts(s.callee, s.cont, cycle); !ok {
				return nil, false
			}
		}
		out = append(out, contSpec{paths: paths,
			what: fmt.Sprintf("what follows the call %s at %s", s.callee, at(s.caller, exprPos(s.ref)))})
	}
	return out, true
}

// memberStarts follows the body of the member rule, called where its cycle does not grow, through
// the left calls of members of the cycle to the left calls of leaders, which start the growths,
// and returns what follows those calls, continued by after. It fails when a left call of the cycle
// is in an expression that it does not follow.
func (k *classifier) memberStarts(rule string, after *cont, cycle int) (paths []*cont, ok bool) {
	ok = true
	budget := leadBudget
	inCycle := func(e grammar.Expr) bool {
		for _, n := range k.an.leftCalls(e, nil) {
			if k.an.cyclic[n] && k.an.cycle[n] == cycle {
				return true
			}
		}
		return false
	}
	var visit func(p *cont, depth int)
	visit = func(p *cont, depth int) {
		for ok {
			k.control.Poll()
			budget--
			if budget < 0 || depth > len(k.rules) {
				ok = false
				return
			}
			if len(p.items) == 0 {
				if p.next == nil || p.next == after {
					return
				}
				p = p.next
				continue
			}
			rest := p.rest()
			switch e := p.items[0].(type) {
			case *grammar.Group:
				p = rest.then(e.Expr)
			case *grammar.Capture:
				p = rest.then(e.Expr)
			case *grammar.Discard:
				p = rest.then(e.Expr)
			case *grammar.Atomic:
				p = rest.then(e.Expr)
			case *grammar.Stream:
				p = rest.then(e.Expr)
			case *grammar.InlineAction:
				p = rest.then(e.Expr)
			case *grammar.Seq:
				p = rest.then(e.Items...)
			case *grammar.Choice:
				for _, alt := range e.Alts {
					visit(rest.then(alt), depth)
				}
				return
			case *grammar.Optional:
				visit(rest.then(e.Expr), depth)
				p = rest
			case *grammar.Ref:
				if k.an.cyclic[e.Name] && k.an.cycle[e.Name] == cycle {
					if e.Level != "" {
						ok = false
						return
					}
					if k.an.leaders[e.Name] {
						paths = append(paths, rest)
						return
					}
					p = rest.then(k.byName[e.Name].Expr)
					depth++
					continue
				}
				if inCycle(e) {
					ok = false
					return
				}
				if !k.nullable(e) {
					return
				}
				p = rest
			default:
				if inCycle(e) {
					ok = false
					return
				}
				if !k.nullable(e) {
					return
				}
				p = rest
			}
		}
	}
	visit(&cont{items: []grammar.Expr{k.byName[rule].Expr}, next: after}, 0)
	return paths, ok
}

// checkGrowth checks condition 3: no scanning source is evaluated at the position where a
// left-recursion cycle grows, other than inside a call of a memoized rule that is not on the
// cycle, because each step of the growth evaluates the bodies of the cycle again there.
func (k *classifier) checkGrowth() {
	seen := map[*source]bool{}
	k.stepRules = map[string]bool{}
	for _, r := range k.rules {
		if !k.reach[r.Name] || !k.an.cyclic[r.Name] {
			continue
		}
		cycle := k.an.cycle[r.Name]
		var visit func(e grammar.Expr) bool // returns whether e can match without consuming input
		report := func(s *source) {
			if seen[s] {
				return
			}
			seen[s] = true
			k.cause(ClassPoly, 2, s.rule, s.pos, "%s: %s is evaluated in each step of the growth of the left-recursive rule %s, at the position where it grows",
				degreeText(ClassPoly, 2), s, r.Name)
		}
		visit = func(e grammar.Expr) bool {
			k.control.Poll()
			switch e := e.(type) {
			case *grammar.Ref:
				if k.byName[e.Name] == nil {
					return true
				}
				if k.an.cyclic[e.Name] && k.an.cycle[e.Name] == cycle {
					return k.nullable(e)
				}
				if !k.memoAt(e) {
					if s := k.ruleSrc[e.Name]; s != nil {
						report(s)
					}
				}
				return k.nullable(e)
			case *grammar.Seq:
				for _, it := range e.Items {
					if !visit(it) {
						return false
					}
				}
				return true
			case *grammar.Choice:
				null := false
				for _, alt := range e.Alts {
					null = visit(alt) || null
				}
				return null
			case *grammar.Repeat:
				if e.Max == 0 {
					return true
				}
				if e.Max < 0 && !k.runSite(e) {
					report(k.sourceAt(r.Name, e, "the repetition "+k.show(e)))
					for _, c := range allCallsControlled(e.Expr, nil, k.control) {
						maps.Copy(k.stepRules, k.reachable(c))
					}
				}
				visit(e.Expr)
				return k.nullable(e)
			case *grammar.Optional:
				visit(e.Expr)
				return true
			case *grammar.And:
				visit(e.Expr)
				return true
			case *grammar.Not:
				visit(e.Expr)
				return true
			case *grammar.Group:
				return visit(e.Expr)
			case *grammar.Capture:
				return visit(e.Expr)
			case *grammar.Discard:
				return visit(e.Expr)
			case *grammar.Atomic:
				return visit(e.Expr)
			case *grammar.Stream:
				return visit(e.Expr)
			case *grammar.InlineAction:
				return visit(e.Expr)
			case *grammar.Recovery:
				if !reused(e.Skip) {
					report(k.sourceAt(r.Name, e, "the skip expression "+k.show(e.Skip)+" of #recover"))
				}
				visit(e.Expr)
				return k.nullable(e)
			case *grammar.Attributed:
				if skip := recoverSkip(e); skip != nil && !reused(skip) {
					report(k.sourceAt(r.Name, e, "the skip expression "+k.show(skip)+" of #recover"))
				}
				visit(e.Expr)
				return k.nullable(e)
			case *grammar.OperatorsRule:
				report(k.sourceAt(r.Name, e, "the operators that the Operators Rule applies"))
				return k.nullable(e)
			case *grammar.Predicate:
				if s := k.predicateSource[e]; s != nil {
					report(s)
				}
				return true
			}
			return k.nullable(e)
		}
		visit(r.Expr)
	}
}

// sourceAt returns the source recorded at e, so that one cause is reported once.
func (k *classifier) sourceAt(rule string, e grammar.Expr, what string) *source {
	key := sourceKey{rule, exprPos(e), what}
	if s := k.sourceIndex[key]; s != nil {
		return s
	}
	s := &source{rule: rule, pos: exprPos(e), what: what}
	k.sourceIndex[key] = s
	return s
}

type sourceKey struct {
	rule string
	pos  grammar.Pos
	what string
}

// --- Nesting ---

// checkNesting turns the units that are not committing into causes. A unit restarts the scans
// inside it once for each evaluation of the construct that holds it, so it costs more than the
// input only when that construct is evaluated repeatedly: inside a repetition, an Operators Rule
// or a recursion. Units that are not committing and nested in one another multiply: the degree is
// one more than the length of the longest chain of them. A chain that passes through itself, by
// a recursion of rule calls, has no bound that the classifier knows.
func (k *classifier) checkNesting() {
	var nc []*unit
	for _, u := range k.units {
		if u.nc {
			nc = append(nc, u)
		}
	}
	if len(nc) == 0 {
		return
	}
	// ncIn[r] are the units that are not committing in the rules that r reaches.
	ncByRule := map[string][]*unit{}
	for _, u := range nc {
		ncByRule[u.rule] = append(ncByRule[u.rule], u)
	}
	inner := map[*unit][]*unit{}
	for _, u := range nc {
		seen := map[*unit]bool{}
		add := func(v *unit) {
			if v.nc && !seen[v] {
				seen[v] = true
				inner[u] = append(inner[u], v)
			}
		}
		for _, v := range u.inner {
			add(v)
		}
		for _, c := range slices.Sorted(maps.Keys(u.calls)) {
			for _, r := range slices.Sorted(maps.Keys(k.reachable(c))) {
				for _, v := range ncByRule[r] {
					add(v)
				}
			}
		}
	}
	// Units that can reach a cycle of nesting have no bound.
	state := map[*unit]int{} // 0 unvisited, 1 on the stack, 2 done
	unbounded := map[*unit]bool{}
	var dfs func(u *unit) bool
	dfs = func(u *unit) bool {
		k.control.Poll()
		switch state[u] {
		case 1:
			return true
		case 2:
			return unbounded[u]
		}
		state[u] = 1
		bad := false
		for _, v := range inner[u] {
			if dfs(v) {
				bad = true
			}
		}
		state[u] = 2
		unbounded[u] = bad
		return bad
	}
	for _, u := range nc {
		dfs(u)
	}
	// depth[v] is the length of the longest chain of repeated units that ends at v.
	depth := map[*unit]int{}
	from := map[*unit]*unit{}
	var order []*unit
	done := map[*unit]bool{}
	var topo func(u *unit)
	topo = func(u *unit) {
		if done[u] || unbounded[u] {
			return
		}
		done[u] = true
		for _, v := range inner[u] {
			topo(v)
		}
		order = append(order, u)
	}
	for _, u := range nc {
		topo(u)
	}
	for _, u := range order {
		if u.repeated != "" || k.repeatedRule(u.rule) != "" {
			depth[u] = max(depth[u], 1)
		}
		// A repetition that each step of a growth evaluates repeats what it calls once more.
		if k.stepRules[u.rule] {
			depth[u] = max(depth[u], 2)
		}
	}
	for i := len(order) - 1; i >= 0; i-- {
		u := order[i]
		if depth[u] == 0 {
			continue
		}
		for _, v := range inner[u] {
			if !unbounded[v] && depth[u]+1 > depth[v] {
				depth[v], from[v] = depth[u]+1, u
			}
		}
	}
	for _, u := range nc {
		switch {
		case unbounded[u]:
			k.reason(u.rule, u.pos, "%s at %s is not committing against %s, and units that are not committing are nested in it without bound through a recursion of rule calls",
				u.kind, at(u.rule, u.pos), u.witness)
		case depth[u] == 0:
			k.reason(u.rule, u.pos, "%s at %s contains %s and is not committing against %s",
				u.kind, at(u.rule, u.pos), u.src, u.witness)
		case k.leaf(u, inner, unbounded):
			degree := depth[u] + 1
			root := u
			var chain []string
			for v := from[u]; v != nil; v = from[v] {
				chain = append(chain, fmt.Sprintf("%s at %s", v.kind, at(v.rule, v.pos)))
				root = v
			}
			why := root.repeated
			if why == "" {
				why = k.repeatedRule(root.rule)
			}
			msg := fmt.Sprintf("%s: after %s at %s fails, %s can start again inside the input that %s examined",
				degreeText(ClassPoly, degree), u.kind, at(u.rule, u.pos), u.witness, u.src)
			for _, c := range chain {
				msg += "; it is evaluated again inside " + c
			}
			msg += "; this repeats in " + why
			k.cause(ClassPoly, degree, u.src.rule, u.src.pos, "%s", msg)
		}
	}
}

// leaf reports whether u has no bounded unit that is not committing inside it.
func (k *classifier) leaf(u *unit, inner map[*unit][]*unit, unbounded map[*unit]bool) bool {
	for _, v := range inner[u] {
		if !unbounded[v] {
			return false
		}
	}
	return true
}

// repeatedRule says why the rule can be evaluated many times in a parse: it is on a recursion of
// rule calls, or it is called repeatedly from a rule. It is "" for a rule that is evaluated a
// bounded number of times.
func (k *classifier) repeatedRule(rule string) string {
	if k.repeatedRules == nil {
		k.repeatedRules = map[string]string{}
		var queue []string
		for _, r := range k.rules {
			if k.reach[r.Name] && k.recursive[r.Name] {
				k.repeatedRules[r.Name] = "the recursion of rule " + r.Name
				queue = append(queue, r.Name)
			}
		}
		for _, s := range k.sites {
			if s.repeated != "" && k.repeatedRules[s.callee] == "" {
				k.repeatedRules[s.callee] = s.repeated
				queue = append(queue, s.callee)
			}
		}
		for len(queue) > 0 {
			r := queue[0]
			queue = queue[1:]
			for _, s := range k.sites {
				if s.caller == r && k.repeatedRules[s.callee] == "" {
					k.repeatedRules[s.callee] = k.repeatedRules[r]
					queue = append(queue, s.callee)
				}
			}
		}
	}
	return k.repeatedRules[rule]
}

// pathsContainSource reports whether what remains of a unit after its shared calls contains a
// scanning source.
func (k *classifier) pathsContainSource(paths []*cont) bool {
	for _, p := range paths {
		for c := p; c != nil; c = c.next {
			for _, it := range c.items {
				if it == nil {
					continue
				}
				if r, ok := it.(*grammar.Ref); ok && r.Level == rhsLevel {
					return true
				}
				if k.containsSource(it) {
					return true
				}
			}
			if c.next == nil && c.end != endNone {
				return true
			}
		}
	}
	return false
}
