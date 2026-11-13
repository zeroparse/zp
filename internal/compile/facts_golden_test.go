package compile

import (
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/desugar"
	"zeroparse.dev/zp/internal/factstest"
	"zeroparse.dev/zp/internal/starttest"
	"zeroparse.dev/zp/internal/syntax"
)

// TestFactsGolden pins the grammar facts that compilation computes for the repository's grammars
// (internal/factstest).
func TestFactsGolden(t *testing.T) {
	t.Parallel()
	for _, path := range factstest.Corpus {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			g, err := syntax.Parse(factstest.Read(t, path))
			if err != nil {
				t.Fatal(err)
			}
			g = starttest.MarkAll(g)
			names := desugar.InlineNames{}
			prog, err := Check(g, Options{}, nil, names, nil)
			if err != nil {
				t.Fatal(names.Rewrite(err))
			}
			factstest.Golden(t, path, factstest.Render(programFacts(prog)))
		})
	}
}

// programFacts returns the facts of the program's rules: the flags compilation set and the
// analysis it computed them from.
func programFacts(prog *Program) []factstest.Rule {
	defs := make([]*grammar.RuleDef, len(prog.RuleList))
	for i, r := range prog.RuleList {
		defs[i] = r.Def
	}
	an := analyze(defs)
	var out []factstest.Rule
	for _, r := range prog.RuleList {
		f := factstest.Rule{
			Name: r.Name, Nullable: an.Nullable[r.Name], Cyclic: an.Cyclic[r.Name], Cycle: r.Cycle,
			Leader: r.Leader, Member: r.Member, Rec: r.Rec, Positional: r.Positional,
			Recovering: an.Recovering[r.Name], Vars: r.Vars,
			Memo: r.Memo, Transient: r.Transient, Missing: r.Missing,
			First: terminalFact(firstTerminal(prog, r.Def.Expr, 0, map[string]bool{})),
		}
		f.InsCycle, f.InsLeader, f.InsMember = f.Cycle, f.Leader, f.Member
		if prog.InsertionFlags != nil {
			ins := prog.InsertionFlags[r.ID]
			f.InsCycle, f.InsLeader, f.InsMember = ins.Cycle, ins.Leader, ins.Member
		}
		WalkExpr(r.Def.Expr, func(e grammar.Expr) {
			c, ok := e.(*grammar.Choice)
			if !ok {
				return
			}
			pos := c.Diagnostic()
			for i, alt := range c.Alts {
				f.Alts = append(f.Alts, factstest.Alt{Line: pos.Line, Col: pos.Col, Index: i,
					First: terminalFact(firstTerminal(prog, alt, 0, map[string]bool{}))})
			}
		})
		out = append(out, f)
	}
	return out
}

func terminalFact(e grammar.Expr, depth int) factstest.Terminal {
	switch e := e.(type) {
	case *grammar.Literal:
		return factstest.Terminal{Exp: LiteralDesc(e), Depth: depth}
	case *grammar.CharClass:
		return factstest.Terminal{Exp: CharClassString(e), Depth: depth}
	}
	return factstest.Terminal{}
}
