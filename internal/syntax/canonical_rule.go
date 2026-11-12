package syntax

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/syntax/genparser/types"
	"zeroparse.dev/zp/internal/syntax/sourceast"
)

// Parser expressions. The nodes of the AST are built, and recorded for source
// information, in the order the hand-written parser completes them.

func (c *canonical) rule(n types.RuleExpr) grammar.Expr {
	switch n := n.(type) {
	case *types.RuleRef:
		e := &grammar.Ref{Name: c.name(n.Name)}
		c.span(&e.SourceInfo, n)
		return e
	case *types.LevelCall:
		e := &grammar.Ref{Name: c.name(n.Rule), Level: c.name(n.Level)}
		c.created(c.span(&e.SourceInfo, n), n.Rule.Start)
		return e
	case *types.RuleGroup:
		e := &grammar.Group{}
		e.Expr = c.rule(n.Expr)
		c.span(&e.SourceInfo, n)
		return e
	case *types.InlineAction:
		e := &grammar.InlineAction{}
		e.Expr = c.rule(n.Expr)
		if !isNil(n.Action) {
			e.Action, e.DefaultAction = c.actionOrDefault(n.Action)
		}
		c.created(c.span(&e.SourceInfo, n), c.nextToken(end(n.Expr.(ranged))))
		return e
	case *types.Sequence:
		e := &grammar.Seq{}
		for _, o := range flattenSeq(n) {
			e.Items = append(e.Items, c.rule(o))
		}
		c.span(&e.SourceInfo, n)
		return e
	case *types.Choice:
		e := &grammar.Choice{}
		for _, o := range flattenChoice(n) {
			e.Alts = append(e.Alts, c.rule(o))
		}
		c.span(&e.SourceInfo, n)
		return e
	case *types.Discard:
		e := &grammar.Discard{}
		e.Expr = c.rule(n.X)
		c.span(&e.SourceInfo, n)
		return e
	case *types.Lookahead:
		x := c.rule(n.X)
		if n.Op == "&" {
			e := &grammar.And{Expr: x}
			c.span(&e.SourceInfo, n)
			return e
		}
		e := &grammar.Not{Expr: x}
		c.span(&e.SourceInfo, n)
		return e
	case *types.Capture:
		e := &grammar.Capture{Name: c.name(n.Name)}
		e.Expr = c.rule(n.Expr)
		c.span(&e.SourceInfo, n)
		return e
	case *types.Top:
		e := &grammar.Top{}
		c.span(&e.SourceInfo, n)
		return e
	case *types.Bottom:
		e := &grammar.Bottom{}
		c.span(&e.SourceInfo, n)
		return e
	case *types.BeginLine:
		e := &grammar.BeginLine{}
		c.span(&e.SourceInfo, n)
		return e
	case *types.BeginInput:
		e := &grammar.BeginInput{}
		c.span(&e.SourceInfo, n)
		return e
	case *types.EndLine:
		e := &grammar.EndLine{}
		c.span(&e.SourceInfo, n)
		return e
	case *types.EndInput:
		e := &grammar.EndInput{}
		c.span(&e.SourceInfo, n)
		return e
	case *types.Any:
		e := &grammar.Any{}
		c.span(&e.SourceInfo, n)
		return e
	case *types.Cut:
		e := &grammar.Cut{}
		c.span(&e.SourceInfo, n)
		return e
	case *types.CharClass:
		if c.bracketBefore(n.Start) {
			c.errs.add(c.pos.At(n.Start), "a character class directly after a name, a capture, ')' or ']' is read as an index; write a space before it")
		}
		e := c.charClass(n)
		c.span(&e.SourceInfo, n)
		return e
	case *types.String:
		e := c.literal(n)
		c.span(&e.SourceInfo, n)
		return e
	case *types.Optional:
		e := &grammar.Optional{}
		e.Expr = c.rule(n.X)
		c.span(&e.SourceInfo, n)
		return e
	case *types.ZeroOrMore:
		e := &grammar.Repeat{Min: 0, Max: -1}
		e.Expr = c.rule(n.X)
		c.span(&e.SourceInfo, n)
		return e
	case *types.OneOrMore:
		e := &grammar.Repeat{Min: 1, Max: -1}
		e.Expr = c.rule(n.X)
		c.span(&e.SourceInfo, n)
		return e
	case *types.Repeat:
		e := &grammar.Repeat{}
		e.Expr = c.rule(n.X)
		if !n.Range_ {
			e.Min = c.integer(n.Min)
			e.Max = e.Min
		} else {
			e.Max = -1
			if n.Min != nil {
				e.Min = c.integer(n.Min)
			}
			if n.Max != nil {
				e.Max = c.integer(n.Max)
			}
		}
		c.pragma(&e.SourceInfo, "Repeat", n, n.X)
		return e
	case *types.Atomic:
		e := &grammar.Atomic{}
		e.Expr = c.rule(n.X)
		c.pragma(&e.SourceInfo, "Atomic", n, n.X)
		return e
	case *types.Stream:
		e := &grammar.Stream{}
		e.Expr = c.rule(n.X)
		c.pragma(&e.SourceInfo, "Stream", n, n.X)
		return e
	case *types.Recover:
		e := &grammar.Recovery{}
		e.Expr = c.rule(n.X)
		e.Skip = c.rule(n.Skip)
		c.pragma(&e.SourceInfo, "Recovery", n, n.X)
		return e
	case *types.Attributed, *types.MissingPragma, *types.IgnoreCase:
		return c.attributed(n)
	case *types.Predicate:
		return c.predicate(n)
	case *types.OperatorsRule:
		return c.operators(n)
	}
	c.fail("unexpected rule expression %T", n)
	return nil
}

// The flatten functions return the operands of a chain of binary nodes of one
// type, which the public AST holds in one node.

func flattenSeq(n types.RuleExpr) []types.RuleExpr {
	if s, ok := n.(*types.Sequence); ok && s != nil {
		return append(flattenSeq(s.L), s.R)
	}
	return []types.RuleExpr{n}
}

func flattenChoice(n types.RuleExpr) []types.RuleExpr {
	if s, ok := n.(*types.Choice); ok && s != nil {
		return append(flattenChoice(s.L), s.R)
	}
	return []types.RuleExpr{n}
}

// bracketBefore reports whether the "[" at off follows the end of a name, a
// capture or ")" without a space. The hand-written lexer reads it as an index
// bracket, which a parser expression does not have. A "]" before it ends a
// character class, so a class may follow it directly.
func (c *canonical) bracketBefore(off int) bool {
	if off == 0 {
		return false
	}
	switch r := c.src[off-1]; {
	case r == ')':
		return true
	case isIdentPart(r):
		start := off
		for start > 0 && isIdentPart(c.src[start-1]) {
			start--
		}
		if start == off-1 && start >= 2 && c.src[start-1] == '|' && c.src[start-2] == '_' {
			return false // the token _|_
		}
		// The lexer reads leading digits as a number and what follows them as a
		// name, so only a run of digits is not a name. The suffix i of a string
		// is a name to the lexer too: "x"i[a] is an index.
		for start < off && c.src[start] >= '0' && c.src[start] <= '9' {
			start++
		}
		return start < off
	}
	return false
}

// charClass builds a character class from its source.
func (c *canonical) charClass(n *types.CharClass) *grammar.CharClass {
	var cc *grammar.CharClass
	c.decode(n, func(l *lexer) { cc = l.charClass() })
	return &grammar.CharClass{Ranges: cc.Ranges, Negated: cc.Negated}
}

// pragma records a node that a pragma completes twice: at the "#" and then,
// once the suffix loop has it, from the start of its operand.
func (c *canonical) pragma(target *grammar.SourceInfo, label string, n ranged, operand types.RuleExpr) int {
	if isNil(operand) {
		c.fail("the source tree has no operand of a pragma")
	}
	os, oe := operand.(ranged).Range()
	hash := c.nextToken(oe)
	i := c.add(target,
		sourceast.Event{Start: hash, End: end(n)},
		sourceast.Event{Start: os, End: end(n)})
	if label == "Repeat" {
		return c.created(i, os) // #repeat starts at its operand
	}
	return c.created(i, hash)
}

// layer returns the operand of a layer of a run of attributes and bare
// pragmas, and whether n is one: an attribute, or a pragma without arguments,
// which the source tree records as a layer of its own.
func layer(n types.RuleExpr) (types.RuleExpr, bool) {
	switch n := n.(type) {
	case *types.Attributed:
		return n.X, n != nil
	case *types.MissingPragma:
		return n.X, n != nil
	case *types.IgnoreCase:
		return n.X, n != nil
	}
	return nil, false
}

// attributed builds the one node that a run of attributes and bare pragmas
// becomes. It is completed again by every suffix after the first.
func (c *canonical) attributed(n types.RuleExpr) *grammar.Attributed {
	var layers []types.RuleExpr // outermost first
	base := n
	for {
		x, ok := layer(base)
		if !ok {
			break
		}
		layers = append(layers, base)
		base = x
	}
	if isNil(base) {
		c.fail("the source tree has no operand of an attribute")
	}
	baseStart := start(base.(ranged))
	a := &grammar.Attributed{}
	a.Expr = c.rule(base)
	var events []sourceast.Event
	for i := len(layers) - 1; i >= 0; i-- {
		l := layers[i]
		if _, attr := l.(*types.Attributed); !attr {
			x, _ := layer(l)
			hash := c.nextToken(end(x.(ranged)))
			pragma := &grammar.Attribute{Name: "missing", Pragma: true}
			if _, ok := l.(*types.IgnoreCase); ok {
				pragma.Name = "ignorecase"
			}
			pragma.Pos = c.pos.At(hash)
			a.Attrs = append(a.Attrs, pragma)
			events = append(events,
				sourceast.Event{Start: hash, End: end(l.(ranged))},
				sourceast.Event{Start: baseStart, End: end(l.(ranged))})
			continue
		}
		// The suffix loop reads the whole run of attributes before it completes the node.
		run := l.(*types.Attributed)
		for ; i > 0; i-- {
			next, ok := layers[i-1].(*types.Attributed)
			if !ok {
				break
			}
			a.Attrs = append(a.Attrs, c.attr(run.Attr))
			run = next
		}
		a.Attrs = append(a.Attrs, c.attr(run.Attr))
		events = append(events, sourceast.Event{Start: baseStart, End: run.End})
	}
	c.add(&a.SourceInfo, events...)
	return a
}

func (c *canonical) attr(n *types.Attr) *grammar.Attribute {
	if n == nil {
		c.fail("the source tree has a syntax error in an attribute")
	}
	a := &grammar.Attribute{Name: c.name(n.Name)}
	for _, v := range n.Args {
		a.Args = append(a.Args, c.attrValue(v))
	}
	for _, kw := range n.KwArgs {
		if kw == nil {
			c.fail("the source tree has a syntax error in an attribute")
		}
		name := c.name(kw.Name)
		v := c.attrValue(kw.Value)
		if a.KwArgs == nil {
			a.KwArgs = map[string]any{}
		}
		if _, exists := a.KwArgs[name]; exists {
			at := c.pos.At(c.nextToken(end(kw.Value.(ranged))))
			c.errs.add(at, "duplicate attribute argument "+name)
		}
		a.KwArgs[name] = v
		a.KeywordOrder = append(a.KeywordOrder, name)
	}
	c.span(&a.SourceInfo, n)
	return a
}

// attrValue builds an argument. Numbers and keywords are read without a node,
// and a string is completed as a literal that the attribute then holds as its
// value.
func (c *canonical) attrValue(v types.AttrValue) any {
	if isNil(v) {
		return nil
	}
	switch v := v.(type) {
	case *types.Int:
		return c.integer(v)
	case *types.True:
		return true
	case *types.False:
		return false
	case *types.Nil:
		return nil
	case *types.String:
		lit := c.literal(v)
		c.span(nil, v)
		return lit.Value
	}
	x, ok := v.(types.RuleExpr)
	if !ok {
		c.fail("unexpected attribute argument %T", v)
	}
	return c.rule(x)
}

func (c *canonical) operators(n *types.OperatorsRule) *grammar.OperatorsRule {
	r := &grammar.OperatorsRule{}
	for _, it := range n.Items {
		switch it := it.(type) {
		case *types.OperatorsSkip:
			sep := c.semicolons(it.SourceSpan)
			skip := c.rule(it.Expr)
			if r.Skip != nil {
				c.errs.add(c.pos.At(start(it.Expr.(ranged))), "duplicate skip")
			}
			r.Skip = skip
			c.statement(&r.SkipSource, it.SourceSpan, sep, false)
		case *types.OperatorsOperand:
			sep := c.semicolons(it.SourceSpan)
			o := &grammar.OperatorsRuleOperand{}
			o.Expr = c.rule(it.Expr)
			if !isNil(it.Action) {
				o.Action, o.DefaultAction = c.actionOrDefault(it.Action)
			}
			c.statement(&o.SourceInfo, it.SourceSpan, sep, false)
			r.Operands = append(r.Operands, o)
		case *types.OperatorsLevel:
			sep := c.semicolons(it.SourceSpan)
			l := &grammar.OperatorsRuleLevel{}
			if it.Name != nil {
				l.Name = it.Name.Value // a level may have no name
			}
			for _, op := range it.Operators {
				o := &grammar.OperatorsRuleOperator{}
				var body *types.OperatorsOperatorBody
				var expr types.RuleExpr
				var action types.Action
				switch op := op.(type) {
				case *types.Prefix:
					o.Kind, body, expr, action = grammar.Prefix, op.SourceSpan, op.Expr, op.Action
				case *types.Postfix:
					o.Kind, body, expr, action = grammar.Postfix, op.SourceSpan, op.Expr, op.Action
				case *types.Infix:
					o.Kind, body, expr, action = grammar.Infix, op.SourceSpan, op.Expr, op.Action
					o.Assoc = op.Assoc
				}
				if body == nil {
					c.fail("the source tree has a syntax error in an Operators Rule")
				}
				o.Expr = c.rule(expr)
				if !isNil(action) {
					o.Action, o.DefaultAction = c.actionOrDefault(action)
				}
				c.statement(&o.SourceInfo, body, c.separators(body.End, ';', 1<<30), false)
				l.Operators = append(l.Operators, o)
			}
			c.statement(&l.SourceInfo, it.SourceSpan, sep, false)
			r.Levels = append(r.Levels, l)
		default:
			c.fail("the source tree has a syntax error in an Operators Rule")
		}
	}
	c.created(c.span(&r.SourceInfo, n), c.nextToken(n.Start+len("#operators")))
	return r
}
