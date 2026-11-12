package compile

import (
	"maps"
	"slices"
	"strings"
	"unicode/utf8"
	"zeroparse.dev/zp/internal/cancel"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/grammaranalysis"
)

// The normal view of a grammar.
//
// The normal view rewrites the rule bodies of a grammar by the normalization rules of
// spec/proofs/grammar-laws.md, so that analyses compare shapes instead of spellings: unrolled
// whitespace becomes one repetition, runs of single-character alternatives become one class, and
// alternatives that share a first item share it once. Every rule preserves the relation of its law
// (≡E at least: the same match or failure, end, environment, escape of cuts and captures read by
// predicates), so an analysis of matching may read the view in place of the grammar. A parser never
// runs it.
//
// The complexity classifier reads the view too, so every rule applied for it also keeps the cost
// of the grammar within a constant factor (costView). A rule that drops an evaluation drops only
// one that the grammar either never makes or makes again in a state where it made it before, and
// only when that evaluation calls no rule that the memo leaves unanswered (evaluatedAgainCheaply):
// it then costs at most what its first evaluation cost, which the view keeps, and it cannot
// compound through recursion. Rules that only an analysis of matching may use, because they change
// what the runtime mechanisms of cost see (labels, inlining), are off for the classifier.
//
// The view shares every node that no rule changes with the grammar, and keeps the source
// positions of the nodes it rewrites, so the messages of an analysis point into the grammar as
// written.

// viewRule names a normalization rule of spec/proofs/grammar-laws.md.
type viewRule string

const (
	ruleN1  viewRule = "N1"  // splice groups and sequences, flatten choices (S1, C2)
	ruleN2  viewRule = "N2"  // drop the alternatives after one that always succeeds (C3)
	ruleN3  viewRule = "N3"  // drop duplicate and subsumed alternatives (C1, C5)
	ruleN4  viewRule = "N4"  // drop bottom alternatives, cut sequences after a leading bottom (C4, S2)
	ruleN5  viewRule = "N5"  // canonicalize stacked lookaheads (K1, K2)
	ruleN6  viewRule = "N6"  // drop harmless cuts and recoveries that never fire (T2, V1)
	ruleN7  viewRule = "N7"  // drop labels: @error and @diagnostic
	ruleN8  viewRule = "N8"  // merge runs of single-character alternatives (C9)
	ruleN9  viewRule = "N9"  // merge repetitions in a row (R3, R4, R5)
	ruleN10 viewRule = "N10" // flatten nested repetitions (R6)
	ruleN11 viewRule = "N11" // unroll X* (Y X*)* and absorb X* (X / Y)* (R7)
	ruleN12 viewRule = "N12" // replace !X X by bottom, left-factor alternatives (K4, C6)
	ruleN13 viewRule = "N13" // inline calls (P1)
)

// viewOptions selects the rules of a view.
type viewOptions struct {
	// labels applies N7. It changes the expectations and the run sites of the grammar.
	labels bool
	// inline applies N13. It changes which evaluations the memo answers.
	inline bool
	// laws, if not nil, holds the only rules that apply, for comparing views that apply some
	// rules only. N7 and N13 apply only if labels and inline select them, too.
	laws map[viewRule]bool
}

// costView is the view that the complexity classifier reads: every rule but N7 and N13.
var costView = viewOptions{}

// normalView computes the view of a set of rules.
type normalView struct {
	opts     viewOptions
	an       *Analysis
	memo     map[string]bool // the rules that every parse memoizes, in the grammar
	byName   map[string]*grammar.RuleDef
	inlined  map[string]grammar.Expr // the views of the bodies of inlined rules
	inlining map[string]bool
	// cheap are the rules whose calls evaluated again cost at most a constant factor of their first
	// evaluation (cheapRules).
	cheap   map[string]bool
	control *cancel.Control
	// applied counts the rewrites by rule.
	applied map[viewRule]int
	// orig are the expressions of the rules, and text gives, for an expression that the view builds,
	// the source text of the expressions that it stands for.
	orig map[grammar.Expr]bool
	text map[grammar.Expr]string
	// reads are, for each rule, the names of its captures that a predicate of the rule reads, and
	// valued the rules whose value reaches such a capture (valueReaders). Inside such a capture and
	// in the body of such a rule, depth is positive and the rules that change values (N2, N3, N9,
	// N10, N11, N12, N13, and the parts of N1 and N6 that turn a sequence or a choice of one item
	// into the item) are off.
	reads  map[string]map[string]bool
	valued map[string]bool
	depth  int
	cur    string // the rule whose body is being rewritten
}

// view is the normal view of a set of rules.
type view struct {
	rules []*grammar.RuleDef
	// applied counts the rewrites by rule.
	applied map[viewRule]int
	// text gives, for each expression that the view builds, the text of the expressions of the
	// grammar that it stands for, so that messages quote the grammar as it is written.
	text map[grammar.Expr]string
}

// normalizeRules returns the view of rules. The rules that no rewrite changes are returned as they
// are. memo is the memoization of the grammar
// (memoizedRules) and an its analysis, which the view does not change.
func normalizeRules(rules []*grammar.RuleDef, an *Analysis, memo map[string]bool, opts viewOptions, control *cancel.Control) *view {
	v := &normalView{opts: opts, an: an, memo: memo, orig: map[grammar.Expr]bool{}, text: map[grammar.Expr]string{},
		byName: map[string]*grammar.RuleDef{}, inlined: map[string]grammar.Expr{}, inlining: map[string]bool{},
		control: control, applied: map[viewRule]int{}}
	for _, r := range rules {
		v.byName[r.Name] = r
		WalkExpr(r.Expr, func(x grammar.Expr) { v.orig[x] = true })
	}
	v.cheapRules(rules)
	v.valueReaders(rules)
	out := make([]*grammar.RuleDef, len(rules))
	for i, r := range rules {
		v.cur, v.depth = r.Name, 0
		if v.valued[r.Name] {
			v.depth = 1
		}
		body := v.fix(r.Expr)
		if body == r.Expr {
			out[i] = r
			continue
		}
		copy := *r
		copy.Expr = body
		out[i] = &copy
	}
	return &view{rules: out, applied: v.applied, text: v.text}
}

// fix rewrites e until no rule applies. Every rule lowers the measure of spec/proofs/grammar-laws.md
// (Termination), so the loop ends; the bound only guards against a defect of a rule.
func (v *normalView) fix(e grammar.Expr) grammar.Expr {
	for range 64 {
		next := v.expr(e)
		if next == e {
			return e
		}
		e = next
	}
	return e
}

func (v *normalView) count(r viewRule) { v.applied[r]++ }

// on reports whether the rule r applies.
func (v *normalView) on(r viewRule) bool { return v.opts.laws == nil || v.opts.laws[r] }

// expr rewrites the parts of e and then e. It returns e itself when nothing changes. An expression
// that it builds in place of e stands for the text of e.
func (v *normalView) expr(e grammar.Expr) grammar.Expr {
	r := v.rewrite(e)
	if r != e {
		v.standsFor(r, v.textOf(e))
	}
	return r
}

// standsFor records that the expression x, if the view built it, stands for the text t.
func (v *normalView) standsFor(x grammar.Expr, t string) {
	if !v.orig[x] {
		if _, ok := v.text[x]; !ok {
			v.text[x] = t
		}
	}
}

// textOf returns the text of the grammar that e stands for.
func (v *normalView) textOf(e grammar.Expr) string {
	if t, ok := v.text[e]; ok {
		return t
	}
	return grammar.FormatExpr(e)
}

// textOfAll returns the texts of es joined by sep.
func (v *normalView) textOfAll(es []grammar.Expr, sep string) string {
	ts := make([]string, len(es))
	for i, e := range es {
		ts[i] = v.textOf(e)
	}
	return strings.Join(ts, sep)
}

func (v *normalView) rewrite(e grammar.Expr) grammar.Expr {
	v.control.Poll()
	switch x := e.(type) {
	case *grammar.Group:
		if !v.on(ruleN1) {
			inner := v.expr(x.Expr)
			if inner == x.Expr {
				return x
			}
			c := *x
			c.Expr = inner
			return &c
		}
		v.count(ruleN1)
		return v.expr(x.Expr)
	case *grammar.Seq:
		return v.seq(x)
	case *grammar.Choice:
		return v.choice(x)
	case *grammar.Repeat:
		inner := v.expr(x.Expr)
		if v.on(ruleN10) {
			if r := v.nestedRepeat(x, inner); r != nil {
				return r
			}
		}
		if inner == x.Expr {
			return x
		}
		c := *x
		c.Expr = inner
		return &c
	case *grammar.Optional:
		inner := v.expr(x.Expr)
		if inner == x.Expr {
			return x
		}
		c := *x
		c.Expr = inner
		return &c
	case *grammar.And:
		return v.lookahead(x, x.Expr, false)
	case *grammar.Not:
		return v.lookahead(x, x.Expr, true)
	case *grammar.Capture:
		read := v.reads[v.cur][x.Name]
		if read {
			v.depth++
		}
		inner := v.expr(x.Expr)
		if read {
			v.depth--
		}
		if inner == x.Expr {
			return x
		}
		c := *x
		c.Expr = inner
		return &c
	case *grammar.Discard:
		inner := v.expr(x.Expr)
		if inner == x.Expr {
			return x
		}
		c := *x
		c.Expr = inner
		return &c
	case *grammar.Atomic:
		inner := v.expr(x.Expr)
		if inner == x.Expr {
			return x
		}
		c := *x
		c.Expr = inner
		return &c
	case *grammar.Attributed:
		return v.attributed(x)
	case *grammar.Ref:
		if v.opts.inline && v.on(ruleN13) && v.depth == 0 {
			// Inside a value that a predicate reads the call stays: the body of a rule with several
			// items would add them to the sequence around the call instead of being one element.
			if body := v.inline(x); body != nil {
				v.count(ruleN13)
				return body
			}
		}
		return x
	}
	// Terminals, predicates, recoveries, streams, inline actions and Operators Rules are kept as
	// they are: the classifier reads their structure as the runtime runs it.
	return e
}

// seq applies N1 (splice), N4 (leading bottom), N9 (repetitions in a row), N11 (unrolling) and
// N12 (!X X) to a sequence.
func (v *normalView) seq(s *grammar.Seq) grammar.Expr {
	changed := false
	var items []grammar.Expr
	for _, it := range s.Items {
		n := v.expr(it)
		if n != it {
			changed = true
		}
		if inner, ok := n.(*grammar.Seq); ok && v.on(ruleN1) {
			// S1: a sequence in a sequence adds its elements to the surrounding one.
			v.count(ruleN1)
			changed = true
			items = append(items, inner.Items...)
			continue
		}
		items = append(items, n)
	}
	for i := 0; i+1 < len(items); i++ {
		if m := v.adjacent(items[i], items[i+1]); m != nil {
			v.standsFor(m, v.textOfAll(items[i:i+2], " "))
			items = slices.Replace(slices.Clip(items), i, i+2, m)
			changed = true
			i = max(i-2, -1)
		}
	}
	for i := 0; i+1 < len(items); i++ {
		if _, ok := items[i].(*grammar.Bottom); ok && v.on(ruleN4) {
			// S2: nothing after a bottom is evaluated.
			v.count(ruleN4)
			items, changed = items[:i+1], true
			break
		}
	}
	if len(items) == 1 && v.on(ruleN1) {
		_, bottom := items[0].(*grammar.Bottom)
		if bottom || v.depth == 0 && !v.observed(items[0]) {
			// A sequence of one item matches as the item does, and has the value of a tuple of
			// one element where the item does not: a predicate that reads it sees the difference
			// (spec/language/parser-expressions.md, Values), so the sequence stays where one can.
			if len(s.Items) != 1 || changed {
				v.count(ruleN1)
			}
			return items[0]
		}
	}
	if !changed {
		return s
	}
	c := *s
	c.Items, c.Breaks = items, nil
	return &c
}

// adjacent rewrites two items in a row into one, or returns nil.
func (v *normalView) adjacent(a, b grammar.Expr) grammar.Expr {
	ra, aRep := a.(*grammar.Repeat)
	rb, bRep := b.(*grammar.Repeat)
	if bRep && rb.Max < 0 && rb.Min <= 1 && v.depth == 0 && (v.on(ruleN9) || v.on(ruleN11)) && !v.observed(a, b) {
		x := rb.Expr
		switch {
		case !v.on(ruleN9):
		case aRep && ra.Max < 0 && ra.Min <= 1 && rb.Min == 0 && v.same(ra.Expr, x) && v.idempotent(x):
			// R3 and R4: X* X* and X+ X* match as the first repetition does. The second evaluates
			// X once more where the first stopped, in a state that differs only by bindings that X
			// does not read.
			v.count(ruleN9)
			return ra
		case !aRep && rb.Min == 0 && v.same(a, x) && v.idempotent(x):
			// R5: X X* is X+.
			v.count(ruleN9)
			c := *rb
			c.Min = 1
			return &c
		}
		if o, ok := a.(*grammar.Optional); ok && v.on(ruleN9) && rb.Min == 0 && v.same(o.Expr, x) && v.idempotent(x) && !v.an.isNullable(x) {
			// R4: X? X* matches as X* does.
			v.count(ruleN9)
			return rb
		}
		if aRep && ra.Max < 0 && ra.Min == 0 && rb.Min == 0 && v.on(ruleN11) {
			if r := v.unroll(ra, rb); r != nil {
				return r
			}
		}
	}
	if n, ok := a.(*grammar.Not); ok && v.on(ruleN12) && v.depth == 0 && v.same(n.Expr, b) && boundedTerminal(b) {
		// K4: !X X never matches. X examines a bounded amount of input, so the view drops a
		// constant cost. !X X has the type of X and _|_ has the type _|_, which a choice around it
		// leaves out of its join: (-"b" / !"a" "a") is one element, nil or a string, and
		// (-"b" / _|_) none. So the rewrite stays where a predicate can read the value. X makes
		// no capture, so only depth decides.
		v.count(ruleN12)
		return &grammar.Bottom{SourceInfo: n.SourceInfo}
	}
	return nil
}

// unroll applies R7 to X* (Y X*)*, giving (X / Y)*, and the absorption X* (X / Y)* to (X / Y)*.
func (v *normalView) unroll(ra, rb *grammar.Repeat) grammar.Expr {
	x := ra.Expr
	if v.an.isNullable(x) || v.cut(x) {
		return nil
	}
	if ch, ok := rb.Expr.(*grammar.Choice); ok && len(ch.Alts) > 1 && v.same(ch.Alts[0], x) && v.idempotent(x) {
		// The absorption: the second repetition evaluates X again where the first stopped.
		v.count(ruleN11)
		return rb
	}
	s, ok := rb.Expr.(*grammar.Seq)
	if !ok || len(s.Items) < 2 {
		return nil
	}
	last, ok := s.Items[len(s.Items)-1].(*grammar.Repeat)
	if !ok || last.Min != 0 || last.Max >= 0 || !v.same(last.Expr, x) {
		return nil
	}
	var y grammar.Expr
	if len(s.Items) == 2 {
		y = s.Items[0]
	} else {
		ys := *s
		ys.Items, ys.Breaks = slices.Clone(s.Items[:len(s.Items)-1]), nil
		y = &ys
	}
	if v.cut(y) || bindsOutsideCalls(y) || !v.idempotent(x) {
		return nil
	}
	// R7: both evaluate X first at each position and Y after X fails, in the same states. When Y
	// matches without consuming input, the unrolled form evaluates X again where it failed, which
	// fails again only if Y bound nothing that X reads, and which the view drops: so Y makes no
	// binding outside its calls, and X evaluated again costs at most what it cost first.
	v.count(ruleN11)
	c := *rb
	c.SourceInfo = ra.SourceInfo
	c.Expr = &grammar.Choice{SourceInfo: ra.SourceInfo, Alts: []grammar.Expr{x, y}}
	return &c
}

// nestedRepeat applies R6 to a repetition whose element is a repetition, or returns nil.
func (v *normalView) nestedRepeat(outer *grammar.Repeat, inner grammar.Expr) grammar.Expr {
	if outer.Max >= 0 || outer.Min > 1 || v.depth > 0 || v.observed(outer) {
		return nil
	}
	switch in := inner.(type) {
	case *grammar.Repeat:
		if in.Max >= 0 || in.Min > 1 || outer.Min == 1 && in.Min == 1 {
			return nil
		}
		if !v.idempotent(in.Expr) {
			return nil
		}
		// (X*)*, (X+)* and (X*)+ match as X* does: the outer repetition stops after an iteration
		// that matches nothing, which evaluates X once more where the inner one stopped.
		v.count(ruleN10)
		c := *in
		c.Min = 0
		return &c
	case *grammar.Optional:
		if outer.Min != 0 || v.an.isNullable(in.Expr) || !v.idempotent(in.Expr) {
			return nil
		}
		// (X?)* matches as X* does for an X that cannot match without consuming input.
		v.count(ruleN10)
		c := *outer
		c.Expr = in.Expr
		return &c
	}
	return nil
}

// choice applies N1 (flatten), N2, N3, N4, N6 (cuts), N8 and N12 (left factoring) to a choice.
func (v *normalView) choice(ch *grammar.Choice) grammar.Expr {
	applied := maps.Clone(v.applied)
	changed := false
	var alts []grammar.Expr
	for i, alt := range ch.Alts {
		n := v.expr(alt)
		if n != alt {
			changed = true
		}
		if inner, ok := n.(*grammar.Choice); ok && v.on(ruleN1) && (i == len(ch.Alts)-1 || !slices.ContainsFunc(inner.Alts, v.cut)) {
			// C2: a choice that is the last alternative splices always; an earlier one splices
			// when no cut in it would commit the outer choice instead of it.
			v.count(ruleN1)
			changed = true
			alts = append(alts, inner.Alts...)
			continue
		}
		alts = append(alts, n)
	}
	// C1, C3, C5 and T2 drop alternatives that never give the value of the choice, but they can
	// drop every alternative with a value and keep one without: the choice that gave nil gives no
	// element then, as (_ / "b") with "b" dropped. So they apply only where no predicate can read
	// the value. A bottom alternative adds nothing to the value of the choice, which joins the
	// types of its alternatives, so C4 applies everywhere.
	drops := v.depth == 0
	var kept []grammar.Expr
	for i, alt := range alts {
		if _, ok := alt.(*grammar.Bottom); ok && v.on(ruleN4) {
			// C4: a bottom alternative fails at once and records nothing.
			v.count(ruleN4)
			changed = true
			continue
		}
		if !drops {
			kept = append(kept, alt)
			continue
		}
		if v.on(ruleN3) && v.subsumed(alt, kept) {
			// C1 and C5: an earlier alternative matched wherever this one could, and when it
			// failed, this one fails in the same state.
			v.count(ruleN3)
			changed = true
			continue
		}
		kept = append(kept, alt)
		if v.on(ruleN2) && (v.an.AlwaysSucceeds(alt) || leadingCut(alt)) {
			// C3: no later alternative is tried after one that always succeeds; T2: nor after a
			// cut that the alternative passes before anything else.
			if i < len(alts)-1 {
				v.count(ruleN2)
				changed = true
			}
			break
		}
	}
	alts = kept
	if len(alts) > 0 && v.on(ruleN6) {
		// T2: a cut in the last alternative commits a choice that has nothing left to try.
		if last := v.dropCuts(alts[len(alts)-1], true); last != alts[len(alts)-1] {
			v.count(ruleN6)
			v.standsFor(last, v.textOf(alts[len(alts)-1]))
			alts, changed = append(alts[:len(alts)-1:len(alts)-1], last), true
		}
	}
	for i := range alts {
		if !v.on(ruleN6) {
			break
		}
		// T2: a cut that ends an alternative commits when nothing in it can fail any more.
		if a := v.dropCuts(alts[i], false); a != alts[i] {
			v.count(ruleN6)
			v.standsFor(a, v.textOf(alts[i]))
			alts, changed = slices.Clone(alts), true
			alts[i] = a
		}
	}
	if v.on(ruleN8) {
		if merged, ok := v.mergeChars(alts); ok {
			alts, changed = merged, true
		}
	}
	if v.on(ruleN12) {
		if factored, ok := v.factor(alts, ch); ok {
			alts, changed = factored, true
		}
	}
	switch {
	case len(alts) == 0:
		return &grammar.Bottom{SourceInfo: ch.SourceInfo}
	case len(alts) == 1 && v.cut(alts[0]):
		// A loose cut that dropCuts keeps, in the skip of a recovery, would commit the construct
		// around the choice in place of the choice: the choice is left as it is, with none of the
		// rewrites of its parts.
		v.applied = applied
		return ch
	case len(alts) == 1 && (v.depth == 0 && !v.observed(alts[0]) || v.sameValueBare(alts[0])):
		return alts[0]
	case !changed:
		return ch
	}
	c := *ch
	c.Alts, c.Breaks = alts, nil
	return &c
}

// sameValueBare reports whether an expression has the same value alone as it has as the only
// alternative of a choice, where a sequence around the choice takes it. The expressions it accepts
// are one element in both places, or a call of a splice rule whose body is not a sequence (isSeq),
// which is one element or none in both. A sequence is not: as an alternative it is one element,
// and alone it adds its elements to the sequence around it. Neither is a call of a splice rule
// whose body is a sequence, which adds the elements of the body in the same way. Where a predicate
// can read the value, a choice of one alternative that this does not accept stays.
func (v *normalView) sameValueBare(e grammar.Expr) bool {
	switch e.(type) {
	case *grammar.Ref:
		return !isSeqOf(v.spliceBody, e)
	case *grammar.Literal, *grammar.CharClass, *grammar.Any, *grammar.Repeat, *grammar.Optional,
		*grammar.Atomic, *grammar.Bottom:
		return true
	}
	return false
}

// spliceBody returns the body of the rule name if it is a splice rule (Program.SpliceBody), as the
// grammar writes it.
func (v *normalView) spliceBody(name string) (grammar.Expr, bool) {
	if rd := v.byName[name]; rd != nil && spliceRule(rd) {
		return rd.Expr, true
	}
	return nil, false
}

// subsumed reports whether an earlier alternative covers alt: it equals alt or is a prefix of
// it, and evaluating it again costs at most what its first evaluation cost.
func (v *normalView) subsumed(alt grammar.Expr, earlier []grammar.Expr) bool {
	for _, e := range earlier {
		if !v.evaluatedAgainCheaply(e) {
			continue
		}
		if v.same(e, alt) {
			return true
		}
		if v.prefix(items(e), items(alt)) {
			return true
		}
		if l, ok := e.(*grammar.Literal); ok && !l.IgnoreCase {
			if m, ok := alt.(*grammar.Literal); ok && !m.IgnoreCase && len(m.Value) > len(l.Value) && m.Value[:len(l.Value)] == l.Value {
				return true
			}
		}
	}
	return false
}

// prefix reports whether pre is a proper prefix of all.
func (v *normalView) prefix(pre, all []grammar.Expr) bool {
	if len(pre) >= len(all) {
		return false
	}
	for i, x := range pre {
		if !v.same(x, all[i]) {
			return false
		}
	}
	return true
}

// mergeChars applies C9 to the runs of two or more alternatives that each match one character of a
// class: a class without negation or a literal of one character, matched with case.
func (v *normalView) mergeChars(alts []grammar.Expr) ([]grammar.Expr, bool) {
	var out []grammar.Expr
	changed := false
	for i := 0; i < len(alts); {
		j := i
		var ranges []grammar.CharRange
		for j < len(alts) {
			rs, ok := singleChars(alts[j])
			if !ok {
				break
			}
			ranges = append(ranges, rs...)
			j++
		}
		if j-i >= 2 {
			v.count(ruleN8)
			changed = true
			class := &grammar.CharClass{SourceInfo: exprSource(alts[i]), Ranges: normalizeRanges(ranges)}
			v.standsFor(class, v.textOfAll(alts[i:j], " / "))
			out = append(out, class)
			i = j
			continue
		}
		out = append(out, alts[i])
		i++
	}
	return out, changed
}

// singleChars returns the characters of an expression that matches exactly one of them. A
// replacement character is left out: a literal that holds one also matches input that is not
// valid UTF-8.
func singleChars(e grammar.Expr) ([]grammar.CharRange, bool) {
	switch x := e.(type) {
	case *grammar.CharClass:
		if x.Negated || len(x.Ranges) == 0 {
			return nil, false
		}
		for _, r := range x.Ranges {
			if r.Lo <= utf8.RuneError && utf8.RuneError <= r.Hi {
				return nil, false
			}
		}
		return x.Ranges, true
	case *grammar.Literal:
		r, n := utf8.DecodeRuneInString(x.Value)
		if x.IgnoreCase || n == 0 || n != len(x.Value) || r == utf8.RuneError {
			return nil, false
		}
		return []grammar.CharRange{{Lo: r, Hi: r}}, true
	}
	return nil, false
}

// factor applies C6 to runs of alternatives that begin with the same item: p b / p c becomes
// p (b / c), where p is the longest prefix that the alternatives of the run share. The alternatives
// have no cut that could commit the choice, and p evaluated again costs at most what its first
// evaluation cost (evaluatedAgainCheaply), which is what the view drops. A run whose shared prefix
// is not cheap as a whole is left as it is: factoring a part of it would hide from the classifier
// the shared calls that it strips itself.
func (v *normalView) factor(alts []grammar.Expr, ch *grammar.Choice) ([]grammar.Expr, bool) {
	if v.depth > 0 {
		return alts, false
	}
	var out []grammar.Expr
	changed := false
	for i := 0; i < len(alts); {
		first := items(alts[i])
		j, n := i+1, len(first)
		if len(first) > 1 && !v.cut(alts[i]) {
			for j < len(alts) && !v.cut(alts[j]) {
				next := items(alts[j])
				if len(next) < 2 || !v.same(first[0], next[0]) {
					break
				}
				m := 1
				for m < min(n, len(next)) && v.same(first[m], next[m]) {
					m++
				}
				n = m
				j++
			}
		}
		prefix := first[:min(n, len(first))]
		if j-i < 2 || !v.evaluatedAgainCheaply(&grammar.Seq{Items: prefix}) || v.observed(alts[i:j]...) {
			out = append(out, alts[i])
			i++
			continue
		}
		v.count(ruleN12)
		changed = true
		// Each rest stands for its alternative, at the alternative's position, and the choice of
		// the rests for the run; the factored sequence starts where the first alternative of the
		// run starts and stands for it, since the prefix that it evaluates first is that
		// alternative's. A rest that is an expression of the grammar keeps its own text.
		rests := make([]grammar.Expr, 0, j-i)
		for _, a := range alts[i:j] {
			r := seqOf(a, items(a)[len(prefix):], false)
			v.standsFor(r, v.textOf(a))
			rests = append(rests, r)
		}
		run := &grammar.Choice{SourceInfo: exprSource(alts[i]), Alts: rests}
		v.standsFor(run, v.textOfAll(alts[i:j], " / "))
		f := &grammar.Seq{SourceInfo: exprSource(alts[i]), Items: append(slices.Clone(prefix), run)}
		v.standsFor(f, v.textOf(alts[i]))
		out = append(out, f)
		i = j
	}
	return out, changed
}

// items returns the items of a sequence, or e as the only item.
func items(e grammar.Expr) []grammar.Expr {
	if s, ok := e.(*grammar.Seq); ok {
		return s.Items
	}
	return []grammar.Expr{e}
}

// seqOf returns the sequence of items, at the position of e: the item itself when there is one,
// and a top when there is none. With tuple it returns a sequence of the items, which is a tuple of
// one element (or of none, written as a sequence of a top) and so keeps the value of the
// sequence that it replaces.
func seqOf(e grammar.Expr, items []grammar.Expr, tuple bool) grammar.Expr {
	switch {
	case len(items) == 0 && tuple:
		return &grammar.Seq{SourceInfo: exprSource(e), Items: []grammar.Expr{&grammar.Top{SourceInfo: exprSource(e)}}}
	case len(items) == 0:
		return &grammar.Top{SourceInfo: exprSource(e)}
	case len(items) == 1 && tuple:
		return &grammar.Seq{SourceInfo: exprSource(e), Items: slices.Clone(items)}
	case len(items) == 1:
		return items[0]
	}
	return &grammar.Seq{SourceInfo: exprSource(e), Items: slices.Clone(items)}
}

// leadingCut reports whether alt is a sequence that starts with a cut: the choice commits to it
// before it examines anything.
func leadingCut(alt grammar.Expr) bool {
	s, ok := alt.(*grammar.Seq)
	if !ok || len(s.Items) == 0 {
		_, isCut := alt.(*grammar.Cut)
		return isCut
	}
	_, isCut := s.Items[0].(*grammar.Cut)
	return isCut
}

// dropCuts removes the cuts of an alternative that cannot change what the choice does: every loose
// cut of the last alternative (all), or the cut that ends an alternative. A loose cut of the last
// alternative commits the choice, which has nothing left to try, wherever it is nested: in a
// sequence, a group, a capture, a discard, an atomic, a stream, the expression of a recovery or an
// attributed expression (looseCutsDropped). Dropping all of them also keeps a cut from committing
// an enclosing construct when the choice is replaced by its only alternative. A cut that ends an
// alternative is removed only as an item of the alternative itself.
func (v *normalView) dropCuts(alt grammar.Expr, all bool) grammar.Expr {
	// A sequence that loses its cuts and keeps one item or none would become that item, which
	// changes its value from a tuple to the item's. Where a predicate can read it, it stays a
	// sequence of its items.
	tuple := v.depth > 0 || v.observed(alt)
	if _, ok := alt.(*grammar.Cut); ok || all {
		return v.looseCutsDropped(alt, tuple)
	}
	s, ok := alt.(*grammar.Seq)
	if !ok || len(s.Items) == 0 {
		return alt
	}
	if _, isCut := s.Items[len(s.Items)-1].(*grammar.Cut); !isCut {
		return alt
	}
	if len(s.Items) < 3 {
		return seqOf(alt, s.Items[:len(s.Items)-1], tuple)
	}
	c := *s
	c.Items, c.Breaks = s.Items[:len(s.Items)-1:len(s.Items)-1], nil
	return &c
}

// looseCutsDropped returns e without the cuts that are loose in it (grammaranalysis.HasLooseCut),
// or e itself when it has none outside the skips of recoveries, which it keeps. A cut that is an
// item of a sequence is removed, and a cut that is the whole of an expression becomes a top. Every
// expression that it rebuilds stands for the text of the one it replaces. With tuple, a sequence
// that keeps fewer than two items stays a sequence (seqOf).
func (v *normalView) looseCutsDropped(e grammar.Expr, tuple bool) grammar.Expr {
	n := v.looseCutsDroppedIn(e, tuple)
	if n != e {
		v.standsFor(n, v.textOf(e))
	}
	return n
}

func (v *normalView) looseCutsDroppedIn(e grammar.Expr, tuple bool) grammar.Expr {
	switch x := e.(type) {
	case *grammar.Cut:
		return &grammar.Top{SourceInfo: exprSource(x)}
	case *grammar.Seq:
		var items []grammar.Expr
		changed := false
		for _, it := range x.Items {
			if _, isCut := it.(*grammar.Cut); isCut {
				changed = true
				continue
			}
			n := v.looseCutsDropped(it, tuple)
			changed = changed || n != it
			items = append(items, n)
		}
		if !changed {
			return x
		}
		if len(items) < 2 {
			return seqOf(x, items, tuple)
		}
		c := *x
		c.Items, c.Breaks = items, nil
		return &c
	case *grammar.Group:
		if n := v.looseCutsDropped(x.Expr, tuple); n != x.Expr {
			c := *x
			c.Expr = n
			return &c
		}
	case *grammar.Capture:
		if n := v.looseCutsDropped(x.Expr, tuple); n != x.Expr {
			c := *x
			c.Expr = n
			return &c
		}
	case *grammar.Discard:
		if n := v.looseCutsDropped(x.Expr, tuple); n != x.Expr {
			c := *x
			c.Expr = n
			return &c
		}
	case *grammar.Atomic:
		if n := v.looseCutsDropped(x.Expr, tuple); n != x.Expr {
			c := *x
			c.Expr = n
			return &c
		}
	case *grammar.Stream:
		if n := v.looseCutsDropped(x.Expr, tuple); n != x.Expr {
			c := *x
			c.Expr = n
			return &c
		}
	case *grammar.Recovery:
		if n := v.looseCutsDropped(x.Expr, tuple); n != x.Expr {
			c := *x
			c.Expr = n
			return &c
		}
	case *grammar.Attributed:
		if n := v.looseCutsDropped(x.Expr, tuple); n != x.Expr {
			c := *x
			c.Expr = n
			return &c
		}
	}
	return e
}

// lookahead applies K2 and K1 to a lookahead whose expression is a lookahead.
func (v *normalView) lookahead(e grammar.Expr, body grammar.Expr, not bool) grammar.Expr {
	inner := v.expr(body)
	rebuild := func(not bool, x grammar.Expr) grammar.Expr {
		if not {
			return &grammar.Not{SourceInfo: exprSource(e), Expr: x}
		}
		return &grammar.And{SourceInfo: exprSource(e), Expr: x}
	}
	switch in := inner.(type) {
	case *grammar.And:
		if !v.on(ruleN5) {
			break
		}
		// K2: &&X is &X and !&X is !X: the outer lookahead keeps the result of the inner one,
		// which returns to its start.
		v.count(ruleN5)
		return rebuild(not, in.Expr)
	case *grammar.Not:
		if !v.on(ruleN5) {
			break
		}
		if !not {
			// K2: &!X is !X.
			v.count(ruleN5)
			return in
		}
		if !bindsOutsideCalls(in.Expr) {
			// K1: !!X is &X when X makes no binding that the positive lookahead would keep.
			v.count(ruleN5)
			return rebuild(false, in.Expr)
		}
	}
	if inner == body {
		return e
	}
	return rebuild(not, inner)
}

// attributed applies V1 and, with labels, N7.
func (v *normalView) attributed(a *grammar.Attributed) grammar.Expr {
	inner := v.expr(a.Expr)
	// A recovery makes an expression with a value one element, as attrElems builds it. Without
	// the recovery a sequence around a sequence takes its elements, so the recovery stays where a
	// predicate can read the value. It stays there also on an expression without a value, which
	// it leaves without one, as the gate does not look at the shape of the value.
	shaped := v.depth > 0 || v.observed(inner)
	var attrs []*grammar.Attribute
	for _, at := range a.Attrs {
		switch {
		case at.Name == "recover" && v.on(ruleN6) && !shaped && v.an.AlwaysSucceeds(inner):
			// V1: a recovery of an expression that always succeeds never runs its skip.
			v.count(ruleN6)
			continue
		case v.opts.labels && v.on(ruleN7) && (at.Name == "error" || at.Name == "diagnostic"):
			// A label changes the messages of syntax errors only.
			v.count(ruleN7)
			continue
		}
		attrs = append(attrs, at)
	}
	if len(attrs) == 0 {
		return inner
	}
	if inner == a.Expr && len(attrs) == len(a.Attrs) {
		return a
	}
	c := *a
	c.Expr, c.Attrs, c.Breaks = inner, attrs, nil
	return &c
}

// inline returns the view of the body of the rule that r calls when P1 lets the call be replaced
// by it, or nil: the rule is not memoized, so the call costs what its body costs, it is on no
// cycle of calls, and its body has no action, binding, capture or cut that the call would delimit.
func (v *normalView) inline(r *grammar.Ref) grammar.Expr {
	rd := v.byName[r.Name]
	if rd == nil || r.Level != "" || v.memo[r.Name] || v.an.cyclic[r.Name] || v.inlining[r.Name] ||
		rd.Action != nil || len(rd.Attrs) != 0 {
		return nil
	}
	if body, ok := v.inlined[r.Name]; ok {
		return body
	}
	ok := !grammaranalysis.HasLooseCut(rd.Expr)
	WalkExpr(rd.Expr, func(x grammar.Expr) {
		switch x := x.(type) {
		case *grammar.Capture, *grammar.InlineAction, *grammar.OperatorsRule, *grammar.Stream, *grammar.Recovery:
			ok = false
		case *grammar.Predicate:
			ok = ok && !termBinds(x.Term)
		case *grammar.Attributed:
			ok = ok && !hasPragma(x, "missing") && !hasPragma(x, "recover")
		case *grammar.Ref:
			ok = ok && x.Name != r.Name
		}
	})
	if !ok {
		v.inlined[r.Name] = nil
		return nil
	}
	v.inlining[r.Name] = true
	depth := v.depth
	v.depth = 0
	if v.valued[r.Name] {
		v.depth = 1
	}
	body := v.fix(rd.Expr)
	v.depth = depth
	v.inlining[r.Name] = false
	v.inlined[r.Name] = body
	return body
}

// valueReaders computes the captures that a predicate reads (reads) and the rules whose value
// reaches one (valued): the rules called, through positions that keep their value, from inside such
// a capture, and so on from their bodies. The rules of the view that change values, which keep only
// ≡E,X, are off there, because spec/proofs/grammar-laws.md states ≡E for an expression only in a
// context that reads no value of it in a predicate. Elsewhere the rules apply.
func (v *normalView) valueReaders(rules []*grammar.RuleDef) {
	v.reads, v.valued = map[string]map[string]bool{}, map[string]bool{}
	var work []string
	mark := func(e grammar.Expr) {
		for _, n := range valueCalls(e) {
			if !v.valued[n] {
				v.valued[n] = true
				work = append(work, n)
			}
		}
	}
	for _, r := range rules {
		reads := map[string]bool{}
		WalkExpr(r.Expr, func(x grammar.Expr) {
			if p, ok := x.(*grammar.Predicate); ok {
				WalkTerm(p.Term, func(t grammar.Term) {
					if c, ok := t.(*grammar.CaptureRef); ok && !c.Param && !c.Pragma {
						reads[c.Name] = true
					}
				})
			}
		})
		if len(reads) == 0 {
			continue
		}
		v.reads[r.Name] = reads
		WalkExpr(r.Expr, func(x grammar.Expr) {
			if c, ok := x.(*grammar.Capture); ok && reads[c.Name] {
				mark(c.Expr)
			}
		})
	}
	for len(work) > 0 {
		n := work[len(work)-1]
		work = work[:len(work)-1]
		if rd := v.byName[n]; rd != nil {
			mark(rd.Expr)
		}
	}
}

// observed reports whether a predicate of the current rule can read a value in es: one contains a
// capture whose name a predicate of the rule reads, or calls, in positions that keep their value, a
// rule whose value reaches such a capture. The rules that change values do not rewrite such an
// expression, whose captures they would move or merge.
func (v *normalView) observed(es ...grammar.Expr) bool {
	reads := v.reads[v.cur]
	for _, e := range es {
		found := false
		WalkExpr(e, func(x grammar.Expr) {
			if c, ok := x.(*grammar.Capture); ok && reads[c.Name] {
				found = true
			}
		})
		if found {
			return true
		}
		for _, n := range valueCalls(e) {
			if v.valued[n] {
				return true
			}
		}
	}
	return false
}

// valueCalls returns the rules that e calls in positions that can keep their value: all but those
// under a discard, a negative lookahead, and a positive lookahead outside its captures. A capture
// in a positive lookahead keeps its value beside the tuple of its scope, where actions and
// predicates read it. A negative lookahead has no captures and a capture under a discard is an
// error, so no value survives them.
func valueCalls(e grammar.Expr) []string {
	dropped := map[*grammar.Ref]bool{}
	drop := func(x grammar.Expr) {
		WalkExpr(x, func(y grammar.Expr) {
			if r, ok := y.(*grammar.Ref); ok {
				dropped[r] = true
			}
		})
	}
	WalkExpr(e, func(x grammar.Expr) {
		switch x := x.(type) {
		case *grammar.Discard:
			drop(x.Expr)
		case *grammar.And:
			kept := map[*grammar.Ref]bool{}
			WalkExpr(x.Expr, func(y grammar.Expr) {
				if c, ok := y.(*grammar.Capture); ok {
					WalkExpr(c.Expr, func(z grammar.Expr) {
						if r, ok := z.(*grammar.Ref); ok {
							kept[r] = true
						}
					})
				}
			})
			WalkExpr(x.Expr, func(y grammar.Expr) {
				if r, ok := y.(*grammar.Ref); ok && !kept[r] {
					dropped[r] = true
				}
			})
		case *grammar.Not:
			drop(x.Expr)
		}
	})
	var out []string
	WalkExpr(e, func(x grammar.Expr) {
		if r, ok := x.(*grammar.Ref); ok && !dropped[r] {
			out = append(out, r.Name)
		}
	})
	return out
}

// cut reports whether e has a cut that would commit the choice, option or repetition around it.
func (v *normalView) cut(e grammar.Expr) bool { return grammaranalysis.HasLooseCut(e) }

// idempotent reports the conditions on X of R3 under which a repetition of X evaluated again
// where one stopped matches nothing: X has no cut that commits the repetition, and it cannot match
// without consuming input or makes no binding; and evaluating X again costs at most what its first
// evaluation cost.
func (v *normalView) idempotent(x grammar.Expr) bool {
	return !v.cut(x) && (!v.an.isNullable(x) || !bindsOutsideCalls(x)) && v.evaluatedAgainCheaply(x)
}

// cheapRules computes the rules whose calls cost, when they are evaluated again in a state in
// which they were evaluated before, at most what the first evaluation cost: the memoized rules,
// which the memo answers the second time, and the rules whose bodies are evaluated again cheaply
// and call only such rules. It is the least fixed point, so a recursion through rules that are not
// memoized is never cheap: its evaluations again could compound.
func (v *normalView) cheapRules(rules []*grammar.RuleDef) {
	v.cheap = map[string]bool{}
	for _, r := range rules {
		// A call of a rule of a left-recursion cycle can read the seed of a growth, which the
		// classifier analyzes as the grammar is written.
		v.cheap[r.Name] = v.memo[r.Name] && !v.an.cyclic[r.Name]
	}
	for changed := true; changed; {
		changed = false
		for _, r := range rules {
			v.control.Poll()
			if !v.cheap[r.Name] && !v.an.cyclic[r.Name] && r.Action == nil && v.evaluatedAgainCheaply(r.Expr) {
				v.cheap[r.Name], changed = true, true
			}
		}
	}
}

// evaluatedAgainCheaply reports whether an evaluation of e made again in a state in which it was
// made before costs at most a constant factor of the first, whatever the input: every rule that e
// calls is cheap (cheapRules), and e has nothing else whose cost the runtime does not bound by the
// input that e examines (a predicate or an inline action, which can build values, a recovery, a
// stream, an Operators Rule or a #missing).
func (v *normalView) evaluatedAgainCheaply(e grammar.Expr) bool {
	ok := true
	WalkExpr(e, func(x grammar.Expr) {
		switch x := x.(type) {
		case *grammar.Ref:
			ok = ok && x.Level == "" && v.cheap[x.Name]
		case *grammar.Predicate, *grammar.InlineAction, *grammar.Recovery, *grammar.Stream, *grammar.OperatorsRule:
			ok = false
		case *grammar.Attributed:
			ok = ok && !hasPragma(x, "missing") && !hasPragma(x, "recover") && !hasPragma(x, "stream")
		}
	})
	return ok
}

// boundedTerminal reports whether e examines at most a constant amount of input and calls no
// rule: dropping its evaluations drops a constant cost.
func boundedTerminal(e grammar.Expr) bool {
	ok := true
	WalkExpr(e, func(x grammar.Expr) {
		switch x := x.(type) {
		case *grammar.Literal, *grammar.CharClass, *grammar.Any, *grammar.Seq, *grammar.Choice, *grammar.Group:
		case *grammar.Repeat:
			ok = ok && x.Max >= 0
		default:
			ok = false
		}
	})
	return ok
}

// bindsOutsideCalls reports whether e makes a binding that lasts after it: a binding in a
// predicate or an inline action outside the rules that it calls.
func bindsOutsideCalls(e grammar.Expr) bool {
	found := false
	WalkExpr(e, func(x grammar.Expr) {
		switch x := x.(type) {
		case *grammar.Predicate:
			found = found || termBinds(x.Term)
		case *grammar.InlineAction:
			found = true
		}
	})
	return found
}

// termBinds reports whether t has a binding.
func termBinds(t grammar.Term) bool {
	found := false
	WalkTerm(t, func(x grammar.Term) {
		if _, ok := x.(*grammar.Bind); ok {
			found = true
		}
	})
	return found
}

// same reports whether two expressions are the same expression, whatever their positions and
// layout.
func (v *normalView) same(a, b grammar.Expr) bool {
	if a == b {
		return true
	}
	return grammar.FormatExpr(a) == grammar.FormatExpr(b)
}

// exprSource returns the source information of an expression, for a node that the view builds in
// its place.
func exprSource(e grammar.Expr) grammar.SourceInfo {
	return grammar.SourceInfo{Pos: exprPos(e)}
}

// NormalView returns the grammar that the compiler runs for g, with inline actions lowered and
// attributes normalized, and with the bodies of its rules replaced by their normal view. With
// matching, the view also drops labels and inlines the calls that P1 allows: it then keeps only
// what a parse matches and where it ends (≡E), which is what an analysis of matching, such as the
// exclusion of alternatives, needs. Without matching it is the view that the complexity
// classifier reads. g must compile.
func NormalView(g *grammar.Grammar, matching bool, control *cancel.Control) (*grammar.Grammar, error) {
	v, g, err := normalViewOf(g, matching, nil, control)
	if err != nil {
		return nil, err
	}
	byName := map[string]*grammar.RuleDef{}
	for _, r := range v.rules {
		byName[r.Name] = r
	}
	out := *g
	out.File.Decls = slices.Clone(g.File.Decls)
	for i, d := range out.File.Decls {
		if r, ok := d.(*grammar.RuleDef); ok {
			out.File.Decls[i] = byName[r.Name]
		}
	}
	return &out, nil
}
