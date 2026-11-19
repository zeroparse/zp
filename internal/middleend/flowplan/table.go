package flowplan

import (
	"zeroparse.dev/zp/internal/frontend/elaborate"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/middleend/flowlower"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// TableConfig says how the functions of a Node or recognizer table run: what the parser that runs
// them charges and records.
type TableConfig struct {
	// Values selects the Node table, whose Core IR gathers the values of projected repetitions as its
	// actions read them; false selects the recognizer, whose actions read no projections.
	Values bool
	// Budget charges the budget controls of a parse (flowlower.Options.Budget).
	Budget bool
	// Observe reports the rules a parse enters and the alternatives that guards skip.
	Observe bool
	// Document records how far each call examines the input, as a build with Documents and a traced
	// parse do.
	Document bool
	// Traced selects the traced bodies.
	Traced bool
}

// TableIR is the Core IR of a Node or recognizer table after its Core passes, and the options that
// every function of the table is lowered with.
type TableIR struct {
	M *core.Module
	// Origin maps the ID of each node of M that a grammar expression made to that expression; the
	// passes keep it (elaborate.Origin).
	Origin elaborate.Origin
	Opts   flowlower.Options
}

// Unit returns what the lowering of the function f of the table takes: f with the table's Core IR
// and options. Every function of the table is lowered from the same module with the same options.
func (t *TableIR) Unit(f flowlower.Func) flowlower.Unit {
	return flowlower.Unit{Module: t.M, Opts: t.Opts, Func: f}
}

// BuildTable builds the Core IR of the Node or recognizer table of the program prog, as c selects,
// through coreStep, the Core step of the caller's driver (driver.Driver.Core for the caller's table):
// coreStep runs build, the elaboration of the table, and then the Core passes, and returns the module
// after them. BuildTable returns that module with the lowering options of the table. Every backend
// that runs the Node or recognizer table from Flow IR builds it here, so that, with the same
// optimizations, they lower the same Core IR with the same options. Classes are inline tests in
// these tables; the forms that only typed rules print are the Flow passes' (flowpass.Target).
func BuildTable(prog elaborate.Program, c TableConfig, coreStep func(build func() (*core.Module, error)) (*core.Module, error)) (*TableIR, error) {
	origin := map[core.Node]ast.Expr{}
	m, err := coreStep(func() (*core.Module, error) { return elaborate.CoreProjecting(prog, origin, c.Values) })
	if err != nil {
		return nil, err
	}
	opts := flowlower.Options{Budget: c.Budget, Recovers: flowlower.Recovers(m), Observe: c.Observe, Node: true,
		Document: c.Document, Traced: c.Traced}
	return &TableIR{M: m, Origin: elaborate.OriginByID(origin), Opts: opts}, nil
}
