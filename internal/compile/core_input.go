package compile

import "zeroparse.dev/zp/internal/ir/core/build"

// CoreInput returns the rules and declarations of the program as the Core IR builder reads them
// (build.Program).
func (p *Program) CoreInput() *build.Input {
	in := &build.Input{Types: p.Types, Vars: p.Vars, ByName: map[string]*build.Rule{}}
	all := append(append([]*Rule(nil), p.RuleList...), p.Twins...)
	in.Rules = make([]*build.Rule, len(all))
	byRule := make(map[*Rule]*build.Rule, len(all))
	for i, r := range all {
		br := &build.Rule{ID: r.ID, Name: r.Name, Where: r.Where, Def: r.Def, Body: r.BodyExpr(), Action: r.Action, NodeType: r.NodeType,
			Memo: r.Memo, Transient: r.Transient, Plain: r.Plain, Rec: r.Rec, Leader: r.Leader, Splice: r.Splice, Novalue: r.Novalue,
			Lean: r.Lean, Seen: r.Seen, Vars: r.Vars}
		if r.PredCaps != nil {
			br.PredCaps = r.PredCaps
		}
		br.Projections = func() map[string]string { return RuleProjections(p, r) }
		in.Rules[i], byRule[r] = br, br
	}
	for _, r := range all {
		if r.Twin != nil {
			byRule[r].Twin = byRule[r.Twin]
		}
	}
	for _, r := range p.RuleList {
		in.ByName[r.Name] = byRule[r]
	}
	for _, name := range p.StartRules {
		in.Starts = append(in.Starts, in.ByName[name])
	}
	return in
}

// MissingTokens reports that the grammar uses #missing (build.Program).
func (p *Program) MissingTokens() bool { return p.HasMissing }
