package compile

import (
	"slices"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/cancel"
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
	var starts []string
	for _, rd := range rules {
		if desugar.StartMarked(rd) {
			starts = append(starts, rd.Name)
		}
	}
	origin := map[core.Node]grammar.Expr{}
	m, err := build.Structural(StructuralInput(g, rules, starts), origin)
	if err != nil {
		return nil, err
	}
	return &CostSource{Grammar: g, Rules: rules, Module: m, Origin: origin, Facts: facts.Compute(m, control)}, nil
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

// CostViewRules returns the rules of the normal view of g (NormalView) and the number of rewrites
// of each normalization rule, by its name ("N1" to "N13"). If laws is not nil, only the rules it
// names apply. It is what the cost view built on Core IR is compared with.
func CostViewRules(g *grammar.Grammar, matching bool, laws []string, control *cancel.Control) ([]*grammar.RuleDef, map[string]int, error) {
	v, _, err := normalViewOf(g, matching, laws, control)
	if err != nil {
		return nil, nil, err
	}
	applied := map[string]int{}
	for r, n := range v.applied {
		applied[string(r)] = n
	}
	return slices.Clone(v.rules), applied, nil
}

// normalViewOf returns the normal view of the classifier's rules of g, and the grammar they are
// rules of. If laws is not nil, only the rules it names apply.
func normalViewOf(g *grammar.Grammar, matching bool, laws []string, control *cancel.Control) (*view, *grammar.Grammar, error) {
	g, rules, err := classifierRules(g, control)
	if err != nil {
		return nil, nil, err
	}
	rf, err := classifierFacts(g, rules, control)
	if err != nil {
		return nil, nil, err
	}
	opts := costView
	if matching {
		opts = viewOptions{labels: true, inline: true}
	}
	if laws != nil {
		opts.laws = map[viewRule]bool{}
		for _, l := range laws {
			opts.laws[viewRule(l)] = true
		}
	}
	return normalizeRules(rules, rf.an, rf.memo, opts, control), g, nil
}
