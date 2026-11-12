package syntax

import "zeroparse.dev/zp/grammar"

// This file replays the layout inside rule expressions and actions (see
// layoutReplay). Each method mirrors the parser method of the same name and
// starts at the first token of the node it replays.

// bodyChoice replays the top-level choice of a rule body or a group.
func (r *layoutReplay) bodyChoice(e grammar.Expr) {
	p := r.p
	c, ok := e.(*grammar.Choice)
	if !ok {
		r.bodySequence(e)
		return
	}
	r.bodySequence(c.Alts[0])
	breaks := []*grammar.LineBreak{nil}
	broken := false
	for i, alt := range c.Alts[1:] {
		var b *grammar.LineBreak
		if p.atLineStart() {
			b = p.lineBreak()
			broken = true
		} else {
			p.afterGap(c.Alts[i])
			if p.peek(1).lineBefore {
				b = p.lineBreak()
				broken = true
			}
		}
		p.next() // /
		lead := p.takeCommentGap()
		r.bodySequence(alt)
		p.leadOf(alt, lead)
		breaks = append(breaks, b)
	}
	c.Breaks = nil
	if broken {
		c.Breaks = breaks
	}
}

// bodySequence replays a sequence of a rule body or a group, with its line
// breaks.
func (r *layoutReplay) bodySequence(e grammar.Expr) {
	p := r.p
	s, ok := e.(*grammar.Seq)
	if !ok {
		r.prefixed(e)
		return
	}
	breaks := make([]*grammar.LineBreak, 0, len(s.Items))
	broken := false
	for i, item := range s.Items {
		var b *grammar.LineBreak
		if i > 0 && p.atLineStart() {
			b = p.lineBreak()
			broken = true
		}
		r.prefixed(item)
		breaks = append(breaks, b)
	}
	s.Breaks = nil
	if broken {
		s.Breaks = breaks
	}
}

// choice replays a choice inside a pragma, an attribute argument or an item of
// an Operators Rule.
func (r *layoutReplay) choice(e grammar.Expr) {
	p := r.p
	p.nest++
	defer func() { p.nest-- }()
	c, ok := e.(*grammar.Choice)
	if !ok {
		r.sequence(e)
		return
	}
	r.sequence(c.Alts[0])
	for _, alt := range c.Alts[1:] {
		sep := p.takeCommentGap()
		p.next() // /
		lead := p.takeCommentGap()
		r.sequence(alt)
		p.leadOf(alt, lead)
		if sep != nil {
			layoutOf(alt).Sep = sep
		}
	}
}

func (r *layoutReplay) sequence(e grammar.Expr) {
	if s, ok := e.(*grammar.Seq); ok {
		for _, item := range s.Items {
			r.prefixed(item)
		}
		return
	}
	r.prefixed(e)
}

func (r *layoutReplay) prefixed(e grammar.Expr) {
	p := r.p
	gap := p.takeInlineGap()
	switch x := e.(type) {
	case *grammar.Capture:
		p.next() // the label
		p.next() // :
		r.operand(x.Expr)
	case *grammar.And:
		p.next()
		r.operand(x.Expr)
	case *grammar.Not:
		p.next()
		r.operand(x.Expr)
	case *grammar.Discard:
		p.next()
		r.operand(x.Expr)
	default:
		r.suffixed(e)
	}
	if gap != nil {
		layoutOf(e).Before = gap
	}
}

func (r *layoutReplay) operand(e grammar.Expr) {
	p := r.p
	lead := p.takeCommentGap()
	r.prefixed(e)
	p.leadOf(e, lead)
}

// suffixed replays a primary expression and the suffixes, pragmas and
// attributes after it, from the innermost out.
func (r *layoutReplay) suffixed(e grammar.Expr) {
	p := r.p
	var chain []grammar.Expr
	for inner := e; ; {
		var next grammar.Expr
		switch x := inner.(type) {
		case *grammar.Repeat:
			next = x.Expr
		case *grammar.Optional:
			next = x.Expr
		case *grammar.Attributed:
			next = x.Expr
		case *grammar.Atomic:
			next = x.Expr
		case *grammar.Stream:
			next = x.Expr
		case *grammar.Recovery:
			next = x.Expr
		}
		if next == nil {
			r.primary(inner)
			break
		}
		chain = append(chain, inner)
		inner = next
	}
	for i := len(chain) - 1; i >= 0; i-- {
		switch x := chain[i].(type) {
		case *grammar.Attributed:
			r.attributed(x)
		case *grammar.Optional:
			if gap := p.takeCommentGap(); gap != nil {
				layoutOf(x).OpGap = gap
			}
			p.next() // ?
		case *grammar.Repeat:
			if !p.is("#") {
				if gap := p.takeCommentGap(); gap != nil {
					layoutOf(x).OpGap = gap
				}
				p.next() // * or +
				break
			}
			r.pragma(x)
		default:
			r.pragma(x)
		}
	}
}

// pragma replays a pragma that wraps the expression before it, from the
// comments before its "#".
func (r *layoutReplay) pragma(e grammar.Expr) {
	p := r.p
	operator := p.takeInlineGap()
	var br *grammar.LineBreak
	if p.nest == 0 && p.atLineStart() {
		br = p.lineBreak()
	}
	p.next() // #
	p.next() // the name
	switch x := e.(type) {
	case *grammar.Atomic:
		x.Break = br
		r.emptyArguments()
	case *grammar.Stream:
		x.Break = br
		r.emptyArguments()
	case *grammar.Recovery:
		x.Break = br
		p.next() // (
		open := p.takeCommentGap()
		p.next() // skip
		eq := p.takeCommentGap()
		p.next() // =
		lead := p.takeCommentGap()
		r.choice(x.Skip)
		p.leadOf(x.Skip, lead)
		closing := p.takeCommentGap()
		p.next() // )
		if open != nil || eq != nil || closing != nil {
			l := layoutOf(x)
			l.Open, l.Eq, l.Close = open, eq, closing
		}
	case *grammar.Repeat:
		x.PragmaBreak = br
		x.Spelling = r.repeatSpelling()
	}
	if operator != nil {
		layoutOf(e).Operator = operator
	}
}

// emptyArguments consumes the "()" that may follow #atomic or #stream.
func (r *layoutReplay) emptyArguments() {
	p := r.p
	if p.is("(") && !p.tok().spaceBefore {
		p.next()
		p.next()
	}
}

// repeatSpelling replays the arguments of #repeat, from its "(".
func (r *layoutReplay) repeatSpelling() *grammar.RepeatSpelling {
	p := r.p
	p.next() // (
	outer, stray := p.cur, &grammar.LineBreak{}
	p.cur = stray
	sp := &grammar.RepeatSpelling{}
	gap := func() { sp.Gaps = append(sp.Gaps, p.takeGap()) }
	count := func() {
		if p.is("-") {
			p.next()
		}
		p.next()
	}
	gap()
	first := p.tok().kind == tInt || p.is("-")
	if first {
		count()
		sp.HasMin = true
		gap()
	}
	if p.is(",") {
		p.next()
		sp.Comma = true
		gap()
		if !p.is(")") {
			count()
			sp.HasMax = true
			gap()
		}
	}
	p.next() // )
	p.cur = outer
	if len(stray.Comments) > 0 {
		g := &sp.Gaps[len(sp.Gaps)-1]
		g.Comments = append(g.Comments, stray.Comments...)
	}
	return sp
}

// attributed replays the attributes and the #ignorecase and #missing pragmas
// of a, which follow its expression.
func (r *layoutReplay) attributed(a *grammar.Attributed) {
	p := r.p
	a.Breaks = nil
	for i, attr := range a.Attrs {
		var gap []*grammar.Comment
		if attr.Pragma {
			gap = p.takeInlineGap()
		}
		var b *grammar.LineBreak
		if p.nest == 0 && p.atLineStart() {
			b = p.lineBreak()
		}
		if b != nil || a.Breaks != nil {
			for len(a.Breaks) < i {
				a.Breaks = append(a.Breaks, nil)
			}
			a.Breaks = append(a.Breaks, b)
		}
		if attr.Pragma {
			p.next() // #
			p.next() // the name
		} else {
			if b == nil {
				gap = p.takeInlineGap()
			}
			r.attribute(attr)
		}
		if gap != nil {
			attr.Layout = &grammar.SourceLayout{Before: gap}
		}
	}
}

func (r *layoutReplay) primary(e grammar.Expr) {
	p := r.p
	switch x := e.(type) {
	case *grammar.Literal:
		p.next()
		if x.IgnoreCase {
			p.next() // i
		}
	case *grammar.Ref:
		if x.Level != "" {
			for range 6 { // level ( rule , name )
				p.next()
			}
			return
		}
		p.next()
	case *grammar.Cut:
		p.next() // #
		p.next() // cut
	case *grammar.Group:
		r.group(x)
	case *grammar.Predicate:
		x.OpenBreak, x.Breaks, x.CloseBreak, x.OneLine = r.actionBlock(x.Term)
		if b, ok := x.Term.(*grammar.Block); ok {
			b.OpenBreak, b.Breaks, b.CloseBreak, b.OneLine = x.OpenBreak, x.Breaks, x.CloseBreak, x.OneLine
		}
	default:
		p.next() // a class, _, ^^, ^, $$, $, _|_ or .
	}
}

func (r *layoutReplay) group(g *grammar.Group) {
	p := r.p
	open := p.i
	p.next() // (
	lead := p.openLead()
	g.BodyBreak = p.lineBreak()
	p.leadBreak(g.BodyBreak, lead)
	if a, ok := g.Expr.(*grammar.InlineAction); ok {
		r.bodyChoice(a.Expr)
		gap := p.takeCommentGap()
		p.next() // =>
		r.actionTerm(a.Action, a.DefaultAction)
		if gap != nil {
			layoutOf(a).OpGap = gap
		}
	} else {
		r.bodyChoice(g.Expr)
	}
	last := p.toks[p.i].lineComment
	if last != nil {
		last = p.toks[p.i].takeRun(last)
	}
	p.toks[p.i].lineComment = nil
	g.CloseBreak = p.lineBreak()
	g.CloseBreak.Last = last
	g.OneLine = !p.multiline(open, p.i)
	p.next() // )
}

// attributeValue replays an attribute argument v, a value or an expression.
func (r *layoutReplay) attributeValue(v any) {
	p := r.p
	switch v := v.(type) {
	case grammar.Expr:
		r.choice(v)
	case string:
		// A literal, which choice reads as an expression.
		p.nest++
		p.takeInlineGap()
		p.next()
		if p.is("i") && !p.tok().spaceBefore && p.tok().kind == tIdent {
			p.next()
		}
		p.nest--
	default:
		if p.is("-") {
			p.next()
		}
		p.next() // nil, true, false or an integer
	}
}

// actionTerm replays the action of a rule, an inline action or an item of an
// Operators Rule, from its first token.
func (r *layoutReplay) actionTerm(t grammar.Term, defaultAction bool) {
	p := r.p
	b, block := t.(*grammar.Block)
	switch {
	case defaultAction:
		p.next() // ...
	case block:
		b.OpenBreak, b.Breaks, b.CloseBreak, b.OneLine = r.actionBlock(b)
	default:
		r.term(t)
	}
}

// actionBlock replays the braces and statements of a block. t is the block, or
// its one statement, which a predicate holds instead (a statement never starts
// with "{", so a Block is always the block itself).
func (r *layoutReplay) actionBlock(t grammar.Term) (openBreak *grammar.LineBreak, breaks []*grammar.LineBreak, closeBreak *grammar.LineBreak, oneLine bool) {
	p := r.p
	statements := []grammar.Term{t}
	if b, ok := t.(*grammar.Block); ok {
		statements = b.Statements
	}
	open := p.i
	outer := p.cur
	defer func() { p.cur = outer }()
	p.next() // {
	lead := p.openLead()
	openBreak = &grammar.LineBreak{}
	p.cur = openBreak
	joined := false
	for i := 0; ; i++ {
		var gap []*grammar.Comment
		var gapLine int
		if joined && !p.is("}") {
			if gap = p.takeInlineGap(); gap != nil {
				gapLine = p.toks[p.i-1].end.Line
			}
		}
		br := p.lineBreak()
		p.leadBreak(br, lead)
		lead = nil
		if i == len(statements) {
			oneLine = !p.multiline(open, p.i)
			p.next() // }
			return openBreak, breaks, br, oneLine
		}
		term := statements[i]
		if b, ok := term.(*grammar.Bind); ok {
			p.next() // the name
			p.next() // :=
			r.term(b.Value)
		} else {
			r.term(term)
		}
		if gap != nil {
			l := layoutOf(term)
			l.Before, l.PreviousLine, l.NextLine = gap, gapLine, gapLine
		}
		breaks = append(breaks, br)
		if p.is(";") {
			if gap := p.takeInlineGap(); gap != nil {
				layoutOf(term).After = gap
			}
		}
		joined = p.separator()
	}
}

func (r *layoutReplay) term(t grammar.Term) {
	if l, ok := t.(*grammar.Lambda); ok {
		r.lambda(l)
		return
	}
	r.binary(0, t)
}

func (r *layoutReplay) lambda(l *grammar.Lambda) {
	p := r.p
	layout := p.takeTermLayout()
	p.next() // (
	cur := p.cur
	list := &grammar.ListLayout{}
	list.OpenBreak, list.BodyBreak = p.openList()
	for i := range l.Params {
		var elem *grammar.SourceLayout
		if i > 0 {
			elem = p.takeTermLayout()
		}
		p.next() // the parameter
		p.takeAfter(&elem)
		if i+1 < len(l.Params) || l.TrailingComma {
			p.next() // ,
			p.afterCommaLayout(&elem, ")")
		}
		list.Elems = append(list.Elems, elem)
	}
	list.CloseBreak = p.lineBreak()
	p.next() // )
	p.cur = cur
	if gap := p.takeCommentGap(); gap != nil {
		if layout == nil {
			layout = &grammar.SourceLayout{}
		}
		layout.OpGap = gap
	}
	p.next() // =>
	r.term(l.Body)
	l.ParamsLayout = list
	if layout != nil {
		l.Layout = layout
	}
}

// binary replays t as an operand of the operators of level and above.
func (r *layoutReplay) binary(level int, t grammar.Term) {
	p := r.p
	if level == len(binaryLevels) {
		r.unary(t)
		return
	}
	var chain []*grammar.Binary
	left := t
	for {
		b, ok := left.(*grammar.Binary)
		if !ok || !contains(binaryLevels[level], b.Op) {
			break
		}
		chain = append(chain, b)
		left = b.L
	}
	r.binary(level+1, left)
	for i := len(chain) - 1; i >= 0; i-- {
		b := chain[i]
		operator := p.takeCommentGap()
		p.next() // the operator
		r.binary(level+1, b.R)
		if operator != nil {
			layoutOf(b).OpGap = operator
		}
	}
}

func (r *layoutReplay) unary(t grammar.Term) {
	p := r.p
	layout := p.takeTermLayout()
	if u, ok := t.(*grammar.Unary); ok {
		p.next() // - or !
		r.unary(u.X)
	} else {
		var chain []grammar.Term
		inner := t
		for {
			var x grammar.Term
			switch n := inner.(type) {
			case *grammar.Binary:
				if n.Op == "[]" {
					x = n.L
				}
			case *grammar.Cast:
				x = n.X
			case *grammar.Member:
				x = n.X
			}
			if x == nil {
				break
			}
			chain = append(chain, inner)
			inner = x
		}
		r.termPrimary(inner)
		for i := len(chain) - 1; i >= 0; i-- {
			p.next() // . or [
			switch n := chain[i].(type) {
			case *grammar.Binary:
				outer := p.cur
				n.OpenBreak = &grammar.LineBreak{}
				p.cur = n.OpenBreak
				n.BodyBreak = p.lineBreak()
				r.term(n.R)
				n.CloseBreak = p.lineBreak()
				p.next() // ]
				p.cur = outer
			case *grammar.Cast:
				p.next() // (
				r.typ(n.Type)
				p.next() // )
			case *grammar.Member:
				p.next() // the name
			}
		}
	}
	if layout != nil {
		s := t.(grammar.SourceNode).Source()
		if old := s.Layout; old != nil {
			old.Before, old.PreviousLine, old.NextLine = layout.Before, layout.PreviousLine, layout.NextLine
			layout = old
		}
		s.Layout = layout
	}
}

func (r *layoutReplay) termPrimary(t grammar.Term) {
	p := r.p
	switch x := t.(type) {
	case *grammar.New:
		r.newTerm(x)
	case *grammar.Call:
		p.next() // the name
		p.next() // (
		defer p.restoreBreak()()
		x.OpenBreak, x.BodyBreak = p.openList()
		x.TrailingComma = r.listItems(x.Args, ")")
		x.CloseBreak = p.lineBreak()
		p.next() // )
	case *grammar.ListConstructor:
		p.next() // []
		r.typ(x.Elem)
		p.next() // {
		defer p.restoreBreak()()
		x.OpenBreak, x.BodyBreak = p.openList()
		x.TrailingComma = r.listItems(x.Items, "}")
		x.CloseBreak = p.lineBreak()
		p.next() // }
	case *grammar.TermGroup:
		open := p.i
		p.next() // (
		outer := p.cur
		defer func() { p.cur = outer }()
		x.OpenBreak = &grammar.LineBreak{}
		p.cur = x.OpenBreak
		x.BodyBreak = p.lineBreak()
		r.term(x.Term)
		x.CloseBreak = p.lineBreak()
		x.OneLine = !p.multiline(open, p.i)
		p.next() // )
	case *grammar.TupleLit:
		p.next() // (
		outer := p.cur
		defer func() { p.cur = outer }()
		x.OpenBreak = &grammar.LineBreak{}
		p.cur = x.OpenBreak
		x.BodyBreak = p.lineBreak()
		r.tupleLit(x)
	default:
		p.next() // a literal, a name or a capture
	}
}

// tupleLit replays the elements and closing parenthesis of t.
func (r *layoutReplay) tupleLit(t *grammar.TupleLit) {
	p := r.p
	t.TrailingComma = false
	if len(t.Elems) > 0 {
		r.term(t.Elems[0])
		p.takeAfterLayout(t.Elems[0])
		for i := 1; p.is(","); i++ {
			p.next()
			p.afterCommaLayout(&t.Elems[i-1].(grammar.SourceNode).Source().Layout, ")")
			if p.is(")") {
				t.TrailingComma = true
				break
			}
			r.term(t.Elems[i])
			p.takeAfterLayout(t.Elems[i])
		}
	}
	t.CloseBreak = p.lineBreak()
	p.next() // )
}

func (r *layoutReplay) newTerm(n *grammar.New) {
	p := r.p
	p.next() // the type
	p.next() // {
	defer p.restoreBreak()()
	n.OpenBreak, n.BodyBreak = p.openList()
	trailing := false
	if !p.is("}") && !(p.tok().kind == tIdent && p.peek(1).text == ":") {
		trailing = r.listItems(n.Args, "}")
	} else {
		for _, field := range n.Fields {
			layout := p.takeTermLayout()
			p.next() // the name
			p.next() // :
			r.term(field.Value)
			field.Layout = layout
			p.takeAfterLayout(field)
			trailing = false
			if !p.is("}") {
				p.next() // ,
				p.afterCommaLayout(&field.Layout, "}")
				trailing = p.is("}")
			}
		}
	}
	n.CloseBreak, n.TrailingComma = p.lineBreak(), trailing
	p.next() // }
}

// listItems replays the comma-separated items up to closer, and reports
// whether a trailing comma follows them.
func (r *layoutReplay) listItems(items []grammar.Term, closer string) (trailing bool) {
	p := r.p
	for _, item := range items {
		r.term(item)
		p.takeAfterLayout(item)
		trailing = false
		if !p.is(closer) {
			p.next() // ,
			p.afterCommaLayout(&item.(grammar.SourceNode).Source().Layout, closer)
			trailing = p.is(closer)
		}
	}
	return trailing
}
