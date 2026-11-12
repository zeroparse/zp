package syntax

import (
	"slices"

	"zeroparse.dev/zp/grammar"
)

// The raw formatter state of a grammar (the comment tokens and line breaks
// that LineBreak and SourceLayout values keep, see grammar.LineBreak) depends
// on where the hand-written parser records them: at which token it starts a
// line break, takes the comments of a gap for a node, or leaves them to the
// line break before. Every comment of the source goes to exactly one of them.
//
// layoutReplay rebuilds that state for a grammar that the canonical frontend
// built: it reads the tokens of the source with the lexer and walks the
// grammar in source order, calling the parser's own primitives at the token
// where the parser calls them. The grammar decides each branch the parser
// takes by looking ahead, so the replay needs no lookahead of its own; it
// only finds the tokens the nodes start and end at.
type layoutReplay struct {
	p *parser
	// starts and ends give the index of the token that starts or ends at a
	// position.
	starts, ends map[grammar.Pos]int
}

// replayLayout sets the raw formatter state of g, which the canonical
// frontend built from src, as Parse sets it.
func replayLayout(src string, g *grammar.Grammar) {
	r := &layoutReplay{p: &parser{toks: lex(src, &errorList{}), errs: &errorList{}}, starts: map[grammar.Pos]int{}, ends: map[grammar.Pos]int{}}
	for i, t := range r.p.toks {
		r.starts[t.pos] = i
		r.ends[t.end] = i
	}
	r.file(g)
}

// seek consumes the tokens before token i, as the parser does with next.
func (r *layoutReplay) seek(i int) {
	for r.p.i < i && r.p.tok().kind != tEOF {
		r.p.next()
	}
}

// seekStart consumes the tokens before the token at pos.
func (r *layoutReplay) seekStart(pos grammar.Pos) {
	if i, ok := r.starts[pos]; ok {
		r.seek(i)
	}
}

// seekPast consumes the tokens up to the one that ends at end.
func (r *layoutReplay) seekPast(end grammar.Pos) {
	if i, ok := r.ends[end]; ok {
		r.seek(i + 1)
	}
}

// seekText consumes the tokens before the next one that is the punctuation
// or name text, outside the brackets that open after the current token.
func (r *layoutReplay) seekText(text string) {
	p := r.p
	depth := 0
	for p.tok().kind != tEOF {
		t := p.tok()
		if depth == 0 && (t.kind == tPunct || t.kind == tIdent) && t.text == text {
			return
		}
		if t.kind == tPunct {
			switch t.text {
			case "(", "[", "{", "[]":
				if t.text != "[]" {
					depth++
				}
			case ")", "]", "}":
				depth--
			}
		}
		p.next()
	}
}

// header runs read with the parser in a statement header at the token at pos:
// the comments it consumes are header comments.
func (r *layoutReplay) header(pos grammar.Pos, read func()) {
	p := r.p
	header, headerPos := p.header, p.headerPos
	p.header, p.headerPos = true, pos
	read()
	p.header, p.headerPos = header, headerPos
}

func (r *layoutReplay) file(g *grammar.Grammar) {
	p := r.p
	if p.is("package") {
		g.File.Pkg.Break = p.statementBreak()
		pos := p.next().pos
		r.header(pos, func() { p.next() })
		r.separator()
	}
	for _, d := range g.File.Decls {
		switch d := d.(type) {
		case *grammar.TypeDef:
			r.typeDef(d)
		case *grammar.TypeAlias:
			r.typeAlias(d)
		case *grammar.VarDef:
			r.varDef(d)
		case *grammar.RuleDef:
			r.ruleDef(d)
		}
		r.separator()
	}
	r.seek(len(p.toks) - 1)
	g.File.EndBreak = p.lineBreak()
	g.File.HeaderComments = p.headerComments
}

// separator consumes the semicolon that ends a statement on its line.
func (r *layoutReplay) separator() {
	if !r.p.tok().lineBefore {
		r.p.accept(";")
	}
}

func (r *layoutReplay) typeDef(d *grammar.TypeDef) {
	p := r.p
	r.seekStart(d.Pos)
	d.Break = p.statementBreak()
	p.next() // type
	r.header(d.Pos, func() {
		p.next() // the name
		switch t := d.Type.(type) {
		case *grammar.StructType:
			p.next() // =
			p.header = false
			t.CloseBreak, t.OneLine = r.structFields(t)
		case *grammar.TypeApp:
			p.next() // =
			p.header = false
			r.typ(t)
		}
	})
}

func (r *layoutReplay) typeAlias(d *grammar.TypeAlias) {
	p := r.p
	r.seekStart(d.Pos)
	d.Break = p.statementBreak()
	p.next() // type
	r.header(d.Pos, func() {
		p.next() // the name
		p.next() // ~
		p.header = false
		r.typ(d.Type)
	})
}

func (r *layoutReplay) varDef(d *grammar.VarDef) {
	p := r.p
	r.seekStart(d.Pos)
	d.Break = p.statementBreak()
	p.next() // var
	r.header(d.Pos, func() { p.next() })
	if d.Type != nil {
		r.typ(d.Type)
	}
	if d.Value != nil {
		r.seekPast(d.Value.(grammar.SourceNode).Source().End)
	}
}

func (r *layoutReplay) ruleDef(d *grammar.RuleDef) {
	p := r.p
	if len(d.Attrs) > 0 {
		r.seekStart(d.Attrs[0].Pos)
	} else {
		r.seekStart(d.Pos)
	}
	d.Break = p.statementBreak()
	r.attributes(d)
	pos := p.next().pos // rule
	r.header(pos, func() {
		p.next() // the name
		if d.Type != nil {
			r.typ(d.Type)
		}
		r.seekText("=")
		p.next()
	})
	// An Operators Rule body starts on the line of "=", and takes no line
	// break before it or its action.
	operators := !p.tok().lineBefore && p.is("#") && p.peek(1).kind == tIdent && p.peek(1).text == "operators"
	if !operators && p.tok().lineBefore {
		d.BodyBreak = p.lineBreak()
	}
	if operators {
		r.operatorsBody(d.Expr)
	}
	r.seekPast(d.Expr.(grammar.SourceNode).Source().End)
	if d.Action != nil || d.DefaultAction {
		if p.tok().lineBefore {
			d.ActionBreak = p.lineBreak()
		} else if !operators {
			p.afterGap(d.Expr)
		}
		p.next() // =>
		if d.Action != nil {
			r.seekPast(d.Action.(grammar.SourceNode).Source().End)
		} else {
			p.next() // ...
		}
	}
}

// attributes replays the declaration attributes of d, and the line break
// before its rule keyword.
func (r *layoutReplay) attributes(d *grammar.RuleDef) {
	p := r.p
	if len(d.Attrs) == 0 {
		return
	}
	var breaks []*grammar.LineBreak
	for i, a := range d.Attrs {
		var b *grammar.LineBreak
		var gap []*grammar.Comment
		if i > 0 {
			if p.atLineStart() {
				b = p.lineBreak()
			} else {
				gap = p.takeInlineGap()
			}
		}
		if b != nil || breaks != nil {
			for len(breaks) < i {
				breaks = append(breaks, nil)
			}
			breaks = append(breaks, b)
		}
		r.attribute(a)
		if gap != nil {
			if a.Layout == nil {
				a.Layout = &grammar.SourceLayout{}
			}
			a.Layout.Before = gap
		}
	}
	d.AttrBreaks = breaks
	d.KeywordBreak = p.lineBreak()
}

func (r *layoutReplay) attribute(a *grammar.Attribute) {
	p := r.p
	p.next() // @
	p.next() // the name
	if !p.is("(") || p.tok().spaceBefore {
		return
	}
	p.next()
	defer p.restoreBreak()()
	list := &grammar.ListLayout{}
	list.OpenBreak, list.BodyBreak = p.openList()
	a.ArgsLayout = list
	if p.is(")") {
		list.CloseBreak = p.lineBreak()
		p.next()
		return
	}
	for {
		var layout *grammar.SourceLayout
		if len(list.Elems) > 0 {
			layout = p.takeTermLayout()
		}
		r.seekArgumentEnd()
		list.Elems = append(list.Elems, layout)
		last := &list.Elems[len(list.Elems)-1]
		p.takeAfter(last)
		if p.is(")") || p.tok().kind == tEOF {
			break
		}
		p.next() // ,
		p.afterCommaLayout(last, ")")
		if p.is(")") {
			a.TrailingComma = true
			break
		}
	}
	list.CloseBreak = p.lineBreak()
	p.next() // )
}

// seekArgumentEnd consumes an attribute argument, up to the comma or closing
// parenthesis after it.
func (r *layoutReplay) seekArgumentEnd() {
	p := r.p
	depth := 0
	for p.tok().kind != tEOF {
		t := p.tok()
		if t.kind == tPunct {
			switch t.text {
			case "(", "[", "{":
				depth++
			case ")", "]", "}":
				if depth == 0 {
					return
				}
				depth--
			case ",":
				if depth == 0 {
					return
				}
			}
		}
		p.next()
	}
}

// typ replays the type t, which starts at the current token.
func (r *layoutReplay) typ(t grammar.TypeExpr) {
	p := r.p
	switch t := t.(type) {
	case *grammar.JoinType:
		for i, u := range t.Types {
			if i > 0 {
				p.next() // |
			}
			r.typ(u)
		}
	case *grammar.MeetType:
		for i, u := range t.Types {
			if i > 0 {
				p.next() // &
			}
			r.typ(u)
		}
	case *grammar.ListType:
		p.next() // []
		r.typ(t.Elem)
	case *grammar.OptionalType:
		p.next() // *
		r.typ(t.Elem)
	case *grammar.TypeGroup:
		open := p.i
		p.next() // (
		outer, header := p.cur, p.header
		defer func() { p.cur, p.header = outer, header }()
		p.header = false
		t.OpenBreak = &grammar.LineBreak{}
		p.cur = t.OpenBreak
		t.BodyBreak = p.lineBreak()
		r.typ(t.Type)
		t.CloseBreak = p.lineBreak()
		t.OneLine = !p.multiline(open, p.i)
		p.next() // )
	case *grammar.TupleType:
		p.next() // (
		outer, header := p.cur, p.header
		defer func() { p.cur, p.header = outer, header }()
		p.header = false
		t.OpenBreak = &grammar.LineBreak{}
		p.cur = t.OpenBreak
		t.BodyBreak = p.lineBreak()
		for i, u := range t.Types {
			if i > 0 {
				u.(grammar.SourceNode).Source().Layout = p.takeTermLayout()
			}
			r.typ(u)
			p.takeAfterLayout(u)
			if p.is(",") {
				p.next()
				p.afterCommaLayout(&u.(grammar.SourceNode).Source().Layout, ")")
				t.TrailingComma = p.is(")")
			}
		}
		t.CloseBreak = p.lineBreak()
		p.next() // )
	case *grammar.StructType:
		outer, header := p.cur, p.header
		defer func() { p.cur, p.header = outer, header }()
		p.header = false
		t.CloseBreak, t.OneLine = r.structFields(t)
	case *grammar.TypeApp:
		p.next() // the name
		p.next() // [
		outer, header := p.cur, p.header
		defer func() { p.cur, p.header = outer, header }()
		p.header = false
		t.OpenBreak, t.BodyBreak = p.openList()
		for _, u := range t.Args {
			layout := p.takeTermLayout()
			r.typ(u)
			if layout != nil {
				u.(grammar.SourceNode).Source().Layout = layout
			}
			p.takeAfterLayout(u)
			if p.is(",") {
				p.next()
				p.afterCommaLayout(&u.(grammar.SourceNode).Source().Layout, "]")
				t.TrailingComma = p.is("]")
			}
		}
		t.CloseBreak = p.lineBreak()
		p.next() // ]
	default:
		p.next() // a name or _|_
	}
}

// structFields replays the braces and fields of the struct type t.
func (r *layoutReplay) structFields(t *grammar.StructType) (closeBreak *grammar.LineBreak, oneLine bool) {
	p := r.p
	open := p.i
	p.next() // {
	lead := p.openLead()
	for _, f := range t.Fields {
		f.Break = p.statementBreak()
		p.leadBreak(f.Break, lead)
		lead = nil
		p.next() // the name
		p.takeGapContents()
		r.typ(f.Type)
		r.separator()
	}
	closeBreak = p.statementBreak()
	p.leadBreak(closeBreak, lead)
	oneLine = !p.multiline(open, p.i)
	p.next() // }
	return closeBreak, oneLine
}

// operatorsBody replays the Operators Rule that e is, or that e holds with the
// #ignorecase pragmas after it, from its #operators.
func (r *layoutReplay) operatorsBody(e grammar.Expr) {
	p := r.p
	var pragmas []*grammar.Attribute
	for {
		a, ok := e.(*grammar.Attributed)
		if !ok {
			break
		}
		pragmas = append(a.Attrs, pragmas...)
		e = a.Expr
	}
	pr, ok := e.(*grammar.OperatorsRule)
	if !ok {
		return
	}
	p.next() // #
	p.next() // operators
	r.operatorsRule(pr)
	for _, attr := range pragmas {
		if !p.is("#") || p.tok().lineBefore {
			break
		}
		if gap := p.takeInlineGap(); gap != nil {
			attr.Layout = &grammar.SourceLayout{Before: gap}
		}
		p.next() // #
		p.next() // ignorecase
	}
}

// operatorsRule replays the braces and items of pr, from its "{".
func (r *layoutReplay) operatorsRule(pr *grammar.OperatorsRule) {
	p := r.p
	type item struct {
		pos  grammar.Pos
		read func(b *grammar.LineBreak)
	}
	var items []item
	if pr.Skip != nil {
		items = append(items, item{pr.SkipSource.Pos, func(b *grammar.LineBreak) {
			pr.SkipBreak = b
			r.seekPast(pr.Skip.(grammar.SourceNode).Source().End)
		}})
	}
	for _, o := range pr.Operands {
		items = append(items, item{o.Pos, func(b *grammar.LineBreak) {
			o.Break = b
			r.seekPast(o.Source().End)
		}})
	}
	for _, l := range pr.Levels {
		items = append(items, item{l.Pos, func(b *grammar.LineBreak) {
			l.Break = b
			r.level(l)
		}})
	}
	slices.SortFunc(items, func(a, b item) int {
		if a.pos.Line != b.pos.Line {
			return a.pos.Line - b.pos.Line
		}
		return a.pos.Col - b.pos.Col
	})
	p.next() // {
	p.leadAfterOpen()
	for _, it := range items {
		b := p.statementBreak()
		it.read(b)
		r.separator()
	}
	pr.CloseBreak = p.statementBreak()
	p.next() // }
}

// level replays a level of an Operators Rule, from its keyword.
func (r *layoutReplay) level(l *grammar.OperatorsRuleLevel) {
	p := r.p
	pos := p.next().pos // level
	open := 0
	r.header(pos, func() {
		if p.tok().kind == tIdent {
			p.next()
		}
		open = p.i
		p.next() // {
	})
	p.leadAfterOpen()
	for _, op := range l.Operators {
		op.Break = p.statementBreak()
		start := p.next() // prefix, postfix or infix
		if start.text == "infix" {
			r.header(start.pos, func() {
				if p.is("left") || p.is("right") || p.is("none") {
					p.next()
				}
			})
		}
		r.seekPast(op.Source().End)
		r.separator()
	}
	l.CloseBreak = p.statementBreak()
	l.OneLine = !p.multiline(open, p.i)
	p.next() // }
}
