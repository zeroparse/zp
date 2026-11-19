package print

import (
	"fmt"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/ir/code/flow"
	"zeroparse.dev/zp/internal/middleend/flowplan"
)

// Value-free rules of the Node library printed from Flow IR.
//
// The rules of a recognizer (recRules) and the lean rules of the Node table (rules) that the table's
// plan lowers as flat functions (flowplan.Plan) are printed by flowPrinter for the untyped Node
// library. The other rules print their framed body from Flow IR as well (flowNodeFrameBody), in the
// same output. Both tables share the helpers whose printed text is equal. The text passes run on the
// whole output afterwards.

// flowRecRule prints the flat function spec of the rule r from Flow IR as a flat rule. It returns nil
// if the plan refuses the lowered function (flowplan.Refused: it reads its start or its context, or a
// variable holds a value), and the caller (generator.rules) then prints the spec's framed fallback
// (flowNodeFrameBody) and calls it through the rule table (plainCall); the generation fails if the
// rule has no fallback.
func (g *Generator) flowRecRule(r *compile.Rule, spec *flowplan.FuncSpec) *flatRule {
	if g.hooks.RefuseFlowRecRule != nil && g.hooks.RefuseFlowRecRule(r) {
		return nil
	}
	f, err := g.FlowPasses(spec.Lower(g.Flow.M, g.Flow.Opts))
	if err != nil {
		panic(fmt.Errorf("rule %s of table %s: %w", r.Name, g.Table, err)) // LocalRule holds
	}
	g.Cur, g.Proj = r, g.Projections(r)
	defer func() { g.Proj = nil }()
	p := newFlowPrinter(g, false)
	p.document = flow.AttrsOf(flow.FuncOf(f)).Document
	return g.execRecRule(p, f)
}

// untypedGoType returns the Go type of a variable of a Flow IR function printed for the Node
// library, or false if the printer has no such type for it.
func untypedGoType(t flow.Type) (string, bool) {
	switch t {
	case flow.TypeInt, flow.TypeBool, flow.TypeRune, flow.TypeMark, flow.TypeExpMark, flow.TypeExpIDs:
		return goType(t), true
	}
	return "", false
}
