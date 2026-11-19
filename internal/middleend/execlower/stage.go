package execlower

import (
	"fmt"
	"io"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
	"zeroparse.dev/zp/internal/pass"
)

// The conversion as a step of the pipeline of a compilation.
//
// Lowering is the conversion as the required pass "execlower" from the Flow stage to the Exec stage: it
// converts the module of one lowered function (LowerFunc) to a Lowered, and Verify, its
// postcondition, checks the result before any Exec pass sees it.

// LowerFunc converts the module m of one lowered function from the Flow stage to the Exec stage, as
// Lower does but without verifying the result, and returns the function with its variables.
func LowerFunc(m *code.Module) (*Lowered, error) {
	fn := flow.FuncOf(m) // the conversion moves it out of m
	x, err := lower(m)
	if err != nil {
		return nil, err
	}
	return &Lowered{Module: x, Func: fn, Vars: flow.AttrsOf(fn).Vars}, nil
}

// IR describes a lowered function to the manager of the pipeline: its module is verified by Verify
// and must hold the function with its variables (Lowered.Keeps), and it is dumped as the listing of
// each function of its module, labeled with its table.
var IR = pass.IR[*Lowered]{
	Stage: Stage,
	// The module verifies, and holds the function with its variables (Lowered.Keeps).
	Verify: func(l *Lowered) error {
		if err := Verify(l.Module); err != nil {
			return err
		}
		return l.Keeps()
	},
	Print: func(w io.Writer, lb pass.Label, after string, l *Lowered) {
		PrintModule(w, lb, after, l.Module)
	},
}

// PrintModule writes the dump of the Exec module m, labeled l, after the step after: each function
// under a header line of its own.
func PrintModule(w io.Writer, l pass.Label, after string, m *code.Module) {
	for _, f := range m.Funcs {
		fmt.Fprintf(w, "-- Exec IR of %s in the %s table after %s --\n%s\n", f.Name, l.Table, after, code.FuncStringIn(m, f))
	}
}

// Name is the name of the conversion as a step of the pipeline.
const Name = "execlower"

// Lowering is the conversion as the required step Name from the Flow stage to the Exec stage.
var Lowering = pass.Lower(Name, flow.Stage, IR, LowerFunc)
