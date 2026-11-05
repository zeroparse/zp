package syntax

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/syntax/sourceast"
	"zeroparse.dev/zp/internal/value"
)

// Parser expressions. The nodes of the AST are built, and recorded for source
// information, in the order the hand-written parser completes them.

func (c *canonical) rule(n *value.Node) grammar.Expr {
	switch n.Type() {
	case "RuleRef":
		e := &grammar.Ref{Name: textField(n, "Name")}
		c.span(&e.SourceInfo, n)
		return e
	case "LevelCall":
		e := &grammar.Ref{Name: textField(n, "Rule"), Level: textField(n, "Level")}
		c.created(c.span(&e.SourceInfo, n), nodeField(n, "Rule").Start())
		return e
	case "RuleGroup":
		e := &grammar.Group{}
		e.Expr = c.rule(nodeField(n, "Expr"))
		c.span(&e.SourceInfo, n)
		return e
	case "InlineAction":
		e := &grammar.InlineAction{}
		e.Expr = c.rule(nodeField(n, "Expr"))
		if a := nodeField(n, "Action"); a != nil {
			e.Action, e.DefaultAction = c.actionOrDefault(a)
		}
		c.created(c.span(&e.SourceInfo, n), c.nextToken(nodeField(n, "Expr").End()))
		return e
	case "Sequence":
		e := &grammar.Seq{}
		for _, o := range flatten(n, "Sequence") {
			e.Items = append(e.Items, c.rule(o))
		}
		c.span(&e.SourceInfo, n)
		return e
	case "Choice":
		e := &grammar.Choice{}
		for _, o := range flatten(n, "Choice") {
			e.Alts = append(e.Alts, c.rule(o))
		}
		c.span(&e.SourceInfo, n)
		return e
	case "Discard":
		e := &grammar.Discard{}
		e.Expr = c.rule(nodeField(n, "X"))
		c.span(&e.SourceInfo, n)
		return e
	case "Lookahead":
		x := c.rule(nodeField(n, "X"))
		if stringField(n, "Op") == "&" {
			e := &grammar.And{Expr: x}
			c.span(&e.SourceInfo, n)
			return e
		}
		e := &grammar.Not{Expr: x}
		c.span(&e.SourceInfo, n)
		return e
	case "Capture":
		e := &grammar.Capture{Name: textField(n, "Name")}
		e.Expr = c.rule(nodeField(n, "Expr"))
		c.span(&e.SourceInfo, n)
		return e
	case "Top":
		e := &grammar.Top{}
		c.span(&e.SourceInfo, n)
		return e
	case "Bottom":
		e := &grammar.Bottom{}
		c.span(&e.SourceInfo, n)
		return e
	case "BeginLine":
		e := &grammar.BeginLine{}
		c.span(&e.SourceInfo, n)
		return e
	case "BeginInput":
		e := &grammar.BeginInput{}
		c.span(&e.SourceInfo, n)
		return e
	case "EndLine":
		e := &grammar.EndLine{}
		c.span(&e.SourceInfo, n)
		return e
	case "EndInput":
		e := &grammar.EndInput{}
		c.span(&e.SourceInfo, n)
		return e
	case "Any":
		e := &grammar.Any{}
		c.span(&e.SourceInfo, n)
		return e
	case "Cut":
		e := &grammar.Cut{}
		c.span(&e.SourceInfo, n)
		return e
	case "CharClass":
		e := c.charClass(n)
		c.span(&e.SourceInfo, n)
		return e
	case "String":
		e := c.literal(n)
		c.span(&e.SourceInfo, n)
		return e
	case "Optional":
		e := &grammar.Optional{}
		e.Expr = c.rule(nodeField(n, "X"))
		c.span(&e.SourceInfo, n)
		return e
	case "ZeroOrMore", "OneOrMore":
		e := &grammar.Repeat{Min: 0, Max: -1}
		if n.Type() == "OneOrMore" {
			e.Min = 1
		}
		e.Expr = c.rule(nodeField(n, "X"))
		c.span(&e.SourceInfo, n)
		return e
	case "Repeat":
		x := nodeField(n, "X")
		e := &grammar.Repeat{}
		e.Expr = c.rule(x)
		if !boolField(n, "Range") {
			e.Min = c.integer(nodeField(n, "Min"))
			e.Max = e.Min
		} else {
			e.Max = -1
			if m := nodeField(n, "Min"); m != nil {
				e.Min = c.integer(m)
			}
			if m := nodeField(n, "Max"); m != nil {
				e.Max = c.integer(m)
			}
		}
		c.pragma(&e.SourceInfo, "Repeat", n, x)
		return e
	case "Atomic":
		x := nodeField(n, "X")
		e := &grammar.Atomic{}
		e.Expr = c.rule(x)
		c.pragma(&e.SourceInfo, "Atomic", n, x)
		return e
	case "Stream":
		x := nodeField(n, "X")
		e := &grammar.Stream{}
		e.Expr = c.rule(x)
		c.pragma(&e.SourceInfo, "Stream", n, x)
		return e
	case "Recover":
		x := nodeField(n, "X")
		e := &grammar.Recovery{}
		e.Expr = c.rule(x)
		e.Skip = c.rule(nodeField(n, "Skip"))
		c.pragma(&e.SourceInfo, "Recovery", n, x)
		return e
	case "Attributed", "MissingPragma", "IgnoreCase":
		return c.attributed(n)
	case "Predicate":
		return c.predicate(n)
	case "OperatorsRule":
		return c.operators(n)
	}
	c.fail("unexpected rule expression %s", n.Type())
	return nil
}

// charClass builds a character class from its source.
func (c *canonical) charClass(n *value.Node) *grammar.CharClass {
	var cc *grammar.CharClass
	c.decode(n, func(l *lexer) { cc = l.charClass() })
	return &grammar.CharClass{Ranges: cc.Ranges, Negated: cc.Negated}
}

// pragma records a node that a pragma completes twice: at the "#" and then,
// once the suffix loop has it, from the start of its operand.
func (c *canonical) pragma(target *grammar.SourceInfo, label string, n, operand *value.Node) int {
	hash := c.nextToken(operand.End())
	i := c.add(target,
		sourceast.Event{Start: hash, End: n.End()},
		sourceast.Event{Start: operand.Start(), End: n.End()})
	if label == "Repeat" {
		return c.created(i, operand.Start()) // #repeat starts at its operand
	}
	return c.created(i, hash)
}

// bareLayer reports a pragma without arguments, which the source tree records
// as a layer of its own.
func bareLayer(n *value.Node) bool {
	return n.Type() == "MissingPragma" || n.Type() == "IgnoreCase"
}

// attributed builds the one node that a run of attributes and bare pragmas
// becomes. It is completed again by every suffix after the first.
func (c *canonical) attributed(n *value.Node) *grammar.Attributed {
	var layers []*value.Node // outermost first
	base := n
	for base.Type() == "Attributed" || bareLayer(base) {
		layers = append(layers, base)
		base = nodeField(base, "X")
	}
	a := &grammar.Attributed{}
	a.Expr = c.rule(base)
	var events []sourceast.Event
	for i := len(layers) - 1; i >= 0; i-- {
		layer := layers[i]
		if bareLayer(layer) {
			hash := c.nextToken(nodeField(layer, "X").End())
			pragma := &grammar.Attribute{Name: "missing", Pragma: true}
			if layer.Type() == "IgnoreCase" {
				pragma.Name = "ignorecase"
			}
			pragma.Pos = c.pos.At(hash)
			a.Attrs = append(a.Attrs, pragma)
			events = append(events,
				sourceast.Event{Start: hash, End: layer.End()},
				sourceast.Event{Start: base.Start(), End: layer.End()})
			continue
		}
		// The suffix loop reads the whole run of attributes before it completes the node.
		run := layer
		for ; i > 0 && layers[i-1].Type() == "Attributed"; i-- {
			a.Attrs = append(a.Attrs, c.attr(nodeField(run, "Attr")))
			run = layers[i-1]
		}
		a.Attrs = append(a.Attrs, c.attr(nodeField(run, "Attr")))
		events = append(events, sourceast.Event{Start: base.Start(), End: run.End()})
	}
	c.add(&a.SourceInfo, events...)
	return a
}

func (c *canonical) attr(n *value.Node) *grammar.Attribute {
	a := &grammar.Attribute{Name: textField(n, "Name")}
	for _, v := range listField(n, "Args") {
		a.Args = append(a.Args, c.attrValue(v))
	}
	for _, kw := range listField(n, "KwArgs") {
		name := textField(kw, "Name")
		value := nodeField(kw, "Value")
		v := c.attrValue(value)
		if a.KwArgs == nil {
			a.KwArgs = map[string]any{}
		}
		if _, exists := a.KwArgs[name]; exists {
			at := c.pos.At(c.nextToken(value.End()))
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
func (c *canonical) attrValue(v *value.Node) any {
	if v == nil {
		return nil
	}
	switch v.Type() {
	case "Int":
		return c.integer(v)
	case "True":
		return true
	case "False":
		return false
	case "Nil":
		return nil
	case "String":
		lit := c.literal(v)
		c.span(nil, v)
		return lit.Value
	}
	return c.rule(v)
}

func (c *canonical) operators(n *value.Node) *grammar.OperatorsRule {
	r := &grammar.OperatorsRule{}
	for _, it := range listField(n, "Items") {
		span := nodeField(it, "SourceSpan")
		if span == nil {
			c.fail("the source tree has a syntax error in an Operators Rule")
		}
		sep := c.separators(span.End(), ';', 1<<30)
		switch it.Type() {
		case "OperatorsSkip":
			expr := nodeField(it, "Expr")
			skip := c.rule(expr)
			if r.Skip != nil {
				c.errs.add(c.pos.At(expr.Start()), "duplicate skip")
			}
			r.Skip = skip
			c.statement(&r.SkipSource, it, sep, false)
		case "OperatorsOperand":
			o := &grammar.OperatorsRuleOperand{}
			o.Expr = c.rule(nodeField(it, "Expr"))
			if a := nodeField(it, "Action"); a != nil {
				o.Action, o.DefaultAction = c.actionOrDefault(a)
			}
			c.statement(&o.SourceInfo, it, sep, false)
			r.Operands = append(r.Operands, o)
		case "OperatorsLevel":
			l := &grammar.OperatorsRuleLevel{Name: textField(it, "Name")}
			for _, op := range listField(it, "Operators") {
				ospan := nodeField(op, "SourceSpan")
				if ospan == nil {
					c.fail("the source tree has a syntax error in an Operators Rule")
				}
				o := &grammar.OperatorsRuleOperator{Assoc: stringField(op, "Assoc")}
				switch op.Type() {
				case "Prefix":
					o.Kind = grammar.Prefix
				case "Postfix":
					o.Kind = grammar.Postfix
				default:
					o.Kind = grammar.Infix
				}
				o.Expr = c.rule(nodeField(op, "Expr"))
				if a := nodeField(op, "Action"); a != nil {
					o.Action, o.DefaultAction = c.actionOrDefault(a)
				}
				c.statement(&o.SourceInfo, op, c.separators(ospan.End(), ';', 1<<30), false)
				l.Operators = append(l.Operators, o)
			}
			c.statement(&l.SourceInfo, it, sep, false)
			r.Levels = append(r.Levels, l)
		default:
			c.fail("unexpected operators item %s", it.Type())
		}
	}
	c.created(c.span(&r.SourceInfo, n), c.nextToken(n.Start()+len("#operators")))
	return r
}
