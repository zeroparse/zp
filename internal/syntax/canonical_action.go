package syntax

import (
	"slices"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/syntax/grammarparser/types"
)

// Value expressions.

// actionOrDefault builds the action of a rule or of an inline action: a value
// expression, a block, or the default action, which has no term.
func (c *canonical) actionOrDefault(n types.Action) (grammar.Term, bool) {
	if _, ok := n.(*types.DefaultAction); ok {
		return nil, true
	}
	return c.action(n), false
}

// statements builds the statements of an action block.
func (c *canonical) statements(list []types.ActionStmt) []grammar.Term {
	out := []grammar.Term{}
	for _, st := range list {
		if isNil(st) {
			c.fail("the source tree has a syntax error in an action block")
		}
		out = append(out, c.action(st))
		// A statement takes the trailing group after its separator too.
		c.joined(c.last(), c.separators(end(st.(ranged)), ';', 1))
	}
	return out
}

// predicate builds a predicate action. Its statements are a block of their own
// unless there is exactly one.
func (c *canonical) predicate(n *types.Predicate) *grammar.Predicate {
	stmts := c.statements(n.Actions)
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
func (c *canonical) terms(list []types.ActionExpr) []grammar.Term {
	out := []grammar.Term{}
	each(c, list, func(a types.ActionExpr) { out = append(out, c.action(a)) })
	return out
}

// action builds a value expression: a types.ActionExpr, a types.ActionStmt or
// a types.Action other than the default action.
func (c *canonical) action(n any) grammar.Term {
	switch n := n.(type) {
	case *types.Int:
		t := &grammar.IntLit{Value: c.integer(n)}
		c.span(&t.SourceInfo, n)
		return t
	case *types.String:
		t := &grammar.StringLit{Value: c.stringValue(n)}
		c.span(&t.SourceInfo, n)
		return t
	case *types.True:
		t := &grammar.BoolLit{Value: true}
		c.span(&t.SourceInfo, n)
		return t
	case *types.False:
		t := &grammar.BoolLit{Value: false}
		c.span(&t.SourceInfo, n)
		return t
	case *types.Nil:
		t := &grammar.NilLit{}
		c.span(&t.SourceInfo, n)
		return t
	case *types.CaptureRef:
		// The text is $name.
		name := n.Value[1:]
		if slices.Contains(c.params, name) {
			c.errorAt(n, "lambda parameters are written without $: write %s", name)
		}
		t := &grammar.CaptureRef{Name: name}
		c.span(&t.SourceInfo, n)
		return t
	case *types.VarRef:
		name := n.Value
		if slices.Contains(c.params, name) {
			t := &grammar.CaptureRef{Name: name, Param: true}
			c.span(&t.SourceInfo, n)
			return t
		}
		t := &grammar.VarRef{Name: name}
		c.span(&t.SourceInfo, n)
		return t
	case *types.BinaryOp:
		t := &grammar.Binary{Op: n.Op}
		t.L = c.action(n.L)
		t.R = c.action(n.R)
		c.created(c.span(&t.SourceInfo, n), c.nextToken(end(n.L.(ranged))))
		return t
	case *types.UnaryOp:
		if x, ok := n.X.(*types.Int); ok && n.Op == "-" && x.Value == minimumIntegerText[1:] && x.Start == n.Start+1 {
			// The minimum int is one token, whose magnitude alone is not an int.
			t := &grammar.IntLit{Value: minimumIntegerValue}
			c.span(&t.SourceInfo, n)
			return t
		}
		t := &grammar.Unary{Op: n.Op}
		t.X = c.action(n.X)
		c.span(&t.SourceInfo, n)
		return t
	case *types.FieldAccess:
		t := &grammar.Member{Name: c.name(n.Name)}
		t.X = c.action(n.X)
		if captureMember(t.Name) {
			c.errorAt(n.Name, "a capture is read with .$%s", t.Name)
		}
		c.created(c.span(&t.SourceInfo, n), n.Name.Start)
		return t
	case *types.CaptureAccess:
		if n.Capture == nil {
			c.fail("the source tree has no capture of a member")
		}
		t := &grammar.Member{Name: n.Capture.Value}
		t.X = c.action(n.X)
		c.created(c.span(&t.SourceInfo, n), n.Capture.Start)
		return t
	case *types.Cast:
		t := &grammar.Cast{}
		t.X = c.action(n.X)
		t.Type = c.typ(n.Type)
		dot := c.nextToken(end(n.X.(ranged)))
		c.created(c.span(&t.SourceInfo, n), c.nextToken(dot+1))
		return t
	case *types.Call:
		t := &grammar.Call{Func: c.text(c.present(n.X))}
		t.Args = c.terms(n.Args)
		c.span(&t.SourceInfo, n)
		return t
	case *types.Constructor:
		t := &grammar.New{Type: c.text(c.present(n.X))}
		for _, f := range n.Inits {
			if f == nil || f.SourceSpan == nil {
				c.fail("the source tree has a syntax error in a constructor")
			}
			init := &grammar.FieldInit{Name: c.name(f.Name)}
			init.Value = c.action(f.Val)
			c.statement(&init.SourceInfo, f.SourceSpan, c.separators(f.SourceSpan.End, ',', 1), true)
			t.Fields = append(t.Fields, init)
		}
		if args := c.terms(n.Args); len(args) > 0 {
			t.Args = args
		}
		c.span(&t.SourceInfo, n)
		return t
	case *types.ListConstructor:
		t := &grammar.ListConstructor{}
		t.Elem = c.typ(n.Elem)
		t.Items = c.terms(n.Items)
		c.span(&t.SourceInfo, n)
		return t
	case *types.ActionTuple:
		t := &grammar.TupleLit{Elems: []grammar.Term{}}
		t.Elems = append(t.Elems, c.terms(n.Elems)...)
		c.span(&t.SourceInfo, n)
		return t
	case *types.ActionGroup:
		t := &grammar.TermGroup{}
		t.Term = c.action(n.Expr)
		c.span(&t.SourceInfo, n)
		return t
	case *types.Lambda:
		t := &grammar.Lambda{}
		for _, p := range n.Params {
			t.Params = append(t.Params, c.name(p))
		}
		outer := len(c.params)
		c.params = append(c.params, t.Params...)
		t.Body = c.action(n.Body)
		c.params = c.params[:outer]
		c.span(&t.SourceInfo, n)
		return t
	case *types.ActionBlock:
		t := &grammar.Block{Statements: c.statements(n.Actions)}
		c.span(&t.SourceInfo, n)
		return t
	case *types.ActionBind:
		t := &grammar.Bind{Name: c.name(n.Name)}
		t.Value = c.action(n.Expr)
		c.span(&t.SourceInfo, n)
		return t
	}
	c.fail("unexpected action %T", n)
	return nil
}

// present returns x, a value of the tree that must be there.
func (c *canonical) present(x any) ranged {
	r, ok := x.(ranged)
	if !ok || isNil(x) {
		c.fail("the source tree has a syntax error in an action")
	}
	return r
}
