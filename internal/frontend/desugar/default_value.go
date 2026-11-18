package desugar

import (
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// DefaultValueGrammar returns g without the value actions and without the declared types of rules,
// so that every rule makes the default value of its expression (the compile option DefaultValue).
// Predicates stay as written, because they decide the match, and so do the type declarations,
// which predicates can still use. A predicate can read the value of a capture, which an action may
// have made, so the rules in needed, which make the values predicates read, keep their actions and
// types. Unchanged declarations are shared with g; g itself does not change.
func DefaultValueGrammar(g *ast.Grammar, needed map[string]bool, control *cancel.Control) *ast.Grammar {
	decls, changed := normalizeItems(g.File.Decls, false, func(d ast.Decl) ast.Decl {
		control.Poll()
		rd, ok := d.(*ast.RuleDef)
		if !ok || needed[rd.Name] {
			return d
		}
		expr := defaultValueExpr(rd.Expr, control)
		if rd.Action == nil && rd.Type == nil && !rd.DefaultAction && expr == rd.Expr {
			return rd
		}
		rule := *rd
		rule.Expr, rule.Type, rule.Action, rule.DefaultAction = expr, nil, nil, false
		if HiddenRule(rd.Name) && (rd.Action != nil || rd.DefaultAction) {
			// A hidden rule without an action splices into its caller; one with an action makes
			// its value. Its default value keeps that node, as its action did.
			rule.DefaultAction = true
		}
		return &rule
	}, control)
	if !changed {
		return g
	}
	copy := *g
	copy.File.Decls = decls
	return &copy
}

// SourceDefaultValueGrammar returns the grammar as written, source, with what DefaultValueGrammar
// dropped from its lowered form: dv is DefaultValueGrammar(lowered), and names the inline actions
// that lowered was made with. Validating it reports a defect where the grammar as written has it,
// with the paths and positions of that grammar, not of the lowered one, whose declarations are
// numbered differently and whose generated rules the author never wrote.
func SourceDefaultValueGrammar(source, lowered, dv *ast.Grammar, names InlineNames, control *cancel.Control) *ast.Grammar {
	dvRules := map[string]*ast.RuleDef{}
	for _, rd := range control.Rules(dv) {
		dvRules[rd.Name] = rd
	}
	// An inline action keeps its action when the rule it was lowered to does.
	keep := map[*ast.InlineAction]bool{}
	for name, site := range names {
		if rd := dvRules[name]; rd != nil && rd.Action != nil {
			keep[site.node] = true
		}
	}
	walk := &inlineLowerer{control: control, keepInline: func(e *ast.InlineAction, inner ast.Expr) ast.Expr {
		if keep[e] && inner == e.Expr {
			return e
		}
		copy := *e
		copy.Expr = inner
		if !keep[e] {
			copy.Action, copy.DefaultAction = nil, true
		}
		return &copy
	}}
	// Each declaration of the source is followed in the lowered grammar by the rules generated
	// from its inline actions.
	decls := make([]ast.Decl, len(source.File.Decls))
	j := 0
	for i, d := range source.File.Decls {
		control.Poll()
		decls[i] = d
		lowRule, _ := lowered.File.Decls[j].(*ast.RuleDef)
		dvRule, _ := dv.File.Decls[j].(*ast.RuleDef)
		for j++; j < len(lowered.File.Decls); j++ {
			generated, ok := lowered.File.Decls[j].(*ast.RuleDef)
			if !ok || names[generated.Name].node == nil {
				break
			}
		}
		rd, isRule := d.(*ast.RuleDef)
		if !isRule || lowRule == nil || dvRule == nil {
			continue
		}
		expr := walk.expr(rd.Expr)
		if dvRule != lowRule {
			expr = defaultValueExpr(expr, control)
		}
		if expr == rd.Expr && dvRule.Action == rd.Action && dvRule.Type == rd.Type && dvRule.DefaultAction == rd.DefaultAction {
			continue
		}
		rule := *rd
		rule.Expr, rule.Type, rule.Action, rule.DefaultAction = expr, dvRule.Type, dvRule.Action, dvRule.DefaultAction
		decls[i] = &rule
	}
	copy := *source
	copy.File.Decls = decls
	return &copy
}

// defaultValueExpr drops the actions of the Operators Rule at the root of a rule body, the only
// expression that has actions of its own (inline actions are lowered to rules before this runs).
func defaultValueExpr(e ast.Expr, control *cancel.Control) ast.Expr {
	switch e := e.(type) {
	case *ast.Group:
		inner := defaultValueExpr(e.Expr, control)
		if inner == e.Expr {
			return e
		}
		copy := *e
		copy.Expr = inner
		return &copy
	case *ast.OperatorsRule:
		operands, operandsChanged := normalizeItems(e.Operands, false, func(o *ast.OperatorsRuleOperand) *ast.OperatorsRuleOperand {
			control.Poll()
			if o.Action == nil && !o.DefaultAction {
				return o
			}
			copy := *o
			copy.Action, copy.DefaultAction = nil, false
			return &copy
		}, control)
		levels, levelsChanged := normalizeItems(e.Levels, false, func(l *ast.OperatorsRuleLevel) *ast.OperatorsRuleLevel {
			operators, changed := normalizeItems(l.Operators, false, func(o *ast.OperatorsRuleOperator) *ast.OperatorsRuleOperator {
				control.Poll()
				if o.Action == nil && !o.DefaultAction {
					return o
				}
				copy := *o
				copy.Action, copy.DefaultAction = nil, false
				return &copy
			}, control)
			if !changed {
				return l
			}
			copy := *l
			copy.Operators = operators
			return &copy
		}, control)
		if !operandsChanged && !levelsChanged {
			return e
		}
		copy := *e
		copy.Operands, copy.Levels = operands, levels
		return &copy
	}
	return e
}
