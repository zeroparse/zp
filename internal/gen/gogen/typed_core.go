package gogen

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/ir/coreir"
	irbuild "zeroparse.dev/zp/internal/ir/coreir/build"
)

// TypedCore builds the Core IR of the typed Go rules and runs the pipeline of Core passes on it:
// terminal regions (in rules lowered to Go-local code of lean parsers without observability), term
// rewrites (with literal text dispatch and unboxed field comparisons unless disabled), choice guards
// (with guarded classes in ordinary parsers), scans and, in lean parsers, class runs. With a hook
// (Generator.CoreHook, tests), it calls the hook with the module after it is built and after each
// pass.
func (g *Generator) TypedCore(origin map[coreir.Node]grammar.Expr) (*coreir.Module, error) {
	m, err := irbuild.CoreFrom(g.Prog, origin)
	if err != nil {
		return nil, err
	}
	mode := coreir.Mode{Lean: g.Ordinary, Ordinary: g.Ordinary, Observe: g.Observability}
	if err := g.RunCore("typed", m, mode, CoreLocalRule, nil); err != nil {
		return nil, err
	}
	return m, nil
}

// CoreLocalRule reports whether the typed Go backend lowers the rule r to Go-local code: its
// captures are variables of the rule's function rather than a runtime frame. Left-recursion
// leaders, whose growth the runtime owns, Operators Rules and rules with recovery keep runtime
// frames.
func CoreLocalRule(r *coreir.Rule) bool {
	if r.Policy.Leader {
		return false
	}
	if _, ok := r.Body.(*coreir.OperatorsRule); ok {
		return false
	}
	recovers := false
	coreir.Walk(r.Body, func(n coreir.Node) {
		if _, ok := n.(*coreir.Recover); ok {
			recovers = true
		}
	})
	return !recovers
}
