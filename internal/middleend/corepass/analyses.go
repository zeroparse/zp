package corepass

import (
	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/pass"
)

// The analyses of a Core IR module that the front end, the passes and the tools share through an
// AnalysisManager (internal/pass), so that each is computed once per module while it stays valid.

// FactsKey is the analysis of the rule facts of a module (Compute). The facts are a function of
// the rules' structure; the Core passes rewrite the bodies of a typed module after the facts are
// taken, and preserve no fact.
var FactsKey = pass.NewAnalysisKey("rule-facts", func(m *core.Module, am *pass.AnalysisManager[*core.Module]) (*Facts, error) {
	var control *cancel.Control
	if ctx := am.Context(); ctx != nil {
		control = &cancel.Control{Ctx: ctx}
	}
	return Compute(m, control), nil
})

// NewAnalyses returns a manager of the analyses of a module for work under control, which may be
// nil.
func NewAnalyses(control *cancel.Control) *pass.AnalysisManager[*core.Module] {
	if control == nil {
		return pass.NewAnalysisManager[*core.Module](nil)
	}
	return pass.NewAnalysisManager[*core.Module](control.Ctx)
}

// FactsOf returns the rule facts of m through am: those am holds, or new ones that it keeps.
func FactsOf(am *pass.AnalysisManager[*core.Module], m *core.Module) *Facts {
	f, err := pass.Get(am, m, FactsKey)
	if err != nil {
		panic(err) // Compute does not fail
	}
	return f
}
