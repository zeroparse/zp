package compile

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/check"
	"zeroparse.dev/zp/internal/desugar"
	"zeroparse.dev/zp/internal/ir/core/build"
	"zeroparse.dev/zp/internal/ir/core/facts"
)

// StructuralInput returns the input of the untyped Core IR module (build.Structural) of rules,
// the rules of g once each, as UniqueRules gives them: rule i has ID i. starts names the start
// rules in order; a name that is not a rule is left out. The variables are those g declares,
// by name only.
func StructuralInput(g *grammar.Grammar, rules []*grammar.RuleDef, starts []string) *build.Input {
	in := &build.Input{ByName: map[string]*build.Rule{}}
	for i, rd := range rules {
		_, operators := rd.Expr.(*grammar.OperatorsRule)
		r := &build.Rule{ID: i, Name: rd.Name, Def: rd, Body: rd.Expr, Action: rd.Action,
			Splice: desugar.HiddenRule(rd.Name) && rd.Action == nil && !rd.DefaultAction && !operators}
		in.Rules = append(in.Rules, r)
		in.ByName[rd.Name] = r
	}
	for _, name := range starts {
		if r := in.ByName[name]; r != nil {
			in.Starts = append(in.Starts, r)
		}
	}
	for _, d := range g.File.Decls {
		if vd, ok := d.(*grammar.VarDef); ok {
			if in.Vars == nil {
				in.Vars = check.NewVarTable()
			}
			if in.Vars.Lookup(vd.Name) == nil {
				in.Vars.Add(&check.VarDecl{Name: vd.Name})
			}
		}
	}
	return in
}

// structuralFacts returns the facts of the untyped Core IR module of rules, the unique rules of
// g, whose start rules are those marked @start. Rule i of rules has ID i in the module.
func structuralFacts(g *grammar.Grammar, rules []*grammar.RuleDef, control *cancel.Control) (*facts.Facts, error) {
	var starts []string
	for _, rd := range rules {
		if desugar.StartMarked(rd) {
			starts = append(starts, rd.Name)
		}
	}
	m, err := build.Structural(StructuralInput(g, rules, starts), nil)
	if err != nil {
		return nil, err
	}
	return facts.Compute(m, control), nil
}
