package flowlower

import (
	"fmt"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/middleend/corepass"
	"zeroparse.dev/zp/internal/pass"
)

// The lowering as a step of the pipeline of a compilation.
//
// A Unit is what one lowering takes: the arguments of Lower, LowerFrame and LowerOutline, with the
// form that selects among them. Lowering is the lowering as the required pass
// "flowlower" from the Core stage to the Flow stage: it lowers a Unit, and the Flow verifier
// (Verify), its postcondition, checks the result before any Flow pass sees it.

// Form selects the function a Unit is lowered to.
type Form int

const (
	// Flat is a local function of a rule (Lower): Node, Slots and Build are unused.
	Flat Form = iota
	// Frame is a framed function (LowerFrame).
	Frame
	// Outline is a function that a CallOutline of a framed function runs (LowerOutline).
	Outline
)

// Func is one function to lower, as the planner of a table (flowplan.FuncSpec) lists it: the rule
// Rule in the form Form, named Name. A framed function or an outline lowers the node Body with the
// capture scope Slots, building its value if Build; a flat function lowers the body of Rule, and
// Body, Slots and Build are unused.
type Func struct {
	Form  Form
	Rule  *core.Rule
	Name  string
	Body  core.Node
	Slots []core.Slot
	Build bool
}

// Unit is what one run of the lowering takes: the Core module Module of a table, on which Analyze
// must have run, the options Opts of its functions, and the function Func.
type Unit struct {
	Module *core.Module
	Opts   Options
	Func   Func
}

// LowerUnit lowers u to a module of one function at the Flow stage, which it does not verify:
// Verify, the postcondition of the lowering, does.
func LowerUnit(u Unit) (*code.Module, error) {
	f := u.Func
	switch f.Form {
	case Flat:
		return lowerFlat(u.Module, f.Rule, f.Name, u.Opts)
	case Frame, Outline:
		return lowerFrameBody(u.Module, f.Rule, f.Body, f.Slots, f.Build, f.Name, u.Opts)
	}
	return nil, fmt.Errorf("rule %s: no lowering of the form %d", f.Rule.Name, f.Form)
}

// Verify checks a module that the lowering made with the Flow verifier. A module it rejects is an
// internal error of the lowering.
func Verify(m *code.Module) error {
	if err := flow.VerifyModule(m); err != nil {
		return fmt.Errorf("internal error: %w", err)
	}
	return nil
}

// verified returns the module of a lowering once Verify accepts it.
func verified(m *code.Module, err error) (*code.Module, error) {
	if err == nil {
		err = Verify(m)
	}
	if err != nil {
		return nil, err
	}
	return m, nil
}

// IR describes a module of Flow functions to the manager of the pipeline: it is verified by Verify
// and dumped as the listing of each of its functions, labeled with its table.
var IR = pass.IR[*code.Module]{Stage: flow.Stage, Verify: Verify, Print: flow.Print, Copy: (*code.Module).Clone}

// Name is the name of the lowering as a step of the pipeline.
const Name = "flowlower"

// Lowering is the lowering as the required step Name from the Core stage to the Flow stage.
var Lowering = pass.Lower(Name, corepass.Stage, IR, LowerUnit)
