package syntax

import (
	"fmt"
	"strconv"
	"strings"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/syntax/sourceast"
	"zeroparse.dev/zp/internal/value"
)

// FromSourceTree builds the public grammar AST from the canonical source tree:
// the File node that the grammar zp.zp parses src into. root must come from a
// parse of src that reported no syntax error and use code point positions (the
// default position unit).
//
// The canonical grammar keeps the syntax of the source and the comment groups
// of every gap between tokens. This adapter gives the nodes of the AST their
// meaning: decoded literals, flattened chains, lambda parameters, and the
// checks that the grammar does not make, and it assigns source information
// with the same ownership rules as the hand-written parser. It returns the
// errors of those checks, in the form of Parse, when the source is not a valid
// grammar. The AST holds the syntax and ownership that Format needs of a
// node, but not yet the raw comment tokens that Layout and the line breaks
// keep; formatting an AST built here writes comments by ownership alone.
func FromSourceTree(src string, root *value.Node) (g *grammar.Grammar, err error) {
	c := &canonical{src: []rune(src), pos: sourceast.NewPositions(src)}
	defer func() {
		if x := recover(); x != nil {
			t, ok := x.(treeError)
			if !ok {
				panic(x)
			}
			g, err = nil, t
		}
	}()
	if root == nil || root.Type() != "File" {
		c.fail("the source tree is not a File")
	}
	g = c.file(root)
	if len(c.errs.list) > 0 {
		return nil, c.errs.list
	}
	return g, nil
}

// treeError reports a source tree that is not the tree of a valid parse: a
// recovered syntax error or a node the adapter does not know.
type treeError string

func (e treeError) Error() string { return string(e) }

type canonical struct {
	src  []rune
	pos  *sourceast.Positions
	errs errorList
	// nodes lists the source-bearing nodes of the AST in the order the
	// hand-written parser completes them, and targets their source
	// information; a nil target is a node that the AST does not keep.
	nodes   []sourceast.Node
	targets []*grammar.SourceInfo
	// comments are all the comments of the file in order, and gapList its
	// comment gaps.
	comments []*value.Node
	gapList  []sourceast.Gap
	// params are the parameters of the lambdas that enclose the value being built.
	params []string
}

func (c *canonical) fail(format string, args ...any) {
	panic(treeError(fmt.Sprintf(format, args...)))
}

// Access to the fields of the tree.

func nodeField(n *value.Node, name string) *value.Node {
	child, _ := n.Field(name).(*value.Node)
	return child
}

func listField(n *value.Node, name string) []*value.Node {
	l := nodeField(n, name)
	if l == nil {
		return nil
	}
	out := make([]*value.Node, 0, l.Len())
	for _, e := range l.Elems() {
		out = append(out, e)
	}
	return out
}

func textField(n *value.Node, name string) string { return nodeField(n, name).Text() }

func stringField(n *value.Node, name string) string {
	s, _ := n.Field(name).(string)
	return s
}

func boolField(n *value.Node, name string) bool {
	b, _ := n.Field(name).(bool)
	return b
}

func stringsField(n *value.Node, name string) []string {
	var out []string
	for _, e := range listField(n, name) {
		s, _ := e.Value().(string)
		out = append(out, s)
	}
	return out
}

// Source information.

func isSpaceRune(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\v', '\f', '\r', 0x85, 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

// nextToken returns the offset of the first token at or after off.
func (c *canonical) nextToken(off int) int {
	src := c.src
	for off < len(src) {
		switch {
		case isSpaceRune(src[off]):
			off++
		case src[off] == '/' && off+1 < len(src) && src[off+1] == '/':
			for off < len(src) && src[off] != '\n' && src[off] != '\r' {
				off++
			}
		case src[off] == '/' && off+1 < len(src) && src[off+1] == '*':
			depth := 0
			for off < len(src) {
				if src[off] == '/' && off+1 < len(src) && src[off+1] == '*' {
					depth++
					off += 2
				} else if src[off] == '*' && off+1 < len(src) && src[off+1] == '/' {
					depth--
					off += 2
					if depth == 0 {
						break
					}
				} else {
					off++
				}
			}
		default:
			return off
		}
	}
	return off
}

// separators returns the end of the last of the separator tokens sep that
// follow off, or nothing when the next token is not one. Most is the most it
// takes.
func (c *canonical) separators(off int, sep rune, most int) []int {
	var last []int
	for n := 0; n < most; n++ {
		at := c.nextToken(off)
		if at >= len(c.src) || c.src[at] != sep {
			break
		}
		off = at + 1
		last = []int{off}
	}
	return last
}

// add records a node, completed once, with the span of the events.
func (c *canonical) add(target *grammar.SourceInfo, events ...sourceast.Event) int {
	c.nodes = append(c.nodes, sourceast.Node{Events: events, Created: sourceast.Unset})
	c.targets = append(c.targets, target)
	return len(c.nodes) - 1
}

// span records a node with the span of the tree node n.
func (c *canonical) span(target *grammar.SourceInfo, n *value.Node) int {
	return c.add(target, sourceast.Event{Start: n.Start(), End: n.End()})
}

// created records the position the hand-written parser gave node i before it
// completed it.
func (c *canonical) created(i, off int) int {
	c.nodes[i].Created = off
	return i
}

func (c *canonical) later(i int, at []int) { c.nodes[i].Later = append(c.nodes[i].Later, at...) }

// joined records the semicolons after node i, whose trailing comments follow
// its own.
func (c *canonical) joined(i int, at []int) {
	c.nodes[i].Joined = append(c.nodes[i].Joined, at...)
}

// last returns the index of the node recorded last.
func (c *canonical) last() int { return len(c.nodes) - 1 }

// each builds the elements of a comma-separated list. The outermost node of an
// element claims the trailing group after the comma that follows it.
func (c *canonical) each(n *value.Node, field string, build func(*value.Node)) {
	for _, a := range listField(n, field) {
		build(a)
		c.later(c.last(), c.separators(a.End(), ',', 1))
	}
}

// statement records the node of a statement or list element with its span.
func (c *canonical) statement(target *grammar.SourceInfo, n *value.Node, sep []int, comma bool) {
	i := c.span(target, nodeField(n, "SourceSpan"))
	if comma {
		c.later(i, sep)
		return
	}
	c.joined(i, sep)
}

// errorAt reports a check failure on the tree node n.
func (c *canonical) errorAt(n *value.Node, format string, args ...any) {
	c.errorRange(n.Start(), n.End(), fmt.Sprintf(format, args...))
}

func (c *canonical) errorRange(start, end int, msg string) {
	c.errs.addRange(c.pos.At(start), c.pos.At(end), msg)
}

// File.

func (c *canonical) file(root *value.Node) *grammar.Grammar {
	g := &grammar.Grammar{}
	c.comments = listField(root, "Comments")
	c.gapList = c.gaps(root)
	hasToken := false
	if pkg := nodeField(root, "Pkg"); pkg != nil {
		if pkg.Type() != "Pkg" {
			c.fail("the source tree has a syntax error in the package clause")
		}
		hasToken = true
		c.pkg(&g.File.Pkg, pkg)
	}
	for _, d := range listField(root, "Decls") {
		hasToken = true
		g.File.Decls = append(g.File.Decls, c.decl(d))
	}
	infos, err := c.pos.Claim(c.gapList, c.nodes)
	if err != nil {
		c.fail("source information: %v", err)
	}
	for i, info := range infos {
		if c.targets[i] != nil {
			*c.targets[i] = info
		}
	}
	// The file spans its tokens; with none, it is the end of the input.
	if hasToken {
		g.File.Pos = c.pos.At(c.nextToken(0))
		g.File.End = c.pos.At(c.tokensEnd(root))
	} else {
		g.File.Pos = c.pos.At(len(c.src))
		g.File.End = g.File.Pos
	}
	return g
}

// gaps returns the comment gaps of the tree.
func (c *canonical) gaps(root *value.Node) []sourceast.Gap {
	var gaps []sourceast.Gap
	for _, g := range listField(root, "Gaps") {
		gap := sourceast.Gap{Start: g.Start(), End: g.End()}
		gap.Trailing = stringsField(g, "Trailing")
		gap.HasTrailing = boolField(g, "HasTrailing")
		gap.Leading = stringField(g, "Leading")
		gap.HasNext = boolField(g, "HasNext")
		for _, d := range listField(g, "Detached") {
			gap.Detached = append(gap.Detached, stringField(d, "Text"))
		}
		gaps = append(gaps, gap)
	}
	return gaps
}

// tokensEnd returns the offset where the last token of the source ends.
func (c *canonical) tokensEnd(root *value.Node) int {
	// A gap that reaches the end of the input starts where the last token ends.
	if gaps := listField(root, "Gaps"); len(gaps) > 0 {
		if g := gaps[len(gaps)-1]; g.End() == len(c.src) {
			return g.Start()
		}
	}
	end := len(c.src)
	for end > 0 && isSpaceRune(c.src[end-1]) {
		end--
	}
	return end
}

func (c *canonical) pkg(p *grammar.Pkg, n *value.Node) {
	body := nodeField(nodeField(n, "Source"), "Span")
	p.Name = textField(n, "Name")
	i := c.span(&p.SourceInfo, body)
	c.joined(i, c.separators(body.End(), ';', 1<<30))
	name := nodeField(n, "Name")
	p.HeaderComments = c.commentsIn(body.Start()+len("package"), name.Start())
}

func (c *canonical) decl(d *value.Node) grammar.Decl {
	span := nodeField(d, "SourceSpan")
	var sep []int
	if span != nil {
		sep = c.separators(span.End(), ';', 1<<30)
	}
	switch d.Type() {
	case "TypeDef":
		def := &grammar.TypeDef{Name: textField(d, "Name")}
		ty := nodeField(d, "Type")
		name := nodeField(d, "Name")
		eq := c.nextToken(name.End())
		def.HeaderComments = c.commentsIn(span.Start()+len("type"), name.Start())
		def.HeaderComments = append(def.HeaderComments, c.commentsIn(name.End(), eq)...)
		if ty.Type() == "StructType" {
			// The hand-written parser completes the struct from the "=".
			def.Type = c.structType(ty, eq)
		} else {
			def.Type = c.typ(ty)
			def.HeaderComments = append(def.HeaderComments, c.commentsIn(eq+1, ty.Start())...)
		}
		c.statement(&def.SourceInfo, d, sep, false)
		return def
	case "TypeAlias":
		alias := &grammar.TypeAlias{Name: textField(d, "Name")}
		name := nodeField(d, "Name")
		ty := nodeField(d, "Type")
		tilde := c.nextToken(name.End())
		alias.HeaderComments = c.commentsIn(span.Start()+len("type"), name.Start())
		alias.HeaderComments = append(alias.HeaderComments, c.commentsIn(name.End(), tilde)...)
		alias.HeaderComments = append(alias.HeaderComments, c.commentsIn(tilde+1, ty.Start())...)
		alias.Type = c.typ(ty)
		c.statement(&alias.SourceInfo, d, sep, false)
		return alias
	case "VarDef":
		v := &grammar.VarDef{Name: textField(d, "Name")}
		name := nodeField(d, "Name")
		v.HeaderComments = c.commentsIn(span.Start()+len("var"), name.Start())
		if t := nodeField(d, "Type"); t != nil {
			v.HeaderComments = append(v.HeaderComments, c.commentsIn(name.End(), t.Start())...)
			v.Type = c.typ(t)
		}
		if val := nodeField(d, "Value"); val != nil {
			v.Value = c.action(val)
		}
		c.statement(&v.SourceInfo, d, sep, false)
		return v
	case "RuleDef":
		return c.ruleDef(d, span, sep)
	}
	c.fail("the source tree has a syntax error in a declaration")
	return nil
}

func (c *canonical) ruleDef(d, span *value.Node, sep []int) *grammar.RuleDef {
	rd := &grammar.RuleDef{Name: textField(d, "Name")}
	attrs := listField(d, "Attrs")
	keyword := span.Start()
	for _, attr := range attrs {
		rd.Attrs = append(rd.Attrs, c.attr(attr))
	}
	if len(attrs) > 0 {
		keyword = c.nextToken(attrs[len(attrs)-1].End())
		// No source-bearing node starts at "rule" after attributes, so the
		// declaration owns the comments before it.
		for _, g := range c.gapList {
			if g.End == keyword && g.HasNext {
				rd.KeywordComments = g.Leading
				rd.KeywordDetachedComments = append([]string(nil), g.Detached...)
			}
		}
	}
	name := nodeField(d, "Name")
	ty := nodeField(d, "Type")
	expr := nodeField(d, "Expr")
	// The header holds the comments before the name, the type and the "=".
	rd.HeaderComments = c.commentsIn(keyword+len("rule"), name.Start())
	from := name.End()
	if ty != nil {
		rd.Type = c.typ(ty)
		var gaps [][2]int
		from = c.typeHeader(ty, from, &gaps)
		for _, g := range gaps {
			rd.HeaderComments = append(rd.HeaderComments, c.commentsIn(g[0], g[1])...)
		}
	}
	rd.HeaderComments = append(rd.HeaderComments, c.commentsIn(from, c.nextToken(from))...)
	rd.Expr = c.rule(expr)
	if a := nodeField(d, "Action"); a != nil {
		rd.Action, rd.DefaultAction = c.actionOrDefault(a)
	}
	c.statement(&rd.SourceInfo, d, sep, false)
	if len(attrs) > 0 {
		// The hand-written parser gives a rule with attributes the position of
		// "rule" for diagnostics.
		c.created(c.last(), keyword)
	}
	return rd
}

// Types.

func (c *canonical) structType(n *value.Node, start int) *grammar.StructType {
	st := &grammar.StructType{Fields: nil}
	for _, f := range listField(n, "Fields") {
		if f.Type() != "Field" {
			c.fail("the source tree has a syntax error in a struct field")
		}
		span := nodeField(f, "SourceSpan")
		field := &grammar.Field{Name: textField(f, "Name")}
		name := nodeField(f, "Name")
		ty := nodeField(f, "Type")
		field.HeaderComments = c.commentsIn(name.End(), ty.Start())
		field.Type = c.typ(ty)
		c.statement(&field.SourceInfo, f, c.separators(span.End(), ';', 1), false)
		st.Fields = append(st.Fields, field)
	}
	c.add(&st.SourceInfo, sourceast.Event{Start: start, End: n.End()})
	return st
}

func (c *canonical) types(n *value.Node, field string) []grammar.TypeExpr {
	var out []grammar.TypeExpr
	c.each(n, field, func(a *value.Node) { out = append(out, c.typ(a)) })
	return out
}

func (c *canonical) typ(n *value.Node) grammar.TypeExpr {
	switch n.Type() {
	case "TypeRef":
		t := &grammar.TypeRef{Name: textField(n, "Name")}
		if t.Name == "node" {
			c.errs.add(c.pos.At(nodeField(n, "Name").Start()), bareNode)
		}
		c.span(&t.SourceInfo, n)
		return t
	case "Bottom":
		t := &grammar.TypeRef{Name: n.Text()}
		c.span(&t.SourceInfo, n)
		return t
	case "TypeApp":
		t := &grammar.TypeApp{Name: textField(n, "Name")}
		t.Args = c.types(n, "Args")
		c.span(&t.SourceInfo, n)
		return t
	case "TypeGroup":
		t := &grammar.TypeGroup{}
		t.Type = c.typ(nodeField(n, "Type"))
		c.span(&t.SourceInfo, n)
		return t
	case "TupleType":
		t := &grammar.TupleType{}
		t.Types = c.types(n, "Types")
		c.span(&t.SourceInfo, n)
		return t
	case "StructType":
		return c.structType(n, n.Start())
	case "OptionalType":
		t := &grammar.OptionalType{}
		t.Elem = c.typ(nodeField(n, "Type"))
		c.span(&t.SourceInfo, n)
		return t
	case "ListType":
		t := &grammar.ListType{}
		t.Elem = c.typ(nodeField(n, "Type"))
		c.span(&t.SourceInfo, n)
		return t
	case "JoinType":
		t := &grammar.JoinType{}
		for _, o := range flatten(n, "JoinType") {
			t.Types = append(t.Types, c.typ(o))
		}
		c.span(&t.SourceInfo, n)
		return t
	case "MeetType":
		t := &grammar.MeetType{}
		for _, o := range flatten(n, "MeetType") {
			t.Types = append(t.Types, c.typ(o))
		}
		c.span(&t.SourceInfo, n)
		return t
	}
	c.fail("unexpected type node %s", n.Type())
	return nil
}

// flatten returns the operands of a chain of binary nodes of one type, which
// the public AST holds in one node.
func flatten(n *value.Node, typ string) []*value.Node {
	if n.Type() != typ {
		return []*value.Node{n}
	}
	return append(flatten(nodeField(n, "L"), typ), nodeField(n, "R"))
}

// typeHeader collects the gaps before the tokens of a type in a rule header,
// where the hand-written parser reads the tokens as part of the header: all
// but those inside delimiters and the closing ones. from is the offset where
// the token before the type ends; it returns where the type ends.
func (c *canonical) typeHeader(t *value.Node, from int, gaps *[][2]int) int {
	gap := func(to int) { *gaps = append(*gaps, [2]int{from, to}) }
	switch t.Type() {
	case "ListType":
		gap(t.Start())
		return c.typeHeader(nodeField(t, "Type"), t.Start()+len("[]"), gaps)
	case "OptionalType":
		gap(t.Start())
		return c.typeHeader(nodeField(t, "Type"), t.Start()+len("*"), gaps)
	case "JoinType", "MeetType":
		for i, o := range flatten(t, t.Type()) {
			if i > 0 {
				op := c.nextToken(from)
				gap(op)
				from = op + 1
			}
			from = c.typeHeader(o, from, gaps)
		}
		return from
	case "StructType":
		return t.End()
	}
	// A name, a name with arguments, a group and a tuple have one token before
	// the delimiters of their own.
	gap(t.Start())
	return t.End()
}

// commentsIn returns the contents of the comments between the offsets from and
// to, one string for each.
func (c *canonical) commentsIn(from, to int) []string {
	var out []string
	for _, cm := range c.comments {
		if cm.Start() < from || cm.End() > to {
			continue
		}
		if cm.Type() == "CommentGroup" {
			for _, line := range listField(cm, "Lines") {
				out = append(out, c.commentContent(line.Start(), line.End()))
			}
			continue
		}
		out = append(out, c.commentContent(cm.Start(), cm.End()))
	}
	return out
}

// commentContent returns the documentation text of the comment at the offsets.
func (c *canonical) commentContent(start, end int) string {
	raw := string(c.src[start:end])
	ending := ""
	if strings.HasPrefix(raw, "//") && end < len(c.src) {
		ending = string(c.src[end : end+1])
		if c.src[end] == '\r' && end+1 < len(c.src) && c.src[end+1] == '\n' {
			ending = "\r\n"
		}
	}
	return commentContent(raw, ending)
}

// Literals. The lexer decodes the text of a string or a character class, so a
// literal has the value and the errors it has in the hand-written parser.

// decode runs read, a lexer method, on the source of the tree node n and
// reports the errors it finds at their place in the whole source.
func (c *canonical) decode(n *value.Node, read func(*lexer)) {
	l := &lexer{line: 1, col: 1, errs: &errorList{}, src: c.src[n.Start():n.End()]}
	read(l)
	origin := c.pos.At(n.Start())
	at := func(p grammar.Pos) grammar.Pos {
		if !p.IsValid() {
			return p
		}
		if p.Line == 1 {
			return grammar.Pos{Line: origin.Line, Col: origin.Col + p.Col - 1}
		}
		return grammar.Pos{Line: origin.Line + p.Line - 1, Col: p.Col}
	}
	for _, e := range l.errs.list {
		c.errs.addRange(at(e.Pos), at(e.End), e.Msg)
	}
}

// literal builds the string literal n, with its suffix i.
func (c *canonical) literal(n *value.Node) *grammar.Literal {
	lit := &grammar.Literal{}
	c.decode(n, func(l *lexer) {
		// The suffix i follows the closing quote, so the lexer stops there.
		lit.Value = l.string()
	})
	lit.IgnoreCase = strings.HasSuffix(n.Text(), "i") && !strings.HasSuffix(n.Text(), "\"")
	return lit
}

func (c *canonical) stringValue(n *value.Node) string {
	return c.literal(n).Value
}

// integer decodes the digits of the decimal literal n, which may follow a sign.
func (c *canonical) integer(n *value.Node) int {
	if n == nil {
		c.fail("the source tree has no number where one is required")
	}
	text := n.Text()
	digits := text
	for len(digits) > 0 && digits[len(digits)-1] >= '0' && digits[len(digits)-1] <= '9' {
		digits = digits[:len(digits)-1]
	}
	digits = text[len(digits):]
	negative := strings.HasPrefix(text, "-")
	if negative && digits == minimumIntegerText[1:] && len(digits)+1 == len(text) {
		return minimumIntegerValue
	}
	v, err := strconv.Atoi(digits)
	if err != nil {
		c.errorRange(n.End()-len(digits), n.End(), "invalid number "+digits)
		return 0
	}
	if negative {
		return -v
	}
	return v
}
