package complexity

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/ir/core/facts"
)

// builder computes the view of a source module. It rewrites a rule body bottom up, again and again
// until no rule applies (fix), and never changes a node of the module: a node whose parts change
// is copied.
type builder struct {
	src     *Source
	opts    Options
	control *cancel.Control
	view    *View
	// orig holds the nodes of the source module.
	orig map[core.Node]bool
	// values are the value conditions of the rules (valueReaders); cur is the rule whose body is
	// being rewritten, and depth is positive inside a value that a predicate of it can read, where
	// the rewrites that change values are off.
	values *values
	cur    int
	depth  int
	// cheap are the rules whose calls evaluated again cost at most a constant factor of their
	// first evaluation (cheapRules).
	cheap []bool
	// exps and classes intern the expectations and classes of the view's module, which a class
	// that the view builds adds to.
	exps    map[string]core.ExpID
	classes map[string]*core.Class
}

// on reports whether the rule l applies.
func (b *builder) on(l Law) bool { return b.opts.Laws == nil || slices.Contains(b.opts.Laws, l) }

func (b *builder) count(l Law) { b.view.Applied[l]++ }

// fix rewrites n until no rule applies. Every rule lowers the measure of
// spec/proofs/grammar-laws.md (Termination), so the loop ends; the bound only guards against a
// defect of a rule.
func (b *builder) fix(n core.Node) core.Node {
	for range 64 {
		next := b.node(n)
		if next == n {
			return n
		}
		n = next
	}
	return n
}

// node rewrites the parts of n and then n. It returns n itself when nothing changes. A node that
// it builds in place of n stands for the text of n.
func (b *builder) node(n core.Node) core.Node {
	r := b.rewrite(n)
	if r != n {
		b.standsFor(r, n)
	}
	return r
}

// standsFor records that x, if the view built it, stands for the text of the node n.
func (b *builder) standsFor(x, n core.Node) {
	if b.orig[x] {
		return
	}
	b.standsForText(x, b.view.textOf(n))
}

// standsForText records that x, if the view built it, stands for the source text t.
func (b *builder) standsForText(x core.Node, t string) {
	if b.orig[x] {
		return
	}
	if _, ok := b.view.Text[x]; !ok {
		b.view.Text[x] = t
	}
}

// textOfAll returns the source texts that ns stand for, joined by sep.
func (b *builder) textOfAll(ns []core.Node, sep string) string {
	ts := make([]string, len(ns))
	for i, n := range ns {
		ts[i] = b.view.textOf(n)
	}
	return strings.Join(ts, sep)
}

// built records that x, a node the view builds, replaces n with new parts: it is written as the
// expression of the node of the module that n is or replaces (View.Grammar).
func (b *builder) built(x, n core.Node) core.Node {
	*x.Attrs() = core.Attrs{}
	if s, ok := b.view.replaces[n]; ok {
		n = s
	}
	b.view.replaces[x] = n
	return x
}

func (b *builder) rewrite(n core.Node) core.Node {
	b.control.Poll()
	switch x := n.(type) {
	case *core.Seq:
		return b.seq(x)
	case *core.Choice:
		return b.choice(x)
	case *core.Repeat:
		body := b.node(x.Body)
		if b.on(N10) {
			if r := b.nestedRepeat(x, body); r != nil {
				return r
			}
		}
		if body != x.Body {
			c := *x
			c.Body = body
			return b.built(&c, x)
		}
	case *core.Opt:
		if body := b.node(x.Body); body != x.Body {
			c := *x
			c.Body = body
			return b.built(&c, x)
		}
	case *core.Look:
		return b.lookahead(x)
	case *core.Capture:
		read := b.values.reads[b.cur][x.Name]
		if read {
			b.depth++
		}
		body := b.node(x.Body)
		if read {
			b.depth--
		}
		if body != x.Body {
			c := *x
			c.Body = body
			return b.built(&c, x)
		}
	case *core.Drop:
		if body := b.node(x.Body); body != x.Body {
			c := *x
			c.Body = body
			return b.built(&c, x)
		}
	case *core.Text:
		if body := b.node(x.Body); body != x.Body {
			c := *x
			c.Body = body
			return b.built(&c, x)
		}
	case *core.Label:
		if body := b.node(x.Body); body != x.Body {
			c := *x
			c.Body = body
			return b.built(&c, x)
		}
	case *core.Recover:
		return b.recover(x)
	case *core.Stream:
		// The rules keep a repetition a repetition, so the stream keeps its repetition.
		if r, ok := b.node(x.Repeat).(*core.Repeat); ok && r != x.Repeat {
			c := *x
			c.Repeat = r
			return b.built(&c, x)
		}
	}
	// Terminals, calls, predicates, bindings, #missing and Operators Rules are kept as they are:
	// the classifier reads their structure as the runtime runs it.
	return n
}

// seq applies N1 (splice), N4 (leading bottom), N9 (repetitions in a row), N11 (unrolling) and
// N12 (!X X) to a sequence.
func (b *builder) seq(s *core.Seq) core.Node {
	changed := false
	var items []core.Node
	for _, it := range s.Items {
		n := b.node(it)
		if n != it {
			changed = true
		}
		if inner, ok := n.(*core.Seq); ok && b.on(N1) {
			// S1: a sequence in a sequence adds its elements to the surrounding one.
			b.count(N1)
			changed = true
			items = append(items, inner.Items...)
			continue
		}
		items = append(items, n)
	}
	for i := 0; i+1 < len(items); i++ {
		if m := b.adjacent(items[i], items[i+1]); m != nil {
			b.standsForText(m, b.textOfAll(items[i:i+2], " "))
			items = slices.Replace(slices.Clip(items), i, i+2, m)
			changed = true
			i = max(i-2, -1)
		}
	}
	for i := 0; i+1 < len(items); i++ {
		if _, ok := items[i].(*core.FailNode); ok && b.on(N4) {
			// S2: nothing after a bottom is evaluated.
			b.count(N4)
			items, changed = items[:i+1], true
			break
		}
	}
	if len(items) == 1 && b.on(N1) {
		_, bottom := items[0].(*core.FailNode)
		if bottom || b.depth == 0 && !b.values.observed(b.cur, items[0]) {
			// A sequence of one item matches as the item does, and has the value of a tuple of
			// one element where the item does not: a predicate that reads it sees the difference
			// (spec/language/parser-expressions.md, Values), so the sequence stays where one can.
			if len(s.Items) != 1 || changed {
				b.count(N1)
			}
			return items[0]
		}
	}
	if !changed {
		return s
	}
	return b.built(&core.Seq{Items: items}, s)
}

// adjacent rewrites two items in a row into one, or returns nil.
func (b *builder) adjacent(x, y core.Node) core.Node {
	rx, xRep := x.(*core.Repeat)
	ry, yRep := y.(*core.Repeat)
	if yRep && ry.Max < 0 && ry.Min <= 1 && b.depth == 0 && (b.on(N9) || b.on(N11)) && !b.values.observed(b.cur, x, y) {
		z := ry.Body
		switch {
		case !b.on(N9):
		case xRep && rx.Max < 0 && rx.Min <= 1 && ry.Min == 0 && b.same(rx.Body, z) && b.idempotent(z):
			// R3 and R4: X* X* and X+ X* match as the first repetition does. The second evaluates
			// X once more where the first stopped, in a state that differs only by bindings that X
			// does not read.
			b.count(N9)
			return rx
		case !xRep && ry.Min == 0 && b.same(x, z) && b.idempotent(z):
			// R5: X X* is X+.
			b.count(N9)
			c := *ry
			c.Min = 1
			return b.built(&c, ry)
		}
		if o, ok := x.(*core.Opt); ok && b.on(N9) && ry.Min == 0 && b.same(o.Body, z) && b.idempotent(z) && !b.src.Facts.Nullable(z) {
			// R4: X? X* matches as X* does.
			b.count(N9)
			return ry
		}
		if xRep && rx.Max < 0 && rx.Min == 0 && ry.Min == 0 && b.on(N11) {
			if r := b.unroll(rx, ry); r != nil {
				return r
			}
		}
	}
	if n, ok := x.(*core.Look); ok && n.Neg && b.on(N12) && b.depth == 0 && b.same(n.Body, y) && boundedTerminal(y) {
		// K4: !X X never matches. X examines a bounded amount of input, so the view drops a
		// constant cost. !X X has the type of X and _|_ has the type _|_, which a choice around it
		// leaves out of its join: (-"b" / !"a" "a") is one element, nil or a string, and
		// (-"b" / _|_) none. So the rewrite stays where a predicate can read the value. X makes
		// no capture, so only depth decides.
		b.count(N12)
		return &core.FailNode{}
	}
	return nil
}

// choice applies N1 (flatten), N2, N3, N4, N6 (cuts), N8 and N12 (left factoring) to a choice.
func (b *builder) choice(ch *core.Choice) core.Node {
	applied := maps.Clone(b.view.Applied)
	changed := false
	var alts []core.Node
	for i, alt := range ch.Alts {
		n := b.node(alt.Body)
		if n != alt.Body {
			changed = true
		}
		if inner, ok := n.(*core.Choice); ok && b.on(N1) && (i == len(ch.Alts)-1 || !slices.ContainsFunc(inner.Alts, looseCutAlt)) {
			// C2: a choice that is the last alternative splices always; an earlier one splices
			// when no cut in it would commit the outer choice instead of it.
			b.count(N1)
			changed = true
			for _, a := range inner.Alts {
				alts = append(alts, a.Body)
			}
			continue
		}
		alts = append(alts, n)
	}
	// C1, C3, C5 and T2 drop alternatives that never give the value of the choice, but they can
	// drop every alternative with a value and keep one without: the choice that gave nil gives no
	// element then, as (_ / "b") with "b" dropped. So they apply only where no predicate can read
	// the value. A bottom alternative adds nothing to the value of the choice, which joins the
	// types of its alternatives, so C4 applies everywhere.
	drops := b.depth == 0
	var kept []core.Node
	for i, alt := range alts {
		if _, ok := alt.(*core.FailNode); ok && b.on(N4) {
			// C4: a bottom alternative fails at once and records nothing.
			b.count(N4)
			changed = true
			continue
		}
		if !drops {
			kept = append(kept, alt)
			continue
		}
		if b.on(N3) && b.subsumed(alt, kept) {
			// C1 and C5: an earlier alternative matched wherever this one could, and when it
			// failed, this one fails in the same state.
			b.count(N3)
			changed = true
			continue
		}
		kept = append(kept, alt)
		if b.on(N2) && (b.src.Facts.Succeeds(alt) || leadingCut(alt)) {
			// C3: no later alternative is tried after one that always succeeds; T2: nor after a
			// cut that the alternative passes before anything else.
			if i < len(alts)-1 {
				b.count(N2)
				changed = true
			}
			break
		}
	}
	alts = kept
	if len(alts) > 0 && b.on(N6) {
		// T2: a cut in the last alternative commits a choice that has nothing left to try.
		last := alts[len(alts)-1]
		if n := b.dropCuts(last, true); n != last {
			b.count(N6)
			b.standsFor(n, last)
			alts, changed = append(alts[:len(alts)-1:len(alts)-1], n), true
		}
	}
	for i := range alts {
		if !b.on(N6) {
			break
		}
		// T2: a cut that ends an alternative commits when nothing in it can fail any more.
		if n := b.dropCuts(alts[i], false); n != alts[i] {
			b.count(N6)
			b.standsFor(n, alts[i])
			alts, changed = slices.Clone(alts), true
			alts[i] = n
		}
	}
	if b.on(N8) {
		if merged, ok := b.mergeChars(alts); ok {
			alts, changed = merged, true
		}
	}
	if b.on(N12) {
		if factored, ok := b.factor(alts); ok {
			alts, changed = factored, true
		}
	}
	switch {
	case len(alts) == 0:
		return b.built(&core.FailNode{}, ch)
	case len(alts) == 1 && facts.LooseCut(alts[0]):
		// A loose cut that dropCuts keeps, in the skip of a recovery, would commit the construct
		// around the choice in place of the choice: the choice is left as it is, with none of the
		// rewrites of its parts.
		b.view.Applied = applied
		return ch
	case len(alts) == 1 && (b.depth == 0 && !b.values.observed(b.cur, alts[0]) || b.sameValueBare(alts[0])):
		return alts[0]
	case !changed:
		return ch
	}
	c := &core.Choice{Alts: make([]*core.Alt, len(alts))}
	for i, a := range alts {
		c.Alts[i] = &core.Alt{Body: a}
	}
	return b.built(c, ch)
}

func looseCutAlt(a *core.Alt) bool { return facts.LooseCut(a.Body) }

// unroll applies R7 to X* (Y X*)*, giving (X / Y)*, and the absorption X* (X / Y)* to (X / Y)*.
func (b *builder) unroll(rx, ry *core.Repeat) core.Node {
	x := rx.Body
	if b.src.Facts.Nullable(x) || facts.LooseCut(x) {
		return nil
	}
	if ch, ok := ry.Body.(*core.Choice); ok && len(ch.Alts) > 1 && b.same(ch.Alts[0].Body, x) && b.idempotent(x) {
		// The absorption: the second repetition evaluates X again where the first stopped.
		b.count(N11)
		return ry
	}
	s, ok := ry.Body.(*core.Seq)
	if !ok || len(s.Items) < 2 {
		return nil
	}
	last, ok := s.Items[len(s.Items)-1].(*core.Repeat)
	if !ok || last.Min != 0 || last.Max >= 0 || !b.same(last.Body, x) {
		return nil
	}
	var y core.Node
	if len(s.Items) == 2 {
		y = s.Items[0]
	} else {
		y = b.built(&core.Seq{Items: slices.Clone(s.Items[:len(s.Items)-1])}, s)
	}
	if facts.LooseCut(y) || bindsOutsideCalls(y) || !b.idempotent(x) {
		return nil
	}
	// R7: both evaluate X first at each position and Y after X fails, in the same states. When Y
	// matches without consuming input, the unrolled form evaluates X again where it failed, which
	// fails again only if Y bound nothing that X reads, and which the view drops: so Y makes no
	// binding outside its calls, and X evaluated again costs at most what it cost first.
	b.count(N11)
	c := *ry
	c.Body = &core.Choice{Alts: []*core.Alt{{Body: x}, {Body: y}}}
	return b.built(&c, ry)
}

// nestedRepeat applies R6 to a repetition whose element, rewritten, is body, or returns nil.
func (b *builder) nestedRepeat(outer *core.Repeat, body core.Node) core.Node {
	if outer.Max >= 0 || outer.Min > 1 || b.depth > 0 || b.values.observed(b.cur, outer) {
		return nil
	}
	switch in := body.(type) {
	case *core.Repeat:
		if in.Max >= 0 || in.Min > 1 || outer.Min == 1 && in.Min == 1 {
			return nil
		}
		if !b.idempotent(in.Body) {
			return nil
		}
		// (X*)*, (X+)* and (X*)+ match as X* does: the outer repetition stops after an iteration
		// that matches nothing, which evaluates X once more where the inner one stopped.
		b.count(N10)
		c := *in
		c.Min = 0
		return b.built(&c, in)
	case *core.Opt:
		if outer.Min != 0 || b.src.Facts.Nullable(in.Body) || !b.idempotent(in.Body) {
			return nil
		}
		// (X?)* matches as X* does for an X that cannot match without consuming input.
		b.count(N10)
		c := *outer
		c.Body = in.Body
		return b.built(&c, outer)
	}
	return nil
}

// lookahead applies K2 and K1 to a lookahead whose body is a lookahead. A lookahead it builds in
// place of two is new: it is written in the grammar's syntax of its own.
func (b *builder) lookahead(l *core.Look) core.Node {
	body := b.node(l.Body)
	if in, ok := body.(*core.Look); ok && b.on(N5) {
		switch {
		case !in.Neg:
			// K2: &&X is &X and !&X is !X: the outer lookahead keeps the result of the inner one,
			// which returns to its start.
			b.count(N5)
			return &core.Look{Body: in.Body, Neg: l.Neg, Site: l.Site}
		case !l.Neg:
			// K2: &!X is !X.
			b.count(N5)
			return in
		case !bindsOutsideCalls(in.Body):
			// K1: !!X is &X when X makes no binding that the positive lookahead would keep.
			b.count(N5)
			return &core.Look{Body: in.Body, Site: l.Site}
		}
	}
	if body == l.Body {
		return l
	}
	c := *l
	c.Body = body
	return b.built(&c, l)
}

// idempotent reports the conditions on X of R3 under which a repetition of X evaluated again
// where one stopped matches nothing: X has no cut that commits the repetition, and it cannot match
// without consuming input or makes no binding; and evaluating X again costs at most what its first
// evaluation cost. A node that is evaluated again cheaply makes no binding, so the last condition
// implies the second.
func (b *builder) idempotent(x core.Node) bool {
	return !facts.LooseCut(x) && b.evaluatedAgainCheaply(x)
}

// bindsOutsideCalls reports whether n makes a binding that lasts after it: a binding outside the
// rules that it calls. The bindings of an inlined call (a nest) are the call's own.
func bindsOutsideCalls(n core.Node) bool {
	found := false
	var walk func(core.Node)
	walk = func(n core.Node) {
		switch n.(type) {
		case *core.BindNode:
			found = true
			return
		case *core.Nest:
			return
		}
		for _, c := range core.Children(n) {
			walk(c)
		}
	}
	walk(n)
	return found
}

// same reports whether two nodes are the same expression: they print the same, whatever their
// positions in the module.
func (b *builder) same(x, y core.Node) bool {
	if x == y {
		return true
	}
	return core.PrintNode(b.src.Module, x, false) == core.PrintNode(b.src.Module, y, false)
}

// subsumed reports whether an earlier alternative covers alt: it equals alt or is a prefix of
// it, and evaluating it again costs at most what its first evaluation cost.
func (b *builder) subsumed(alt core.Node, earlier []core.Node) bool {
	for _, e := range earlier {
		if !b.evaluatedAgainCheaply(e) {
			continue
		}
		if b.same(e, alt) || b.prefix(items(e), items(alt)) {
			return true
		}
		if l, ok := e.(*core.Lit); ok && !l.Fold {
			if m, ok := alt.(*core.Lit); ok && !m.Fold && len(m.Text) > len(l.Text) && strings.HasPrefix(m.Text, l.Text) {
				return true
			}
		}
	}
	return false
}

// prefix reports whether pre is a proper prefix of all.
func (b *builder) prefix(pre, all []core.Node) bool {
	if len(pre) >= len(all) {
		return false
	}
	for i, x := range pre {
		if !b.same(x, all[i]) {
			return false
		}
	}
	return true
}

// items returns the items of a sequence, or n as the only item.
func items(n core.Node) []core.Node {
	if s, ok := n.(*core.Seq); ok {
		return s.Items
	}
	return []core.Node{n}
}

// mergeChars applies C9 to the runs of two or more alternatives that each match one character of a
// class: a class without negation or a literal of one character, matched with case.
func (b *builder) mergeChars(alts []core.Node) ([]core.Node, bool) {
	var out []core.Node
	changed := false
	for i := 0; i < len(alts); {
		j := i
		var ranges []core.Range
		for j < len(alts) {
			rs, ok := singleChars(alts[j])
			if !ok {
				break
			}
			ranges = append(ranges, rs...)
			j++
		}
		if j-i >= 2 {
			b.count(N8)
			changed = true
			class := &core.ClassNode{Class: b.class(ranges)}
			b.standsForText(class, b.textOfAll(alts[i:j], " / "))
			out = append(out, class)
			i = j
			continue
		}
		out = append(out, alts[i])
		i++
	}
	return out, changed
}

// singleChars returns the characters of a node that matches exactly one of them. A replacement
// character is left out: a literal that holds one also matches input that is not valid UTF-8.
func singleChars(n core.Node) ([]core.Range, bool) {
	switch x := n.(type) {
	case *core.ClassNode:
		if x.Class.Negated || len(x.Class.Ranges) == 0 {
			return nil, false
		}
		for _, r := range x.Class.Ranges {
			if r.Lo <= utf8.RuneError && utf8.RuneError <= r.Hi {
				return nil, false
			}
		}
		return x.Class.Ranges, true
	case *core.Lit:
		r, n := utf8.DecodeRuneInString(x.Text)
		if x.Fold || n == 0 || n != len(x.Text) || r == utf8.RuneError {
			return nil, false
		}
		return []core.Range{{Lo: r, Hi: r}}, true
	}
	return nil, false
}

// class returns the class of the characters of rs, sorted and merged, as the view's module holds
// it: equal classes are one class, and its expectation is that of a class the grammar writes.
func (b *builder) class(rs []core.Range) *core.Class {
	rs = slices.Clone(rs)
	slices.SortFunc(rs, func(x, y core.Range) int { return int(x.Lo) - int(y.Lo) })
	var ranges []grammar.CharRange
	for _, r := range rs {
		if r.Lo > r.Hi {
			continue
		}
		if n := len(ranges); n > 0 && int64(r.Lo) <= int64(ranges[n-1].Hi)+1 {
			ranges[n-1].Hi = max(ranges[n-1].Hi, r.Hi)
			continue
		}
		ranges = append(ranges, grammar.CharRange{Lo: r.Lo, Hi: r.Hi})
	}
	d := core.CharClassString(&grammar.CharClass{Ranges: ranges})
	m := b.view.Module
	if b.classes == nil {
		b.exps, b.classes = map[string]core.ExpID{}, map[string]*core.Class{}
		for i, e := range m.Exps {
			b.exps[e] = core.ExpID(i)
		}
		for _, c := range m.Classes {
			b.classes[fmt.Sprint(c.Negated, c.Ranges, m.Exps[c.Exp])] = c
		}
		m.Exps, m.Classes = slices.Clip(m.Exps), slices.Clip(m.Classes)
	}
	c := &core.Class{}
	for _, r := range ranges {
		c.Ranges = append(c.Ranges, core.Range{Lo: r.Lo, Hi: r.Hi})
	}
	key := fmt.Sprint(false, c.Ranges, d)
	if k, ok := b.classes[key]; ok {
		return k
	}
	id, ok := b.exps[d]
	if !ok {
		id = core.ExpID(len(m.Exps))
		b.exps[d] = id
		m.Exps = append(m.Exps, d)
	}
	c.Exp = id
	b.classes[key] = c
	m.Classes = append(m.Classes, c)
	return c
}

// factor applies C6 to runs of alternatives that begin with the same item: p b / p c becomes
// p (b / c), where p is the longest prefix that the alternatives of the run share. The alternatives
// have no cut that could commit the choice, and p evaluated again costs at most what its first
// evaluation cost (evaluatedAgainCheaply), which is what the view drops. A run whose shared prefix
// is not cheap as a whole is left as it is: factoring a part of it would hide from the classifier
// the shared calls that it strips itself.
func (b *builder) factor(alts []core.Node) ([]core.Node, bool) {
	if b.depth > 0 {
		return alts, false
	}
	var out []core.Node
	changed := false
	for i := 0; i < len(alts); {
		first := items(alts[i])
		j, n := i+1, len(first)
		if len(first) > 1 && !facts.LooseCut(alts[i]) {
			for j < len(alts) && !facts.LooseCut(alts[j]) {
				next := items(alts[j])
				if len(next) < 2 || !b.same(first[0], next[0]) {
					break
				}
				m := 1
				for m < min(n, len(next)) && b.same(first[m], next[m]) {
					m++
				}
				n = m
				j++
			}
		}
		prefix := first[:min(n, len(first))]
		if j-i < 2 || !b.evaluatedAgainCheaply(&core.Seq{Items: prefix}) || b.values.observed(b.cur, alts[i:j]...) {
			out = append(out, alts[i])
			i++
			continue
		}
		b.count(N12)
		changed = true
		// Each rest stands for its alternative, and the choice of the rests for the run; the
		// factored sequence stands for the first alternative of the run, since the prefix that it
		// evaluates first is that alternative's. A rest that is a node of the module keeps its own
		// text.
		run := &core.Choice{Alts: make([]*core.Alt, 0, j-i)}
		for _, a := range alts[i:j] {
			r := b.seqOf(a.(*core.Seq), items(a)[len(prefix):], false)
			b.standsFor(r, a)
			run.Alts = append(run.Alts, &core.Alt{Body: r})
		}
		b.standsForText(run, b.textOfAll(alts[i:j], " / "))
		f := &core.Seq{Items: append(slices.Clone(prefix), run)}
		b.standsFor(f, alts[i])
		out = append(out, f)
		i = j
	}
	return out, changed
}

// sameValueBare reports whether n has the same value alone as it has as the only alternative of
// a choice, where a sequence around the choice takes it. The nodes it accepts are one element in
// both places, or a call of a splice rule whose body is not a sequence (isSeq), which is one
// element or none in both. A sequence is not: as an alternative it is one element, and alone it
// adds its elements to the sequence around it. Neither is a call of a splice rule whose body is a
// sequence, which adds the elements of the body in the same way. Where a predicate can read the
// value, a choice of one alternative that this does not accept stays.
func (b *builder) sameValueBare(n core.Node) bool {
	switch n.(type) {
	case *core.Call:
		return !b.isSeq(n)
	case *core.Lit, *core.ClassNode, *core.Any, *core.Repeat, *core.Opt, *core.Text, *core.FailNode:
		return true
	}
	return false
}

// isSeq reports whether n adds the elements of a sequence to the sequence around it: it is a
// sequence, a label of one, or a call of a splice rule whose body in the source module is one. A
// recovery makes its body one element.
func (b *builder) isSeq(n core.Node) bool {
	seen := map[int]bool{}
	for {
		switch x := n.(type) {
		case *core.Seq:
			return true
		case *core.Label:
			n = x.Body
		case *core.Call:
			r := b.src.Module.Rules[x.Rule]
			if !r.Policy.Splice || seen[x.Rule] {
				return false
			}
			seen[x.Rule] = true
			n = r.Body
		default:
			return false
		}
	}
}

// leadingCut reports whether alt is a cut or a sequence that starts with one: the choice commits
// to it before it examines anything.
func leadingCut(alt core.Node) bool {
	if s, ok := alt.(*core.Seq); ok && len(s.Items) > 0 {
		alt = s.Items[0]
	}
	_, ok := alt.(*core.Cut)
	return ok
}

// recover applies V1 to a recovery: a recovery of an expression that always succeeds never runs
// its skip.
func (b *builder) recover(r *core.Recover) core.Node {
	body := b.node(r.Body)
	// A recovery makes an expression with a value one element. Without the recovery a sequence
	// around a sequence takes its elements, so the recovery stays where a predicate can read the
	// value. It stays there also on an expression without a value, which it leaves without one,
	// as the gate does not look at the shape of the value.
	shaped := b.depth > 0 || b.values.observed(b.cur, body)
	if b.on(N6) && !shaped && b.src.Facts.Succeeds(body) {
		b.count(N6)
		return body
	}
	if body == r.Body {
		return r
	}
	c := *r
	c.Body = body
	return b.built(&c, r)
}

// dropCuts removes the cuts of an alternative that cannot change what the choice does: every loose
// cut of the last alternative (all), or the cut that ends an alternative. A loose cut of the last
// alternative commits the choice, which has nothing left to try, wherever it is nested: in a
// sequence, a capture, a discard, an atomic, a label or the expression of a recovery
// (looseCutsDropped). Dropping all of them also keeps a cut from committing an enclosing construct
// when the choice is replaced by its only alternative. A cut that ends an alternative is removed
// only as an item of the alternative itself.
func (b *builder) dropCuts(alt core.Node, all bool) core.Node {
	// A sequence that loses its cuts and keeps one item or none would become that item, which
	// changes its value from a tuple to the item's. Where a predicate can read it, it stays a
	// sequence of its items.
	tuple := b.depth > 0 || b.values.observed(b.cur, alt)
	if _, ok := alt.(*core.Cut); ok || all {
		return b.looseCutsDropped(alt, tuple)
	}
	s, ok := alt.(*core.Seq)
	if !ok || len(s.Items) == 0 {
		return alt
	}
	if _, isCut := s.Items[len(s.Items)-1].(*core.Cut); !isCut {
		return alt
	}
	return b.seqOf(s, s.Items[:len(s.Items)-1], tuple)
}

// seqOf returns the sequence of items in place of the sequence s: the item itself when there is
// one, and a top when there is none. With tuple it returns a sequence of the items, which is a
// tuple of one element (or of none, written as a sequence of a top) and so keeps the value of the
// sequence that it replaces.
func (b *builder) seqOf(s *core.Seq, items []core.Node, tuple bool) core.Node {
	switch {
	case len(items) == 0 && tuple:
		return b.built(&core.Seq{Items: []core.Node{&core.Succeed{}}}, s)
	case len(items) == 0:
		return b.built(&core.Succeed{}, s)
	case len(items) == 1 && !tuple:
		return items[0]
	}
	return b.built(&core.Seq{Items: slices.Clone(items)}, s)
}

// looseCutsDropped returns n without the cuts that are loose in it (facts.LooseCut), or n itself
// when it has none outside the skips of recoveries, which it keeps. A cut that is an item of a
// sequence is removed, and a cut that is the whole of an expression becomes a top. Every node that
// it rebuilds stands for the text of the one it replaces. With tuple, a sequence that keeps fewer
// than two items stays a sequence (seqOf).
func (b *builder) looseCutsDropped(n core.Node, tuple bool) core.Node {
	r := b.looseCutsDroppedIn(n, tuple)
	if r != n {
		b.standsFor(r, n)
	}
	return r
}

func (b *builder) looseCutsDroppedIn(n core.Node, tuple bool) core.Node {
	switch x := n.(type) {
	case *core.Cut:
		return b.built(&core.Succeed{}, x)
	case *core.Seq:
		var items []core.Node
		changed := false
		for _, it := range x.Items {
			if _, isCut := it.(*core.Cut); isCut {
				changed = true
				continue
			}
			r := b.looseCutsDropped(it, tuple)
			changed = changed || r != it
			items = append(items, r)
		}
		if !changed {
			return x
		}
		if len(items) < 2 {
			return b.seqOf(x, items, tuple)
		}
		return b.built(&core.Seq{Items: items}, x)
	case *core.Capture:
		if r := b.looseCutsDropped(x.Body, tuple); r != x.Body {
			c := *x
			c.Body = r
			return b.built(&c, x)
		}
	case *core.Drop:
		if r := b.looseCutsDropped(x.Body, tuple); r != x.Body {
			c := *x
			c.Body = r
			return b.built(&c, x)
		}
	case *core.Text:
		if r := b.looseCutsDropped(x.Body, tuple); r != x.Body {
			c := *x
			c.Body = r
			return b.built(&c, x)
		}
	case *core.Label:
		if r := b.looseCutsDropped(x.Body, tuple); r != x.Body {
			c := *x
			c.Body = r
			return b.built(&c, x)
		}
	case *core.Recover:
		if r := b.looseCutsDropped(x.Body, tuple); r != x.Body {
			c := *x
			c.Body = r
			return b.built(&c, x)
		}
	}
	return n
}
