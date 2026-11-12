package syntax

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/syntax/genparser/types"
	"zeroparse.dev/zp/internal/syntax/sourceast"
)

// FromSourceTree builds the public grammar AST from the canonical source tree:
// the File that the grammar zp.zp parses src into, as the typed values of its
// generated parser. root must come from a parse of src that reported no syntax
// error and use code point positions (the default position unit).
//
// The canonical grammar keeps the syntax of the source and the comment groups
// of every gap between tokens. This adapter gives the nodes of the AST their
// meaning: decoded literals, flattened chains, lambda parameters, and the
// checks that the grammar does not make, and it assigns source information
// with the same ownership rules as the hand-written parser. It returns the
// errors of those checks, in the form of Parse, when the source is not a valid
// grammar. The raw comment tokens and line breaks that Format reads are
// replayed from the tokens of src (see layoutReplay).
func FromSourceTree(src string, root *types.File) (g *grammar.Grammar, err error) {
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
	if root == nil {
		c.fail("the source tree is not a File")
	}
	g = c.file(root)
	if len(c.errs.list) > 0 {
		return nil, c.errs.list
	}
	replayLayout(src, g)
	return g, nil
}

// FromPartialSourceTree builds the grammar of a source with syntax errors from
// the tree that the generated parser recovers from it, as ParsePartial
// returns it: the declarations that are complete. A declaration that holds an
// error (a recovered span, or a missing part) is left out, as is a package
// clause with an error, so a definition with an error in the middle is
// missing. The errors returned are those of the checks that the grammar does
// not make, in the declarations kept; the syntax errors are the caller's, in
// the form of SourceErrors. root must come from a parse of src with the
// default position unit.
func FromPartialSourceTree(src string, root *types.File) (*grammar.Grammar, ErrorList) {
	c := &canonical{src: []rune(src), pos: sourceast.NewPositions(src), partial: true}
	g := &grammar.Grammar{}
	if root == nil {
		return g, nil
	}
	c.comments = root.Comments
	c.gapList = c.gaps(root)
	if root.Pkg != nil {
		if pkg, ok := root.Pkg.(*types.Pkg); ok && pkg != nil {
			c.try(func() { c.pkg(&g.File.Pkg, pkg) })
		}
	}
	for _, d := range root.Decls {
		var out grammar.Decl
		if c.try(func() { out = c.decl(d) }) {
			g.File.Decls = append(g.File.Decls, out)
		}
	}
	if infos, err := c.pos.Claim(c.gapList, c.nodes); err == nil {
		for i, info := range infos {
			if c.targets[i] != nil {
				*c.targets[i] = info
			}
		}
	}
	if len(g.File.Decls) > 0 || g.File.Pkg.Name != "" {
		g.File.Pos = c.pos.At(c.nextToken(0))
		g.File.End = c.pos.At(c.tokensEnd(root))
	} else {
		g.File.Pos = c.pos.At(len(c.src))
		g.File.End = g.File.Pos
	}
	replayLayout(src, g)
	return g, c.errs.list
}

// try runs build, which adds a node of the tree, and reports whether it
// completed. When it fails on an error of the tree, it takes back the nodes,
// parameters and errors that it added, so that the node is left out whole.
func (c *canonical) try(build func()) (ok bool) {
	nodes, targets, params, errs := len(c.nodes), len(c.targets), c.params, len(c.errs.list)
	defer func() {
		if x := recover(); x != nil {
			if _, isTree := x.(treeError); !isTree {
				panic(x)
			}
			c.nodes, c.targets, c.params = c.nodes[:nodes], c.targets[:targets], params
			c.errs.list = c.errs.list[:errs]
			ok = false
		}
	}()
	build()
	return true
}

// treeError reports a source tree that is not the tree of a valid parse: a
// recovered syntax error or a value the adapter does not know.
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
	comments []types.Comment
	gapList  []sourceast.Gap
	// params are the parameters of the lambdas that enclose the value being built.
	params []string
	// partial is set when the tree has recovered syntax errors: a nil gap is skipped.
	partial bool
}

func (c *canonical) fail(format string, args ...any) {
	panic(treeError(fmt.Sprintf(format, args...)))
}

// Access to the values of the tree.

// ranged is a value of the tree: every value has the range of source it covers.
type ranged interface{ Range() (start, end int) }

func start(x ranged) int { s, _ := x.Range(); return s }

func end(x ranged) int { _, e := x.Range(); return e }

// text returns the source that x covers.
func (c *canonical) text(x ranged) string {
	s, e := x.Range()
	return string(c.src[s:e])
}

// name returns the text of an identifier, or fails on a missing one.
func (c *canonical) name(id *types.Ident) string {
	if id == nil {
		c.fail("the source tree has no name where one is required")
	}
	return id.Value
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

// span records a node with the span of the tree value n.
func (c *canonical) span(target *grammar.SourceInfo, n ranged) int {
	s, e := n.Range()
	return c.add(target, sourceast.Event{Start: s, End: e})
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
func each[T any](c *canonical, list []T, build func(T)) {
	for _, a := range list {
		build(a)
		r, ok := any(a).(ranged)
		if !ok || isNil(a) {
			c.fail("the source tree has a syntax error in a list")
		}
		c.later(c.last(), c.separators(end(r), ',', 1))
	}
}

// isNil reports whether x, a value of the tree, is missing: a nil interface or
// a nil pointer, which the typed tree holds where recovery left an error in a
// place of a struct type.
func isNil(x any) bool {
	if x == nil {
		return true
	}
	v := reflect.ValueOf(x)
	return v.Kind() == reflect.Pointer && v.IsNil()
}

// statement records the node of a statement or list element with span, the
// body of the statement without its comments.
func (c *canonical) statement(target *grammar.SourceInfo, span ranged, sep []int, comma bool) {
	if isNil(span) {
		c.fail("the source tree has a syntax error in a statement")
	}
	i := c.span(target, span)
	if comma {
		c.later(i, sep)
		return
	}
	c.joined(i, sep)
}

// errorAt reports a check failure on the tree value n.
func (c *canonical) errorAt(n ranged, format string, args ...any) {
	s, e := n.Range()
	c.errorRange(s, e, fmt.Sprintf(format, args...))
}

func (c *canonical) errorRange(start, end int, msg string) {
	c.errs.addRange(c.pos.At(start), c.pos.At(end), msg)
}

// File.

func (c *canonical) file(root *types.File) *grammar.Grammar {
	g := &grammar.Grammar{}
	c.comments = root.Comments
	c.gapList = c.gaps(root)
	hasToken := false
	if root.Pkg != nil {
		pkg, ok := root.Pkg.(*types.Pkg)
		if !ok || pkg == nil {
			c.fail("the source tree has a syntax error in the package clause")
		}
		hasToken = true
		c.pkg(&g.File.Pkg, pkg)
	}
	for _, d := range root.Decls {
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
func (c *canonical) gaps(root *types.File) []sourceast.Gap {
	gaps := make([]sourceast.Gap, 0, len(root.Gaps))
	for _, g := range root.Gaps {
		if g == nil {
			if c.partial {
				continue
			}
			c.fail("the source tree has a syntax error in a comment gap")
		}
		gap := sourceast.Gap{Start: g.Start, End: g.End}
		if len(g.Trailing) > 0 {
			gap.Trailing = g.Trailing
		}
		gap.HasTrailing = g.HasTrailing
		gap.Leading = g.Leading
		gap.HasNext = g.HasNext
		for _, d := range g.Detached {
			gap.Detached = append(gap.Detached, d.Text)
		}
		gaps = append(gaps, gap)
	}
	if len(gaps) == 0 {
		return nil
	}
	return gaps
}

// tokensEnd returns the offset where the last token of the source ends.
func (c *canonical) tokensEnd(root *types.File) int {
	// A gap that reaches the end of the input starts where the last token ends.
	if gaps := root.Gaps; len(gaps) > 0 {
		if g := gaps[len(gaps)-1]; g.End == len(c.src) {
			return g.Start
		}
	}
	end := len(c.src)
	for end > 0 && isSpaceRune(c.src[end-1]) {
		end--
	}
	return end
}

func (c *canonical) pkg(p *grammar.Pkg, n *types.Pkg) {
	if n.Source == nil || isNil(n.Source.Span_) {
		c.fail("the source tree has a syntax error in the package clause")
	}
	body := n.Source.Span_.(ranged)
	p.Name = c.name(n.Name)
	i := c.span(&p.SourceInfo, body)
	c.joined(i, c.separators(end(body), ';', 1<<30))
	p.HeaderComments = c.commentsIn(start(body)+len("package"), n.Name.Start)
}

// semicolons returns the semicolons after the body of a declaration.
func (c *canonical) semicolons(span ranged) []int {
	if isNil(span) {
		c.fail("the source tree has a syntax error in a declaration")
	}
	return c.separators(end(span), ';', 1<<30)
}

func (c *canonical) decl(d types.Decl) grammar.Decl {
	switch d := d.(type) {
	case *types.TypeDef:
		sep := c.semicolons(d.SourceSpan)
		def := &grammar.TypeDef{Name: c.name(d.Name)}
		name := d.Name
		eq := c.nextToken(name.End)
		def.HeaderComments = c.commentsIn(d.SourceSpan.Start+len("type"), name.Start)
		def.HeaderComments = append(def.HeaderComments, c.commentsIn(name.End, eq)...)
		if st, ok := d.Type.(*types.StructType); ok && st != nil {
			// The hand-written parser completes the struct from the "=".
			def.Type = c.structType(st, eq)
		} else {
			if isNil(d.Type) {
				c.fail("the source tree has a syntax error in a declaration")
			}
			def.Type = c.typ(d.Type)
			def.HeaderComments = append(def.HeaderComments, c.commentsIn(eq+1, start(d.Type.(ranged)))...)
		}
		c.statement(&def.SourceInfo, d.SourceSpan, sep, false)
		return def
	case *types.TypeAlias:
		sep := c.semicolons(d.SourceSpan)
		alias := &grammar.TypeAlias{Name: c.name(d.Name)}
		name := d.Name
		if isNil(d.Type) {
			c.fail("the source tree has a syntax error in a declaration")
		}
		ty := d.Type.(ranged)
		tilde := c.nextToken(name.End)
		alias.HeaderComments = c.commentsIn(d.SourceSpan.Start+len("type"), name.Start)
		alias.HeaderComments = append(alias.HeaderComments, c.commentsIn(name.End, tilde)...)
		alias.HeaderComments = append(alias.HeaderComments, c.commentsIn(tilde+1, start(ty))...)
		alias.Type = c.typ(d.Type)
		c.statement(&alias.SourceInfo, d.SourceSpan, sep, false)
		return alias
	case *types.VarDef:
		sep := c.semicolons(d.SourceSpan)
		v := &grammar.VarDef{Name: c.name(d.Name)}
		name := d.Name
		v.HeaderComments = c.commentsIn(d.SourceSpan.Start+len("var"), name.Start)
		if !isNil(d.Type) {
			v.HeaderComments = append(v.HeaderComments, c.commentsIn(name.End, start(d.Type.(ranged)))...)
			v.Type = c.typ(d.Type)
		}
		if !isNil(d.Value) {
			v.Value = c.action(d.Value)
		}
		c.statement(&v.SourceInfo, d.SourceSpan, sep, false)
		return v
	case *types.RuleDef:
		return c.ruleDef(d, c.semicolons(d.SourceSpan))
	}
	c.fail("the source tree has a syntax error in a declaration")
	return nil
}

func (c *canonical) ruleDef(d *types.RuleDef, sep []int) *grammar.RuleDef {
	rd := &grammar.RuleDef{Name: c.name(d.Name)}
	attrs := d.Attrs
	keyword := d.SourceSpan.Start
	for _, attr := range attrs {
		rd.Attrs = append(rd.Attrs, c.attr(attr))
	}
	if len(attrs) > 0 {
		keyword = c.nextToken(attrs[len(attrs)-1].End)
		// No source-bearing node starts at "rule" after attributes, so the
		// declaration owns the comments before it.
		for _, g := range c.gapList {
			if g.End == keyword && g.HasNext {
				rd.KeywordComments = g.Leading
				rd.KeywordDetachedComments = append([]string(nil), g.Detached...)
			}
		}
	}
	name := d.Name
	// The header holds the comments before the name, the type and the "=".
	rd.HeaderComments = c.commentsIn(keyword+len("rule"), name.Start)
	from := name.End
	if !isNil(d.Type) {
		rd.Type = c.typ(d.Type)
		var gaps [][2]int
		from = c.typeHeader(d.Type, from, &gaps)
		for _, g := range gaps {
			rd.HeaderComments = append(rd.HeaderComments, c.commentsIn(g[0], g[1])...)
		}
	}
	rd.HeaderComments = append(rd.HeaderComments, c.commentsIn(from, c.nextToken(from))...)
	rd.Expr = c.rule(d.Expr)
	if !isNil(d.Action) {
		rd.Action, rd.DefaultAction = c.actionOrDefault(d.Action)
	}
	c.statement(&rd.SourceInfo, d.SourceSpan, sep, false)
	if len(attrs) > 0 {
		// The hand-written parser gives a rule with attributes the position of
		// "rule" for diagnostics.
		c.created(c.last(), keyword)
	}
	return rd
}

// Types.

func (c *canonical) structType(n *types.StructType, start int) *grammar.StructType {
	st := &grammar.StructType{Fields: nil}
	for _, spec := range n.Fields {
		f, ok := spec.(*types.Field)
		if !ok || f == nil || f.SourceSpan == nil || isNil(f.Type) {
			c.fail("the source tree has a syntax error in a struct field")
		}
		field := &grammar.Field{Name: c.name(f.Name)}
		field.HeaderComments = c.commentsIn(f.Name.End, start0(f.Type))
		field.Type = c.typ(f.Type)
		c.statement(&field.SourceInfo, f.SourceSpan, c.separators(f.SourceSpan.End, ';', 1), false)
		st.Fields = append(st.Fields, field)
	}
	c.add(&st.SourceInfo, sourceast.Event{Start: start, End: n.End})
	return st
}

// start0 returns the start of a value of the tree that is known to be present.
func start0(x any) int { return start(x.(ranged)) }

func (c *canonical) types(list []types.Type) []grammar.TypeExpr {
	var out []grammar.TypeExpr
	each(c, list, func(a types.Type) { out = append(out, c.typ(a)) })
	return out
}

// typ builds a type expression: a types.Type, or the types.DefinedType of a
// type definition.
func (c *canonical) typ(n any) grammar.TypeExpr {
	switch n := n.(type) {
	case *types.TypeRef:
		t := &grammar.TypeRef{Name: c.name(n.Name)}
		if t.Name == "node" {
			c.errs.add(c.pos.At(n.Name.Start), bareNode+" (zp fix rewrites it)")
		}
		c.span(&t.SourceInfo, n)
		return t
	case *types.Bottom:
		t := &grammar.TypeRef{Name: n.Value}
		c.span(&t.SourceInfo, n)
		return t
	case *types.TypeApp:
		t := &grammar.TypeApp{Name: c.name(n.Name)}
		t.Args = c.types(n.Args)
		c.span(&t.SourceInfo, n)
		return t
	case *types.TypeGroup:
		t := &grammar.TypeGroup{}
		t.Type = c.typ(n.Type)
		c.span(&t.SourceInfo, n)
		return t
	case *types.TupleType:
		t := &grammar.TupleType{}
		t.Types = c.types(n.Types)
		c.span(&t.SourceInfo, n)
		return t
	case *types.StructType:
		return c.structType(n, n.Start)
	case *types.OptionalType:
		t := &grammar.OptionalType{}
		t.Elem = c.typ(n.Type)
		c.span(&t.SourceInfo, n)
		return t
	case *types.ListType:
		t := &grammar.ListType{}
		t.Elem = c.typ(n.Type)
		c.span(&t.SourceInfo, n)
		return t
	case *types.JoinType:
		t := &grammar.JoinType{}
		for _, o := range flattenJoin(n) {
			t.Types = append(t.Types, c.typ(o))
		}
		c.span(&t.SourceInfo, n)
		return t
	case *types.MeetType:
		t := &grammar.MeetType{}
		for _, o := range flattenMeet(n) {
			t.Types = append(t.Types, c.typ(o))
		}
		c.span(&t.SourceInfo, n)
		return t
	}
	c.fail("unexpected type %T", n)
	return nil
}

// The flatten functions return the operands of a chain of binary nodes of one
// type, which the public AST holds in one node.

func flattenJoin(n types.Type) []types.Type {
	if j, ok := n.(*types.JoinType); ok && j != nil {
		return append(flattenJoin(j.L), j.R)
	}
	return []types.Type{n}
}

func flattenMeet(n types.Type) []types.Type {
	if m, ok := n.(*types.MeetType); ok && m != nil {
		return append(flattenMeet(m.L), m.R)
	}
	return []types.Type{n}
}

// typeHeader collects the gaps before the tokens of a type in a rule header,
// where the hand-written parser reads the tokens as part of the header: all
// but those inside delimiters and the closing ones. from is the offset where
// the token before the type ends; it returns where the type ends.
func (c *canonical) typeHeader(t types.Type, from int, gaps *[][2]int) int {
	gap := func(to int) { *gaps = append(*gaps, [2]int{from, to}) }
	chain := func(operands []types.Type) int {
		for i, o := range operands {
			if i > 0 {
				op := c.nextToken(from)
				gap(op)
				from = op + 1
			}
			from = c.typeHeader(o, from, gaps)
		}
		return from
	}
	switch t := t.(type) {
	case *types.ListType:
		gap(t.Start)
		return c.typeHeader(t.Type, t.Start+len("[]"), gaps)
	case *types.OptionalType:
		gap(t.Start)
		return c.typeHeader(t.Type, t.Start+len("*"), gaps)
	case *types.JoinType:
		return chain(flattenJoin(t))
	case *types.MeetType:
		return chain(flattenMeet(t))
	case *types.StructType:
		return t.End
	}
	// A name, a name with arguments, a group and a tuple have one token before
	// the delimiters of their own.
	r := t.(ranged)
	gap(start(r))
	return end(r)
}

// commentsIn returns the contents of the comments between the offsets from and
// to, one string for each.
func (c *canonical) commentsIn(from, to int) []string {
	var out []string
	for _, cm := range c.comments {
		r, ok := cm.(ranged)
		if !ok || isNil(cm) {
			c.fail("the source tree has a syntax error in a comment")
		}
		if s, e := r.Range(); s < from || e > to {
			continue
		}
		if g, ok := cm.(*types.CommentGroup); ok {
			for _, line := range g.Lines {
				out = append(out, c.commentContent(line.Start, line.End))
			}
			continue
		}
		s, e := r.Range()
		out = append(out, c.commentContent(s, e))
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

// decode runs read, a lexer method, on the source of the tree value n and
// reports the errors it finds at their place in the whole source.
func (c *canonical) decode(n ranged, read func(*lexer)) {
	s, e := n.Range()
	l := &lexer{line: 1, col: 1, errs: &errorList{}, src: c.src[s:e]}
	read(l)
	origin := c.pos.At(s)
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
func (c *canonical) literal(n *types.String) *grammar.Literal {
	lit := &grammar.Literal{}
	c.decode(n, func(l *lexer) {
		// The suffix i follows the closing quote, so the lexer stops there.
		lit.Value = l.string()
	})
	lit.IgnoreCase = strings.HasSuffix(n.Value, "i") && !strings.HasSuffix(n.Value, "\"")
	return lit
}

// openLiteral builds the literal that the user is typing from the text that the recovery of a string
// without a closing quote skipped, and reports the missing quote. It returns nil for any other error.
func (c *canonical) openLiteral(n *types.Error) *grammar.Literal {
	s, _ := n.Range()
	if !c.partial || s >= len(c.src) || c.src[s] != '"' {
		return nil
	}
	lit := &grammar.Literal{}
	before := len(c.errs.list)
	c.decode(n, func(l *lexer) { lit.Value = l.string() })
	// The syntax error of the tree already reports the missing quote.
	kept := c.errs.list[:before]
	for _, e := range c.errs.list[before:] {
		if e.Msg != "unterminated string" {
			kept = append(kept, e)
		}
	}
	c.errs.list = kept
	return lit
}

func (c *canonical) stringValue(n *types.String) string {
	return c.literal(n).Value
}

// integer decodes the digits of the decimal literal n, which may follow a sign.
func (c *canonical) integer(n *types.Int) int {
	if n == nil {
		c.fail("the source tree has no number where one is required")
	}
	text := n.Value
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
		c.errorRange(n.End-len(digits), n.End, "invalid number "+digits)
		return 0
	}
	if negative {
		return -v
	}
	return v
}
