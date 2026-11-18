package parser

import (
	"slices"

	"zeroparse.dev/zp/pkg/grammar/ast"
)

// The raw formatter state of a grammar (the comment tokens and line breaks
// that LineBreak and SourceLayout values keep, see ast.LineBreak) depends
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
	starts, ends map[ast.Pos]int
}

// replayLayout sets the raw formatter state of g, which the canonical
// frontend built from the source of the tokens toks, as Parse sets it.
func replayLayout(toks []token, g *ast.Grammar) {
	r := &layoutReplay{p: &parser{toks: toks, errs: &errorList{}}, starts: make(map[ast.Pos]int, len(toks)), ends: make(map[ast.Pos]int, len(toks))}
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
func (r *layoutReplay) seekStart(pos ast.Pos) {
	if i, ok := r.starts[pos]; ok {
		r.seek(i)
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
func (r *layoutReplay) header(pos ast.Pos, read func()) {
	p := r.p
	header, headerPos := p.header, p.headerPos
	p.header, p.headerPos = true, pos
	read()
	p.header, p.headerPos = header, headerPos
}

func (r *layoutReplay) file(g *ast.Grammar) {
	p := r.p
	if pkg := &g.File.Pkg; len(pkg.Attrs) > 0 || p.is("package") {
		pkg.Break = p.statementBreak()
		pkg.AttrBreaks, pkg.KeywordBreak = r.attributes(pkg.Attrs)
		pos := p.next().pos
		r.header(pos, func() { p.next() })
		r.separator()
	}
	for _, d := range g.File.Decls {
		switch d := d.(type) {
		case *ast.TypeDef:
			r.typeDef(d)
		case *ast.TypeAlias:
			r.typeAlias(d)
		case *ast.VarDef:
			r.varDef(d)
		case *ast.RuleDef:
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

func (r *layoutReplay) typeDef(d *ast.TypeDef) {
	p := r.p
	r.seekStart(d.Pos)
	d.Break = p.statementBreak()
	p.next() // type
	r.header(d.Pos, func() {
		p.next() // the name
		switch t := d.Type.(type) {
		case *ast.StructType:
			p.next() // =
			p.header = false
			t.CloseBreak, t.OneLine = r.structFields(t)
		case *ast.TypeApp:
			p.next() // =
			p.header = false
			r.typ(t)
		}
	})
}

func (r *layoutReplay) typeAlias(d *ast.TypeAlias) {
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

func (r *layoutReplay) varDef(d *ast.VarDef) {
	p := r.p
	r.seekStart(d.Pos)
	d.Break = p.statementBreak()
	p.next() // var
	r.header(d.Pos, func() { p.next() })
	if d.Type != nil {
		r.typ(d.Type)
	}
	if d.Value != nil {
		p.next() // =
		r.term(d.Value)
	}
}

func (r *layoutReplay) ruleDef(d *ast.RuleDef) {
	p := r.p
	if len(d.Attrs) > 0 {
		r.seekStart(d.Attrs[0].Pos)
	} else {
		r.seekStart(d.Pos)
	}
	d.Break = p.statementBreak()
	d.AttrBreaks, d.KeywordBreak = r.attributes(d.Attrs)
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
	} else {
		r.bodyChoice(d.Expr)
	}
	if d.Action != nil || d.DefaultAction {
		if p.tok().lineBefore {
			d.ActionBreak = p.lineBreak()
		} else if !operators {
			p.afterGap(d.Expr)
		}
		p.next() // =>
		r.actionTerm(d.Action, d.DefaultAction)
	}
}

// attributes replays the declaration attributes of a rule or the package
// clause, and returns their line breaks and the line break before the
// keyword that follows them.
func (r *layoutReplay) attributes(attrs []*ast.Attribute) (breaks []*ast.LineBreak, keywordBreak *ast.LineBreak) {
	p := r.p
	if len(attrs) == 0 {
		return nil, nil
	}
	for i, a := range attrs {
		var b *ast.LineBreak
		var gap []*ast.Comment
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
				a.Layout = &ast.SourceLayout{}
			}
			a.Layout.Before = gap
		}
	}
	return breaks, p.lineBreak()
}

func (r *layoutReplay) attribute(a *ast.Attribute) {
	p := r.p
	p.next() // @
	p.next() // the name
	if !p.is("(") || p.tok().spaceBefore {
		return
	}
	p.next()
	defer p.restoreBreak()()
	list := &ast.ListLayout{}
	list.OpenBreak, list.BodyBreak = p.openList()
	a.ArgsLayout = list
	if p.is(")") {
		list.CloseBreak = p.lineBreak()
		p.next()
		return
	}
	values := append([]any(nil), a.Args...)
	for _, k := range a.KeywordOrder {
		values = append(values, a.KwArgs[k])
	}
	for i, v := range values {
		var layout *ast.SourceLayout
		if len(list.Elems) > 0 {
			layout = p.takeTermLayout()
		}
		if i >= len(a.Args) {
			p.next() // the name
			p.next() // =
		}
		r.attributeValue(v)
		list.Elems = append(list.Elems, layout)
		last := &list.Elems[len(list.Elems)-1]
		p.takeAfter(last)
		if p.is(")") {
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

// typ replays the type t, which starts at the current token.
func (r *layoutReplay) typ(t ast.TypeExpr) {
	p := r.p
	switch t := t.(type) {
	case *ast.JoinType:
		for i, u := range t.Types {
			if i > 0 {
				p.next() // |
			}
			r.typ(u)
		}
	case *ast.MeetType:
		for i, u := range t.Types {
			if i > 0 {
				p.next() // &
			}
			r.typ(u)
		}
	case *ast.ListType:
		p.next() // []
		r.typ(t.Elem)
	case *ast.OptionalType:
		p.next() // *
		r.typ(t.Elem)
	case *ast.TypeGroup:
		open := p.i
		p.next() // (
		outer, header := p.cur, p.header
		defer func() { p.cur, p.header = outer, header }()
		p.header = false
		t.OpenBreak = &ast.LineBreak{}
		p.cur = t.OpenBreak
		t.BodyBreak = p.lineBreak()
		r.typ(t.Type)
		t.CloseBreak = p.lineBreak()
		t.OneLine = !p.multiline(open, p.i)
		p.next() // )
	case *ast.TupleType:
		p.next() // (
		outer, header := p.cur, p.header
		defer func() { p.cur, p.header = outer, header }()
		p.header = false
		t.OpenBreak = &ast.LineBreak{}
		p.cur = t.OpenBreak
		t.BodyBreak = p.lineBreak()
		for i, u := range t.Types {
			if i > 0 {
				u.(ast.SourceNode).Source().Layout = p.takeTermLayout()
			}
			r.typ(u)
			p.takeAfterLayout(u)
			if p.is(",") {
				p.next()
				p.afterCommaLayout(&u.(ast.SourceNode).Source().Layout, ")")
				t.TrailingComma = p.is(")")
			}
		}
		t.CloseBreak = p.lineBreak()
		p.next() // )
	case *ast.StructType:
		outer, header := p.cur, p.header
		defer func() { p.cur, p.header = outer, header }()
		p.header = false
		t.CloseBreak, t.OneLine = r.structFields(t)
	case *ast.TypeApp:
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
				u.(ast.SourceNode).Source().Layout = layout
			}
			p.takeAfterLayout(u)
			if p.is(",") {
				p.next()
				p.afterCommaLayout(&u.(ast.SourceNode).Source().Layout, "]")
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
func (r *layoutReplay) structFields(t *ast.StructType) (closeBreak *ast.LineBreak, oneLine bool) {
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
func (r *layoutReplay) operatorsBody(e ast.Expr) {
	p := r.p
	var pragmas []*ast.Attribute
	for {
		a, ok := e.(*ast.Attributed)
		if !ok {
			break
		}
		pragmas = append(a.Attrs, pragmas...)
		e = a.Expr
	}
	pr, ok := e.(*ast.OperatorsRule)
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
			attr.Layout = &ast.SourceLayout{Before: gap}
		}
		p.next() // #
		p.next() // ignorecase
	}
}

// operatorsRule replays the braces and items of pr, from its "{".
func (r *layoutReplay) operatorsRule(pr *ast.OperatorsRule) {
	p := r.p
	type item struct {
		pos  ast.Pos
		read func(b *ast.LineBreak)
	}
	var items []item
	if pr.Skip != nil {
		items = append(items, item{pr.SkipSource.Pos, func(b *ast.LineBreak) {
			pr.SkipBreak = b
			p.next() // skip
			r.choice(pr.Skip)
		}})
	}
	for _, o := range pr.Operands {
		items = append(items, item{o.Pos, func(b *ast.LineBreak) {
			o.Break = b
			p.next() // operand
			r.choice(o.Expr)
			if p.is("=>") {
				p.next()
				r.actionTerm(o.Action, o.DefaultAction)
			}
		}})
	}
	for _, l := range pr.Levels {
		items = append(items, item{l.Pos, func(b *ast.LineBreak) {
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
func (r *layoutReplay) level(l *ast.OperatorsRuleLevel) {
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
		r.choice(op.Expr)
		if p.is("=>") {
			p.next()
			r.actionTerm(op.Action, op.DefaultAction)
		}
		r.separator()
	}
	l.CloseBreak = p.statementBreak()
	l.OneLine = !p.multiline(open, p.i)
	p.next() // }
}
