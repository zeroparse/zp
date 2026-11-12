package complexity

import (
	"maps"
	"slices"

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
	if _, ok := b.view.Text[x]; !ok {
		b.view.Text[x] = b.view.textOf(n)
	}
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
		if body := b.node(x.Body); body != x.Body {
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
		if body := b.node(x.Body); body != x.Body {
			c := *x
			c.Body = body
			return b.built(&c, x)
		}
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

// seq applies N1 (splice) and N4 (leading bottom) to a sequence.
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

// choice applies N1 (flatten), N2, N4 and N6 (cuts) to a choice.
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
	// C3 and T2 drop alternatives that never give the value of the choice, but they can drop
	// every alternative with a value and keep one without: the choice that gave nil gives no
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
		kept = append(kept, alt)
		if !drops {
			continue
		}
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
