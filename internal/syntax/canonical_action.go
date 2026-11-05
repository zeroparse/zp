package syntax

import (
	"slices"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/value"
)

// Value expressions.

// actionOrDefault builds the action of a rule or of an inline action: a value
// expression, a block, or the default action, which has no term.
func (c *canonical) actionOrDefault(n *value.Node) (grammar.Term, bool) {
	if n.Type() == "DefaultAction" {
		return nil, true
	}
	return c.action(n), false
}

// statements builds the statements of an action block.
func (c *canonical) statements(n *value.Node) []grammar.Term {
	out := []grammar.Term{}
	for _, st := range listField(n, "Actions") {
		out = append(out, c.action(st))
		// A statement takes the trailing group after its separator too.
		c.joined(c.last(), c.separators(st.End(), ';', 1))
	}
	return out
}

// predicate builds a predicate action. Its statements are a block of their own
// unless there is exactly one.
func (c *canonical) predicate(n *value.Node) *grammar.Predicate {
	stmts := c.statements(n)
	p := &grammar.Predicate{}
	if len(stmts) == 1 {
		p.Term = stmts[0]
	} else {
		b := &grammar.Block{Statements: stmts}
		c.span(&b.SourceInfo, n)
		p.Term = b
	}
	c.span(&p.SourceInfo, n)
	return p
}

// terms builds the elements of a list of values, which the hand-written parser
// holds in an empty list when there are none.
func (c *canonical) terms(n *value.Node, field string) []grammar.Term {
	out := []grammar.Term{}
	c.each(n, field, func(a *value.Node) { out = append(out, c.action(a)) })
	return out
}

func (c *canonical) action(n *value.Node) grammar.Term {
	switch n.Type() {
	case "Int":
		t := &grammar.IntLit{Value: c.integer(n)}
		c.span(&t.SourceInfo, n)
		return t
	case "String":
		t := &grammar.StringLit{Value: c.stringValue(n)}
		c.span(&t.SourceInfo, n)
		return t
	case "True", "False":
		t := &grammar.BoolLit{Value: n.Type() == "True"}
		c.span(&t.SourceInfo, n)
		return t
	case "Nil":
		t := &grammar.NilLit{}
		c.span(&t.SourceInfo, n)
		return t
	case "CaptureRef":
		// The text is $name.
		name := n.Text()[1:]
		if slices.Contains(c.params, name) {
			c.errorAt(n, "lambda parameters are written without $: write %s; run zp fix to rewrite $%s", name, name)
		}
		t := &grammar.CaptureRef{Name: name}
		c.span(&t.SourceInfo, n)
		return t
	case "VarRef":
		name := n.Text()
		if slices.Contains(c.params, name) {
			t := &grammar.CaptureRef{Name: name, Param: true}
			c.span(&t.SourceInfo, n)
			return t
		}
		t := &grammar.VarRef{Name: name}
		c.span(&t.SourceInfo, n)
		return t
	case "BinaryOp":
		t := &grammar.Binary{Op: stringField(n, "Op")}
		t.L = c.action(nodeField(n, "L"))
		t.R = c.action(nodeField(n, "R"))
		c.created(c.span(&t.SourceInfo, n), c.nextToken(nodeField(n, "L").End()))
		return t
	case "UnaryOp":
		x := nodeField(n, "X")
		if stringField(n, "Op") == "-" && x.Type() == "Int" && x.Text() == minimumIntegerText[1:] && x.Start() == n.Start()+1 {
			// The minimum int is one token, whose magnitude alone is not an int.
			t := &grammar.IntLit{Value: minimumIntegerValue}
			c.span(&t.SourceInfo, n)
			return t
		}
		t := &grammar.Unary{Op: stringField(n, "Op")}
		t.X = c.action(x)
		c.span(&t.SourceInfo, n)
		return t
	case "FieldAccess":
		t := &grammar.Member{Name: textField(n, "Name")}
		t.X = c.action(nodeField(n, "X"))
		name := nodeField(n, "Name")
		if captureMember(t.Name) {
			c.errorAt(name, "a capture is read with .$%s; run zp fix to rewrite .%s", t.Name, t.Name)
		}
		c.created(c.span(&t.SourceInfo, n), name.Start())
		return t
	case "CaptureAccess":
		t := &grammar.Member{Name: textField(n, "Capture")}
		t.X = c.action(nodeField(n, "X"))
		c.created(c.span(&t.SourceInfo, n), nodeField(n, "Capture").Start())
		return t
	case "Cast":
		t := &grammar.Cast{}
		t.X = c.action(nodeField(n, "X"))
		t.Type = c.typ(nodeField(n, "Type"))
		dot := c.nextToken(nodeField(n, "X").End())
		c.created(c.span(&t.SourceInfo, n), c.nextToken(dot+1))
		return t
	case "Call":
		t := &grammar.Call{Func: textField(n, "X")}
		t.Args = c.terms(n, "Args")
		c.span(&t.SourceInfo, n)
		return t
	case "Constructor":
		t := &grammar.New{Type: textField(n, "X")}
		for _, f := range listField(n, "Inits") {
			span := nodeField(f, "SourceSpan")
			init := &grammar.FieldInit{Name: textField(f, "Name")}
			init.Value = c.action(nodeField(f, "Val"))
			c.statement(&init.SourceInfo, f, c.separators(span.End(), ',', 1), true)
			t.Fields = append(t.Fields, init)
		}
		if args := c.terms(n, "Args"); len(args) > 0 {
			t.Args = args
		}
		c.span(&t.SourceInfo, n)
		return t
	case "ListConstructor":
		t := &grammar.ListConstructor{}
		t.Elem = c.typ(nodeField(n, "Elem"))
		t.Items = c.terms(n, "Items")
		c.span(&t.SourceInfo, n)
		return t
	case "ActionTuple":
		t := &grammar.TupleLit{Elems: []grammar.Term{}}
		t.Elems = append(t.Elems, c.terms(n, "Elems")...)
		c.span(&t.SourceInfo, n)
		return t
	case "ActionGroup":
		t := &grammar.TermGroup{}
		t.Term = c.action(nodeField(n, "Expr"))
		c.span(&t.SourceInfo, n)
		return t
	case "Lambda":
		t := &grammar.Lambda{}
		for _, p := range listField(n, "Params") {
			t.Params = append(t.Params, p.Text())
		}
		outer := len(c.params)
		c.params = append(c.params, t.Params...)
		t.Body = c.action(nodeField(n, "Body"))
		c.params = c.params[:outer]
		c.span(&t.SourceInfo, n)
		return t
	case "ActionBlock":
		t := &grammar.Block{Statements: c.statements(n)}
		c.span(&t.SourceInfo, n)
		return t
	case "ActionBind":
		t := &grammar.Bind{Name: textField(n, "Name")}
		t.Value = c.action(nodeField(n, "Expr"))
		c.span(&t.SourceInfo, n)
		return t
	}
	c.fail("unexpected action node %s", n.Type())
	return nil
}
