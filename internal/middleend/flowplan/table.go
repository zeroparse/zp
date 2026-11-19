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
	// Origin maps each node of M that a grammar expression made to that expression; the scans keep
	// the origin of the repetitions they replace.
	Origin map[core.Node]ast.Expr
	Opts   flowlower.Options
}

// BuildTable builds the Core IR of the Node or recognizer table of the program prog, as c selects,
// runs the Core passes on it with runCore and returns it with the lowering options of the table.
// runCore runs the Core pipeline of the caller's driver, calling prepareScans around the scans
// (corepass.Env.PrepareScans); the scans keep the origin of each repetition they replace
// (elaborate.FollowRepeats). Every backend that runs the Node or recognizer table from Flow IR builds
// it here, so that, with the same optimizations, they lower the same Core IR with the same options.
// Classes are inline tests in these tables; the forms that only typed rules print are the Flow
// passes' (flowpass.Target).
func BuildTable(prog elaborate.Program, c TableConfig, runCore func(m *core.Module, prepareScans func(*core.Module) func()) error) (*TableIR, error) {
	origin := map[core.Node]ast.Expr{}
	m, err := elaborate.CoreProjecting(prog, origin, c.Values)
	if err != nil {
		return nil, err
	}
	prepare := func(m *core.Module) func() {
		repeats := elaborate.FollowRepeats(m, origin)
		return func() {
			for _, f := range repeats {
				f()
			}
		}
	}
	if err := runCore(m, prepare); err != nil {
		return nil, err
	}
	opts := flowlower.Options{Budget: c.Budget, Recovers: flowlower.Recovers(m), Observe: c.Observe, Node: true,
		Document: c.Document, Traced: c.Traced}
	return &TableIR{M: m, Origin: origin, Opts: opts}, nil
}
