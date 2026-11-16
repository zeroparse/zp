package compile

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/frontend/check"
	"zeroparse.dev/zp/internal/frontend/desugar"
	"zeroparse.dev/zp/internal/frontend/elaborate"
)

// StructuralInput returns the input of the untyped Core IR module (elaborate.Structural) of rules,
// the rules of g once each, as UniqueRules gives them: rule i has ID i. starts names the start
// rules in order; a name that is not a rule is left out. The variables are those g declares,
// by name only.
func StructuralInput(g *grammar.Grammar, rules []*grammar.RuleDef, starts []string) *elaborate.Input {
	in := &elaborate.Input{ByName: map[string]*elaborate.Rule{}}
	for i, rd := range rules {
		_, operators := rd.Expr.(*grammar.OperatorsRule)
		r := &elaborate.Rule{ID: i, Name: rd.Name, Def: rd, Body: rd.Expr, Action: rd.Action,
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
