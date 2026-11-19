package print

import (
	irbuild "zeroparse.dev/zp/internal/frontend/elaborate"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// TypedCore builds the Core IR of the typed Go rules and runs the pipeline of Core passes on it:
// choice guards and scans. The lean forms of typed rules (terminal programs, class runs and tables,
// guarded classes) are chosen by the Flow passes (common.Generator.FlowTarget). A Runner with a
// hook (driver.Driver.CoreHook, tests) calls it with the module after it is built and after each
// pass.
func (g *Generator) TypedCore(origin map[core.Node]ast.Expr) (*core.Module, error) {
	m, err := irbuild.CoreFrom(g.Prog.Elab, origin)
	if err != nil {
		return nil, err
	}
	if err := g.RunCore(g.IRTable(), m, nil); err != nil {
		return nil, err
	}
	return m, nil
}
