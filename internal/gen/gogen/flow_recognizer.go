package gogen

import (
	"fmt"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/ir/flow"
	flowlower "zeroparse.dev/zp/internal/ir/flow/lower"
)

// Value-free rules of the Node runtime printed from Flow IR.
//
// The rules of a recognizer (recRules) and the lean rules of the Node table (rules) that
// flowlower.RecEligible accepts are built as Core IR, lowered by flowlower.Lower and printed by
// flowPrinter for the untyped Node runtime. The other rules print their framed body from Flow IR as
// well (flowNodeFrameBody), in the same output. Both tables share the helpers whose printed text is
// equal. The text passes run on the whole output afterwards.

// flowRecRule prints the eligible rule r of the recognizer from Flow IR as a flat rule. It returns
// nil in three cases, and the caller (generator.rules) then prints the rule's framed body from Flow IR
// (flowNodeFrameBody) and calls it through the rule table (plainCall); the generation fails if that
// does not print it either:
//   - flowlower.LocalRule rejects the rule, which needs the runtime-owned frame of a framed function;
//   - the printed function would read its start or its context, which a value-free flat rule has
//     no parameters for;
//   - a variable of the function has a type that untypedGoType cannot name.
func (g *Generator) flowRecRule(r *compile.Rule) *flatRule {
	if g.hooks.RefuseFlowRecRule != nil && g.hooks.RefuseFlowRecRule(r) {
		return nil
	}
	cr := g.Flow.M.Rules[r.ID]
	if !flowlower.LocalRule(cr) {
		return nil
	}
	f, err := g.FlowPasses(flowlower.Lower(g.Flow.M, cr, r.Name, g.Flow.Opts))
	if err != nil {
		panic(fmt.Errorf("rule %s of table %s: %w", r.Name, g.Table, err)) // LocalRule holds
	}
	g.Cur, g.Proj = r, g.Projections(r)
	defer func() { g.Proj = nil }()
	p := newFlowPrinter(g, false)
	p.document = f.Document
	return g.execRecRule(p, f)
}

// untypedGoType returns the Go type of a variable of a Flow IR function printed for the Node
// runtime, or false if the printer has no such type for it.
func untypedGoType(t flow.Type) (string, bool) {
	switch t {
	case flow.TypeInt, flow.TypeBool, flow.TypeRune, flow.TypeMark, flow.TypeExpMark, flow.TypeExpIDs:
		return goType(t), true
	}
	return "", false
}
