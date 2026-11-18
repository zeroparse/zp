package compile

import (
	"zeroparse.dev/zp/internal/frontend/check"
	"zeroparse.dev/zp/internal/runtime/program"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// Runtime returns the run-time program of p: the tables that a parser runs, without the grammar
// or its types. It is made once, on first use, from the checked program, which is not changed
// after Check.
func (p *Program) Runtime() *program.Program {
	p.runtimeOnce.Do(func() { p.runtime = p.newRuntime() })
	return p.runtime
}

func (p *Program) newRuntime() *program.Program {
	rt := &program.Program{
		ByName:            make(map[string]*program.Rule, len(p.RuleList)),
		DescTable:         p.DescTable,
		NSeen:             p.NSeen,
		Vars:              runtimeVars(p.Vars),
		StartRules:        p.StartRules,
		DefaultStartRule:  p.DefaultStartRule,
		HasMissing:        p.HasMissing,
		RecordsSourceInfo: p.RecordsSourceInfo,
		MakesDefaultValue: p.MakesDefaultValue,
		Types:             map[string]*program.Type{},
		TypeKinds:         p.typeKinds,
		NodeKinds:         p.NodeKinds,
		NumSkipSites:      p.NumSkipSites,
		NumRunSites:       p.NumRunSites,
		NumScanSites:      p.NumScanSites,
		EnvFreeSites:      p.envFree,
		InsertionCycles:   p.InsertionFlags != nil,
		StructZero: func(name string) *program.StructZero {
			if z := p.StructZeroOf(name); z != nil {
				return &program.StructZero{Names: z.Names, Zeros: z.Zeros}
			}
			return nil
		},
	}
	for name, decl := range p.Types {
		if td, ok := decl.(*ast.TypeDef); ok {
			t := &program.Type{Name: name}
			for _, f := range check.DefFields(td) {
				t.Fields = append(t.Fields, f.Name)
			}
			rt.Types[name] = t
		}
	}
	for _, r := range p.RuleList {
		x := RuntimeRule(r)
		rt.RuleList = append(rt.RuleList, x)
		rt.ByName[x.Name] = x
	}
	for _, r := range p.Twins {
		rt.Twins = append(rt.Twins, RuntimeRule(r))
	}
	return rt
}

// RuntimeRule returns the run-time facts of the rule r.
func RuntimeRule(r *Rule) *program.Rule {
	return &program.Rule{
		Where: r.Where, Splice: r.Splice, ID: r.ID, Name: r.Name, Scope: RuntimeScope(r.Scope),
		NodeType: r.NodeType, OpNodeType: r.OpNodeType, Memo: r.Memo, Missing: r.Missing,
		Leader: r.Leader, Member: r.Member, Cycle: r.Cycle, LeaderIndex: r.LeaderIndex,
		LeaderBit: r.LeaderBit, Stream: r.Stream, Positional: r.Positional, Vars: r.Vars,
		Transient: r.Transient, Seen: r.Seen, Plain: r.Plain, Rec: r.Rec, Novalue: r.Novalue,
	}
}

// RuntimeScope returns the run-time scope of the capture scope s, which shares its slots.
func RuntimeScope(s *check.Scope) *program.Scope {
	if s == nil {
		return nil
	}
	return &program.Scope{Names: s.Names, Slots: s.Slots}
}

// runtimeVars returns the run-time table of the variables of t.
func runtimeVars(t *check.VarTable) *program.VarTable {
	var decls []*program.VarDecl
	for _, d := range t.All() {
		decls = append(decls, &program.VarDecl{Name: d.Name, Init: d.Init})
	}
	return program.NewVarTable(decls)
}
