package complexity

import (
	"fmt"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/ir/core"
)

// Grammar returns the rules of the view as grammar rules: rules are the rules the source module
// is built from, rule i of the module from rules[i]. A node the view shares with the module is
// the expression it is built from, and a node the view builds is written in the grammar's syntax,
// so that the formatted rules can be compared with the normal view of the grammar and the view
// can run on an engine that reads grammars. It is a bridge for tests until the engines run Core
// IR.
func (v *View) Grammar(rules []*grammar.RuleDef) ([]*grammar.RuleDef, error) {
	if len(rules) != len(v.Module.Rules) {
		return nil, fmt.Errorf("%d rules for a module of %d", len(rules), len(v.Module.Rules))
	}
	out := make([]*grammar.RuleDef, len(rules))
	for i, rd := range rules {
		body, err := v.expr(v.Module.Rules[i].Body)
		if err != nil {
			return nil, fmt.Errorf("rule %s: %w", rd.Name, err)
		}
		if body == rd.Expr {
			out[i] = rd
			continue
		}
		cp := *rd
		cp.Expr = body
		out[i] = &cp
	}
	return out, nil
}

// expr returns the grammar expression of the node n of the view.
func (v *View) expr(n core.Node) (grammar.Expr, error) {
	if e, ok := v.Origin[n]; ok {
		return e, nil
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
