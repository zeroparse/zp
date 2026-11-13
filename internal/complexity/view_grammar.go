package complexity

import (
	"fmt"
	"slices"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/ir/core"
)

// Grammar returns the rules of the view as grammar rules: rules are the rules the source module
// is built from, rule i of the module from rules[i]. A node the view shares with the module is
// the expression it is built from, and a node the view builds is written in the grammar's syntax
// at the position of the expression it rewrites, so that the formatted rules can be compared with
// the grammar, the complexity classifier can read them, and the view can run on an engine that
// reads grammars.
func (v *View) Grammar(rules []*grammar.RuleDef) ([]*grammar.RuleDef, error) {
	out, _, err := v.GrammarText(rules)
	return out, err
}

// GrammarText returns the rules of Grammar and, for each expression of them that the view
// builds, the source text it stands for (Text).
func (v *View) GrammarText(rules []*grammar.RuleDef) ([]*grammar.RuleDef, map[grammar.Expr]string, error) {
	if len(rules) != len(v.Module.Rules) {
		return nil, nil, fmt.Errorf("%d rules for a module of %d", len(rules), len(v.Module.Rules))
	}
	w := &writer{View: v, text: map[grammar.Expr]string{}}
	out := make([]*grammar.RuleDef, len(rules))
	for i, rd := range rules {
		body, err := w.expr(v.Module.Rules[i].Body)
		if err != nil {
			return nil, nil, fmt.Errorf("rule %s: %w", rd.Name, err)
		}
		if body == rd.Expr {
			out[i] = rd
			continue
		}
		cp := *rd
		cp.Expr = body
		out[i] = &cp
	}
	return out, w.text, nil
}

// writer writes nodes of a view as grammar expressions and records the source text of each
// expression that it writes for a built node.
type writer struct {
	*View
	text map[grammar.Expr]string
}

// expr returns the grammar expression of the node n of the view.
func (v *View) expr(n core.Node) (grammar.Expr, error) {
	return (&writer{View: v}).expr(n)
}

// expr returns the grammar expression of the node n of the view.
func (v *writer) expr(n core.Node) (grammar.Expr, error) {
	if e, ok := v.Origin[n]; ok {
		return e, nil
	}
	e, err := v.built(n)
	if err != nil || e == nil {
		return e, err
	}
	if v.text != nil {
		if t, ok := v.Text[n]; ok {
			v.text[e] = t
		}
	}
	return e, nil
}

// built returns the grammar expression of the node n that the view builds.
func (v *writer) built(n core.Node) (grammar.Expr, error) {
	if l, ok := v.chainOf(n); ok {
		return v.chainExpr(n, l)
	}
	if s, ok := v.replaces[n]; ok {
		if e, ok := v.Origin[s]; ok {
			r, err := v.withParts(n, e)
			if err != nil {
				return nil, err
			}
			if r != nil {
				if at, ok := v.at[n]; ok && at != s {
					v.place(r, at)
				}
				return r, nil
			}
		}
	}
	exprs := func(ns []core.Node) ([]grammar.Expr, error) {
		es := make([]grammar.Expr, len(ns))
		for i, x := range ns {
			e, err := v.expr(x)
			if err != nil {
				return nil, err
			}
			es[i] = e
		}
		return es, nil
	}
	e, err := v.spelled(n, exprs)
	if err != nil {
		return nil, err
	}
	if s, ok := v.at[n]; ok {
		v.place(e, s)
	}
	return e, nil
}

// place gives the expression e, which the writer built, the position of the module node at.
func (v *writer) place(e grammar.Expr, at core.Node) {
	if o, ok := v.Origin[at].(interface{ Source() *grammar.SourceInfo }); ok && o.Source() != nil {
		if d, ok := e.(interface{ Source() *grammar.SourceInfo }); ok {
			*d.Source() = grammar.SourceInfo{Pos: o.Source().Diagnostic()}
		}
	}
}

// spelled returns the grammar expression of the node n that the view builds, of a kind written
// in the grammar's syntax of its own, with the expressions exprs of its parts.
func (v *writer) spelled(n core.Node, exprs func([]core.Node) ([]grammar.Expr, error)) (grammar.Expr, error) {
	switch n := n.(type) {
	case *core.Seq:
		items, err := exprs(n.Items)
		if err != nil {
			return nil, err
		}
		return &grammar.Seq{Items: items}, nil
	case *core.Choice:
		alts, err := exprs(core.Children(n))
		if err != nil {
			return nil, err
		}
		return &grammar.Choice{Alts: alts}, nil
	case *core.Repeat:
		body, err := v.expr(n.Body)
		if err != nil {
			return nil, err
		}
		return &grammar.Repeat{Expr: body, Min: n.Min, Max: n.Max}, nil
	case *core.Opt:
		body, err := v.expr(n.Body)
		if err != nil {
			return nil, err
		}
		return &grammar.Optional{Expr: body}, nil
	case *core.Look:
		body, err := v.expr(n.Body)
		if err != nil {
			return nil, err
		}
		if n.Neg {
			return &grammar.Not{Expr: body}, nil
		}
		return &grammar.And{Expr: body}, nil
	case *core.ClassNode:
		c := &grammar.CharClass{Negated: n.Class.Negated}
		for _, r := range n.Class.Ranges {
			c.Ranges = append(c.Ranges, grammar.CharRange{Lo: r.Lo, Hi: r.Hi})
		}
		return c, nil
	case *core.Cut:
		return &grammar.Cut{}, nil
	case *core.Succeed:
		return &grammar.Top{}, nil
	case *core.FailNode:
		return &grammar.Bottom{}, nil
	}
	return nil, fmt.Errorf("no expression for a built %T", n)
}

// withParts returns the expression e of the node of the module that the built node n replaces,
// with the parts of n: a copy of e with the expression of the body of n. It returns nil for a
// node whose kind is written in the grammar's syntax of its own.
func (v *writer) withParts(n core.Node, e grammar.Expr) (grammar.Expr, error) {
	var body core.Node
	switch n := n.(type) {
	case *core.Repeat:
		body = n.Body
	case *core.Opt:
		body = n.Body
	case *core.Look:
		body = n.Body
	case *core.Capture:
		body = n.Body
	case *core.Drop:
		body = n.Body
	case *core.Text:
		body = n.Body
	case *core.Label:
		body = n.Body
	case *core.Recover:
		body = n.Body
	case *core.Stream:
		body = n.Repeat
	default:
		return nil, nil
	}
	x, err := v.expr(body)
	if err != nil {
		return nil, err
	}
	switch e := e.(type) {
	case *grammar.Repeat:
		r, ok := n.(*core.Repeat)
		if !ok {
			return nil, fmt.Errorf("a built %T of a %T", n, e)
		}
		c := *e
		c.Expr, c.Min, c.Max = x, r.Min, r.Max
		return &c, nil
	case *grammar.Optional:
		c := *e
		c.Expr = x
		return &c, nil
	case *grammar.And:
		c := *e
		c.Expr = x
		return &c, nil
	case *grammar.Not:
		c := *e
		c.Expr = x
		return &c, nil
	case *grammar.Capture:
		c := *e
		c.Expr = x
		return &c, nil
	case *grammar.Discard:
		c := *e
		c.Expr = x
		return &c, nil
	case *grammar.Atomic:
		c := *e
		c.Expr = x
		return &c, nil
	case *grammar.Attributed:
		if len(e.Attrs) != 1 {
			return nil, fmt.Errorf("a built %T of %d attributes", n, len(e.Attrs))
		}
		c := *e
		c.Expr, c.Breaks = x, nil
		return &c, nil
	}
	return nil, fmt.Errorf("a built %T of a %T", n, e)
}

// BuiltTexts returns, for each node of the rules of the view that the view builds, its spelling
// in the grammar's syntax and the text it stands for, as "spelling = text", sorted. A node that
// cannot be written is spelled by its error.
func (v *View) BuiltTexts() []string {
	var out []string
	for _, r := range v.Module.Rules {
		// The nodes of the attributes of an expression of several attributes below the outermost
		// one are written with it, as one expression.
		inner := map[core.Node]bool{}
		core.Walk(r.Body, func(n core.Node) {
			if inner[n] {
				return
			}
			if l, ok := v.chainOf(n); ok {
				_, parts, _ := v.chainParts(n, l)
				for _, p := range parts[1:] {
					inner[p] = true
				}
			}
			if _, ok := v.Origin[n]; ok {
				return
			}
			spelling := ""
			if e, err := v.expr(n); err != nil {
				spelling = err.Error()
			} else {
				spelling = grammar.FormatExpr(e)
			}
			out = append(out, spelling+" = "+v.Text[n])
		})
	}
	slices.Sort(out)
	return out
}

// An expression of several attributes, as `(X @error(message="m")) #recover(skip=s)`, is one
// grammar expression that the module builds into one node per attribute, the last attribute
// outermost: a Label for @error and @diagnostic, a Recover for #recover, a Missing for #missing,
// and for #stream the Stream of the repetition, innermost. Only the outermost node has the
// expression as its origin. An attrChain records the others, so that the view writes the nodes
// that it keeps back as one expression.
type attrChain struct {
	e *grammar.Attributed
	// nodes[i] is the node of the module that attribute i is built into, nil for #stream, whose
	// node is stream.
	nodes  []core.Node
	stream core.Node
}

// A chainLink is the place of a node in an attrChain: the attribute at it is built from.
type chainLink struct {
	chain *attrChain
	at    int
}

// attrChains returns the place of each node of the module of src that an attribute of an
// expression of several attributes is built into.
func attrChains(src *Source) map[core.Node]chainLink {
	links := map[core.Node]chainLink{}
	for _, r := range src.Module.Rules {
		core.Walk(r.Body, func(n core.Node) {
			e, ok := src.Origin[n].(*grammar.Attributed)
			if !ok || len(e.Attrs) < 2 {
				return
			}
			c := &attrChain{e: e, nodes: make([]core.Node, len(e.Attrs))}
			streamAt := -1
			node := n
			for i := len(e.Attrs) - 1; i >= 0; i-- {
				if e.Attrs[i].Name == "stream" {
					streamAt = i
					continue
				}
				if !attrNode(e.Attrs[i].Name, node) {
					return
				}
				c.nodes[i], node = node, attrBody(node)
			}
			if streamAt >= 0 {
				if _, ok := node.(*core.Stream); !ok {
					return
				}
				c.stream = node
				links[node] = chainLink{c, streamAt}
			}
			for i, x := range c.nodes {
				if x != nil {
					links[x] = chainLink{c, i}
				}
			}
		})
	}
	return links
}

// attrNode reports whether n is of the kind that the attribute name is built into.
func attrNode(name string, n core.Node) bool {
	switch n.(type) {
	case *core.Label:
		return name == "error" || name == "diagnostic"
	case *core.Recover:
		return name == "recover"
	case *core.Missing:
		return name == "missing"
	}
	return false
}

// attrBody returns the operand of the attribute node n.
func attrBody(n core.Node) core.Node {
	switch n := n.(type) {
	case *core.Label:
		return n.Body
	case *core.Recover:
		return n.Body
	case *core.Missing:
		return n.Lit
	case *core.Stream:
		return n.Repeat
	}
	return nil
}

// chainOf returns the place in an attrChain of the node n of the view: that of n, or of the node
// of the module that n replaces.
func (v *View) chainOf(n core.Node) (chainLink, bool) {
	if l, ok := v.chains[n]; ok {
		return l, true
	}
	if s, ok := v.replaces[n]; ok {
		l, ok := v.chains[s]
		return l, ok
	}
	return chainLink{}, false
}

// chainParts follows the chain of l from its node n of the view down to the operand of the
// attributes: keep reports the attributes whose nodes the view keeps (a rule may drop one, as N7
// drops a label), parts are n and the kept nodes below it, and body is the operand.
func (v *View) chainParts(n core.Node, l chainLink) (keep []bool, parts []core.Node, body core.Node) {
	c := l.chain
	keep = make([]bool, len(c.e.Attrs))
	parts = []core.Node{n}
	node := n
	is := func(x, src core.Node) bool { return x == src || v.replaces[x] == src }
	for i := l.at; i >= 0; i-- {
		if src := c.nodes[i]; src != nil && is(node, src) {
			keep[i] = true
			if node != n {
				parts = append(parts, node)
			}
			node = attrBody(node)
		}
	}
	if c.stream != nil && is(node, c.stream) {
		// #stream has no node of its own: it stays with the Stream.
		for i, a := range c.e.Attrs {
			keep[i] = keep[i] || a.Name == "stream"
		}
		if node != n {
			parts = append(parts, node)
		}
		node = attrBody(node)
	}
	return keep, parts, node
}

// chainExpr returns the expression of the node n of the view at l: the expression of the
// attributes whose nodes the view keeps from n down, around the expression of their operand.
func (v *writer) chainExpr(n core.Node, l chainLink) (grammar.Expr, error) {
	keep, _, body := v.chainParts(n, l)
	if body == nil {
		return nil, fmt.Errorf("a built %T without an operand", n)
	}
	x, err := v.expr(body)
	if err != nil {
		return nil, err
	}
	e := l.chain.e
	var attrs []*grammar.Attribute
	for i, a := range e.Attrs {
		if keep[i] {
			attrs = append(attrs, a)
		}
	}
	if len(attrs) == 0 {
		return x, nil
	}
	if x == e.Expr && len(attrs) == len(e.Attrs) {
		return e, nil
	}
	c := *e
	c.Expr, c.Attrs, c.Breaks = x, attrs, nil
	return &c, nil
}
