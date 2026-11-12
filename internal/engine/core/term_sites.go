package core

import (
	"fmt"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	ir "zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/ir/core/build"
)

// CoreTerms builds the Core IR of the program and returns the Core IR term of each term of its
// rules: the actions of the rules and of the lines of Operators Rules, the predicates and the
// values of bindings.
func CoreTerms(prog *compile.Program) (map[grammar.Term]ir.Term, error) {
	origin := map[ir.Node]grammar.Expr{}
	m, err := build.CoreFrom(prog, origin)
	if err != nil {
		return nil, fmt.Errorf("internal error: Core IR: %w", err)
	}
	terms := map[grammar.Term]ir.Term{}
	add := func(g grammar.Term, t ir.Term) {
		if g != nil && t != nil {
			if _, ok := terms[g]; !ok {
				terms[g] = t
			}
		}
	}
	all := append(append([]*compile.Rule(nil), prog.RuleList...), prog.Twins...)
	for _, cr := range m.Rules {
		if cr == nil {
			continue
		}
		if cr.ID < len(all) && all[cr.ID] != nil {
			sr := all[cr.ID]
			add(sr.Action, cr.Finish.Action)
			if o, ok := cr.Body.(*ir.OperatorsRule); ok {
				if g, ok := sr.Def.Expr.(*grammar.OperatorsRule); ok {
					for i, l := range o.Operands {
						add(g.Operands[i].Action, l.Action)
					}
					i := 0
					for _, level := range g.Levels {
						for _, x := range level.Operators {
							add(x.Action, o.Ops[i].Line.Action)
							i++
						}
					}
				}
			}
		}
		ir.Walk(cr.Body, func(n ir.Node) {
			e, ok := origin[n].(*grammar.Predicate)
			if !ok {
				return
			}
			switch n := n.(type) {
			case *ir.Pred:
				add(e.Term, n.Term)
			case *ir.BindNode:
				if a, ok := e.Term.(*grammar.Bind); ok {
					add(a.Value, n.Term)
				}
			}
		})
	}
	return terms, nil
}
