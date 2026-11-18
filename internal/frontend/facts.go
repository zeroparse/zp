package frontend

import (
	"slices"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/diagnostic"
	"zeroparse.dev/zp/internal/frontend/desugar"
	"zeroparse.dev/zp/internal/frontend/elaborate"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/middleend/corepass"
)

// Check checks g into its static program (compile.CheckCanonical) with the rule facts and the site
// numbers of the untyped Core IR module of its rules: the compiler takes every rule fact from the
// Core facts (internal/middleend/corepass). names receives the hidden rules that the lowering of
// inline actions generates. If flags is not nil, the rules take the flags of a compiled grammar
// (compile.Program.Flags), and the sites are numbered with them. A non-nil control polls for
// cancellation.
func Check(g *grammar.Grammar, opts compile.Options, flags []compile.RuleFlags, names desugar.InlineNames, control *cancel.Control) (*compile.Program, error) {
	canon, err := compile.Canonicalize(g, opts, names, control)
	if err != nil {
		return nil, err
	}
	defs, _ := desugar.UniqueRules(canon.Grammar, control)
	origin := map[core.Node]grammar.Expr{}
	m, failure := structural(canon, defs, origin)
	var facts *compile.RuleFacts
	if failure == nil {
		facts = ruleFacts(m, corepass.Compute(m, control), origin, defs)
	} else {
		// The module cannot be built from a grammar with an error that the compiler reports, such
		// as a malformed attribute. The rules then take no facts, so that the compiler still
		// reports every error it finds.
		facts = &compile.RuleFacts{Rules: make([]compile.RuleFact, len(defs))}
	}
	prog, err := compile.CheckCanonical(canon, opts, flags, names, facts, control)
	if err != nil {
		return nil, err
	}
	if failure != nil {
		// A grammar that the compiler accepts has a module: the failure is a bug, which keeps
		// its own error or panic.
		if f, ok := failure.(error); ok {
			return nil, f
		}
		panic(failure)
	}
	prog.SetSites(sites(m, prog, origin, control))
	return prog, nil
}

// structural builds the untyped Core IR module of defs, the unique rules of canon, recording the
// grammar expression of each node in origin. A grammar the compiler rejects may fail to build: the
// failure, an error or the value of a panic, is returned for Check to report only if the compiler
// accepts the grammar. A cancellation is not a failure and unwinds.
func structural(canon *compile.Canonical, defs []*grammar.RuleDef, origin map[core.Node]grammar.Expr) (m *core.Module, failure any) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(cancel.Cancellation); ok {
				panic(r)
			}
			m, failure = nil, r
		}
	}()
	m, err := elaborate.Structural(compile.StructuralInput(canon.Grammar, defs, canon.Starts), origin)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// ruleFacts returns the facts that the compiler reads, of the module m of the rules defs, whose
// facts are f.
func ruleFacts(m *core.Module, f *corepass.Facts, origin map[core.Node]grammar.Expr, defs []*grammar.RuleDef) *compile.RuleFacts {
	out := &compile.RuleFacts{Rules: make([]compile.RuleFact, len(defs))}
	for i := range defs {
		r := &f.Rules[i]
		rf := compile.RuleFact{Cycle: r.Cycle, Leader: r.Leader, Member: r.Member,
			CycleIns: r.CycleIns, LeaderIns: r.LeaderIns, MemberIns: r.MemberIns,
			Positional: r.Positional, Transient: r.Transient, Rec: r.Rec, Vars: r.Vars}
		if r.Recovering >= 0 {
			rf.Recovering = m.Rules[r.Recovering].Name
		}
		if o, ok := m.Rules[i].Body.(*core.OperatorsRule); ok {
			rf.LeftOperands = make([]bool, len(o.Operands))
			for k, l := range o.Operands {
				rf.LeftOperands[k] = slices.Contains(f.LeftCalls(l.Body, false), i) || slices.Contains(f.LeftCalls(l.Body, true), i)
			}
		}
		out.Rules[i] = rf
	}
	for _, e := range f.BindingErrors(m) {
		pos := origin[e.Node].(*grammar.Predicate).Term.(*grammar.Bind).Diagnostic()
		out.BindingErrors = append(out.BindingErrors, &diagnostic.Error{Pos: pos, Msg: e.Msg})
	}
	return out
}

// sites numbers the sites of the module m of prog's rules with the rule flags of prog, keyed by
// the repetitions of the grammar that the module's repetitions were built from.
func sites(m *core.Module, prog *compile.Program, origin map[core.Node]grammar.Expr, control *cancel.Control) compile.Sites {
	rules := make([]corepass.SiteRule, len(prog.RuleList))
	for i, r := range prog.RuleList {
		rules[i] = corepass.SiteRule{LeftRec: r.Leader || r.Member, Vars: r.Vars, Declared: compile.DeclaredBeyondScan(r.Def)}
	}
	s := corepass.ComputeSites(m, rules, control)
	key := func(in map[*core.Repeat]int) map[*grammar.Repeat]int {
		out := make(map[*grammar.Repeat]int, len(in))
		for n, site := range in {
			out[origin[n].(*grammar.Repeat)] = site
		}
		return out
	}
	var runKeys map[int]compile.RunKey
	for site, k := range s.RunKeys {
		if runKeys == nil {
			runKeys = make(map[int]compile.RunKey, len(s.RunKeys))
		}
		runKeys[site] = compile.RunKey{Vars: k.Vars, Bind: k.Bind}
	}
	return compile.Sites{Skip: key(s.Skip), Run: key(s.Run), Scan: key(s.Scan),
		NumSkip: s.NumSkip, NumRun: s.NumRun, NumScan: s.NumScan, EnvFree: s.EnvFreeSites(), RunKeys: runKeys}
}
