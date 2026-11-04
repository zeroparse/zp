package compile

import "zeroparse.dev/zp/grammar"

// sepForm is a separated list (a repetition with a #sep) as the classifier reads it. The list
// commits after each separator, so what the parser retries when an iteration fails is only the
// part before the commit point: the separator, or, with trailing="required", the element. The rest
// of the iteration, once reached, either succeeds or fails the whole list, and no continuation
// is tried at the position before it.
type sepForm struct {
	// first is the first iteration, and tail the iteration after one, as the language of the list
	// reads them: the element, and the separator then the element, or with trailing="required"
	// the element then the separator in both.
	first, tail grammar.Expr
	// retried is the part of an iteration that the parser can leave and run again from the start
	// with the continuation, and committed the part after the commit point.
	retried, committed grammar.Expr
	// trail is the trailing separator that trailing="optional" accepts after the last element, or nil.
	trail grammar.Expr
}

// sepFormOf returns the form of the separated list e, which has a separator.
func (k *classifier) sepFormOf(e *grammar.Repeat) *sepForm {
	if f := k.sepForms[e]; f != nil {
		return f
	}
	sep := &grammar.Discard{SourceInfo: e.SourceInfo, Expr: e.Sep}
	f := &sepForm{}
	if e.Trailing == TrailingRequired {
		body := &grammar.Seq{Items: []grammar.Expr{e.Expr, sep}}
		f.first, f.tail, f.retried, f.committed = body, body, e.Expr, sep
	} else {
		f.first, f.tail = e.Expr, &grammar.Seq{Items: []grammar.Expr{sep, e.Expr}}
		f.retried, f.committed = sep, e.Expr
		if e.Trailing == TrailingOptional {
			f.trail = sep
		}
	}
	if k.sepForms == nil {
		k.sepForms = map[*grammar.Repeat]*sepForm{}
	}
	k.sepForms[e] = f
	return f
}

// walkSeparated walks the separated list e. Its unit is an iteration that consists of the part
// before the commit point; the rest of the iteration is walked outside the unit, because its
// failure does not make the parser continue where the iteration started.
func (k *classifier) walkSeparated(e *grammar.Repeat, c wctx) {
	f := k.sepFormOf(e)
	if e.Max < 0 && !c.skipSites {
		k.addSource(c, exprPos(e), "the repetition "+k.show(e))
		k.srcExprs[e] = true
	}
	u := k.newUnit(unitIteration, c, e, []*cont{{items: []grammar.Expr{f.retried}}},
		[]contSpec{{paths: []*cont{c.cont}, what: "what follows the repetition " + k.show(e)}})
	k.leftUnit(u, c, f.first)
	again := k.againOf(e)
	cc := c.with(c.cont.then(again))
	cc.mustStart = false
	cc.inRepeat = true
	if e.Max < 0 || e.Max > 1 {
		cc.repeated = "the repetition " + k.show(e) + " at " + at(c.rule, exprPos(e))
	}
	if e.Trailing == TrailingRequired {
		// The element is retried; the separator after it is committed.
		k.walk(e.Expr, cc.push(u).with(cc.cont.then(e.Sep)))
		k.walk(e.Sep, cc)
		return
	}
	// The separator is retried; the element after it is committed. The same walk of the element
	// serves the first iteration and the later ones.
	k.walk(e.Sep, cc.push(u).with(cc.cont.then(e.Expr)))
	k.walk(e.Expr, cc)
}
