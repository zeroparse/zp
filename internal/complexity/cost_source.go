package complexity

import (
	"slices"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/desugar"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/ir/core/build"
	"zeroparse.dev/zp/internal/ir/core/facts"
)

// CostSource is what the cost view of a grammar is built from: the rules that the complexity
// classifier reads, their untyped Core IR module, the expression each node of the module is built
// from, and the facts of the module. Rule i of Rules is rule i of Module.
type CostSource struct {
	// Grammar is the grammar with inline actions lowered and attributes normalized.
	Grammar *grammar.Grammar
	// Rules are the unique rules of Grammar.
	Rules  []*grammar.RuleDef
	Module *core.Module
	Origin map[core.Node]grammar.Expr
	Facts  *facts.Facts
}

// NewCostSource returns the cost source of g, which must compile.
func NewCostSource(g *grammar.Grammar, control *cancel.Control) (*CostSource, error) {
	g, rules, err := classifierRules(g, control)
	if err != nil {
		return nil, err
	}
	src, err := costSource(g, rules, control)
	if err != nil {
		return nil, err
	}
	return &CostSource{Grammar: g, Rules: rules, Module: src.Module, Origin: src.Origin, Facts: src.Facts}, nil
}

// classifierRules returns g with inline actions lowered and attributes normalized, and its unique
// rules: the rules that the complexity classifier and the normal view read.
func classifierRules(g *grammar.Grammar, control *cancel.Control) (*grammar.Grammar, []*grammar.RuleDef, error) {
	control.Check()
	if desugar.HasInlineAction(g, control) {
		lowered, errs := desugar.LowerInlineActionsControlled(g, desugar.InlineNames{}, control)
		if errs != nil {
			return nil, nil, errs
		}
		g = lowered
	}
	g, err := desugar.NormalizeAttributesControlled(g, false, control)
	if err != nil {
		return nil, nil, err
	}
	rules, errs := desugar.UniqueRules(g, control)
	if len(errs) > 0 {
		return nil, nil, errs
	}
	return g, rules, nil
}

// NormalView returns the grammar that the compiler runs for g, with inline actions lowered and
// attributes normalized, and with the bodies of its rules replaced by those of their cost view
// (internal/complexity). With matching, the view also drops labels and inlines the calls that P1
// allows: it then keeps only what a parse matches and where it ends (≡E), which is what an
// analysis of matching, such as the exclusion of alternatives, needs. Without matching it is the
// view that the complexity classifier reads. g must compile.
func NormalView(g *grammar.Grammar, matching bool, control *cancel.Control) (*grammar.Grammar, error) {
	cs, err := NewCostSource(g, control)
	if err != nil {
		return nil, err
	}
	v := Build(&Source{Module: cs.Module, Origin: cs.Origin, Facts: cs.Facts},
		Options{Labels: matching, Inline: matching}, control)
	rules, err := v.Grammar(cs.Rules)
	if err != nil {
		return nil, err
	}
	byName := map[string]*grammar.RuleDef{}
	for _, r := range rules {
		byName[r.Name] = r
	}
	out := *cs.Grammar
	out.File.Decls = slices.Clone(cs.Grammar.File.Decls)
	for i, d := range out.File.Decls {
		if r, ok := d.(*grammar.RuleDef); ok {
			out.File.Decls[i] = byName[r.Name]
		}
	}
	return &out, nil
}

// costSource returns the untyped Core IR module of rules, the unique rules of g, whose start rules
// are those marked @start, with the expression each node is built from and the facts of the
// module: what the cost view is built from. Rule i of rules has ID i in the module.
func costSource(g *grammar.Grammar, rules []*grammar.RuleDef, control *cancel.Control) (*Source, error) {
	var starts []string
	for _, rd := range rules {
		if desugar.StartMarked(rd) {
			starts = append(starts, rd.Name)
		}
	}
	origin := map[core.Node]grammar.Expr{}
	m, err := build.Structural(compile.StructuralInput(g, rules, starts), origin)
	if err != nil {
		return nil, err
	}
	return &Source{Module: m, Origin: origin, Facts: facts.Compute(m, control)}, nil
}
