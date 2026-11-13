package gengo

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/ir/core"
	irbuild "zeroparse.dev/zp/internal/ir/core/build"
)

// TypedCore builds the Core IR of the typed Go rules and runs the pipeline of Core passes on it:
// terminal regions (in rules lowered to Go-local code of lean parsers without observability), term
// rewrites (with literal text dispatch and unboxed field comparisons unless disabled), choice guards
// (with guarded classes in ordinary parsers), scans and, in lean parsers, class runs. With a hook
// (Generator.CoreHook, tests), it calls the hook with the module after it is built and after each
// pass.
func (g *Generator) TypedCore(origin map[core.Node]grammar.Expr) (*core.Module, error) {
	m, err := irbuild.CoreFrom(g.Prog, origin)
	if err != nil {
		return nil, err
	}
	mode := core.Mode{Lean: g.Ordinary, Ordinary: g.Ordinary, Observe: g.Observability}
	if err := g.RunCore("typed", m, mode, CoreLocalRule, nil); err != nil {
		return nil, err
	}
	return m, nil
}

// CoreLocalRule reports whether the typed Go backend lowers the rule r to Go-local code: its
// captures are variables of the rule's function rather than a runtime frame. Left-recursion
// leaders, whose growth the runtime owns, Operators Rules and rules with recovery keep runtime
// frames.
func CoreLocalRule(r *core.Rule) bool {
	if r.Policy.Leader {
		return false
	}
	if _, ok := r.Body.(*core.OperatorsRule); ok {
		return false
	}
	recovers := false
	core.Walk(r.Body, func(n core.Node) {
		if _, ok := n.(*core.Recover); ok {
			recovers = true
		}
	})
	return !recovers
}
