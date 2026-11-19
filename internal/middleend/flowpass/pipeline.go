// Package flowpass holds the passes of the Flow dialect of Code IR: the control-level decisions that
// choose the form of lowered grammar code (design 063).
package flowpass

import (
	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
	"zeroparse.dev/zp/internal/pass"
)

// The passes of Flow IR as a pipeline (design 063): transformations of a lowered function that keep
// its behavior and that every backend printing from Flow IR then prints. A pass has the name of the
// flag that turns it off, and that name is distinct from the names of the Core passes, so that one
// name selects one pass in the flags and in the dumps. The unit is a module at the Flow stage that
// holds one function (flow.FuncOf): a rule, a framed body, a line of an Operators Rule or an outlined node,
// as the lowering made it.

// Env is the context of a run of the Flow pipeline on one function.
type Env struct {
	// Func is the attributes of the function, which record how it was lowered (Attrs.Budget, Node,
	// Document, Traced, Frame) and so where a pass applies. Run sets it from the module; a caller that
	// runs a pipeline of its own sets it, as the driver does.
	Func *flow.Attrs
	// Observe reports that the parser reports the rules it enters and the alternatives that guards
	// skip, which a pass must keep.
	Observe bool
	// Flag reports whether the optimization of the given name is on. Nil means every one is.
	Flag func(name string) bool
	// Value is the value of the optimization of the given name that takes a number. Nil means each
	// has its default.
	Value func(name string) int
	// Target says which forms the target prints.
	Target Target
}

// Target is what the backend that prints the function can print, beyond the forms that every
// backend prints. A pass that makes a form applies only where the target prints it.
type Target struct {
	// Terminals: skipTerminal, a terminal region matched as one program, in a local function of
	// typed rules printed in the ordinary form.
	Terminals bool
	// ClassRuns: skipClasses, consecutive classes matched as one, in a function of typed rules printed
	// in the ordinary form.
	ClassRuns bool
	// ClassTables: matchClass, a class matched through its membership table, in a function of typed
	// rules printed in the ordinary form.
	ClassTables bool
	// GuardedClasses: a class that consumes the character a guard read, with the size the guard's
	// peek kept, in a function printed in the ordinary form.
	GuardedClasses bool
	// InlineLiterals: hasLit, a literal compared in place before matchLit.
	InlineLiterals bool
}

// On reports whether the optimization of the given name is on.
func (e *Env) On(name string) bool { return e.Flag == nil || e.Flag(name) }

// ValueOf returns the value of the optimization of the given name that takes a number.
func (e *Env) ValueOf(name string) int {
	if e.Value == nil {
		if f, ok := pass.Lookup(name); ok {
			return f.Default
		}
		return 0
	}
	return e.Value(name)
}

// Pipeline is the pipeline of the Flow passes, in the order they run: each chooses the form of a
// construct that the lowering wrote in its one form (flow.Forms), where the target prints the form
// and the function allows it.
var Pipeline = &pass.Pipeline[*code.Module, *Env]{
	Passes:   passes,
	Analyses: map[pass.Analysis]func(*code.Module){},
	Verify:   flow.VerifyModule,
}

// RunOptions selects what a run of Pipeline does besides running the passes.
type RunOptions struct {
	// VerifyEach verifies the module after every pass.
	VerifyEach bool
	// Dump, if not nil, is called with "flow" before the first pass and with the name of each pass
	// that ran, after it.
	Dump func(pass string, m *code.Module)
}

// Run runs the enabled passes of Pipeline that apply to e on the module m, which the lowering has
// verified, and verifies it again if a pass ran. It returns the module after them and the names of
// the passes that ran; m is not changed. The caller then calls Finish, also after a pipeline of its
// own.
func Run(m *code.Module, e *Env, o RunOptions) (*code.Module, []string, error) {
	e.Func = flow.AttrsOf(flow.FuncOf(m))
	return Pipeline.Run(m, e, o.Options(e))
}

// For reports whether the function of the run has attributes that make p hold: false for a function
// without attributes, which no pass applies to.
func (e *Env) For(p func(*flow.Attrs) bool) bool { return e.Func != nil && p(e.Func) }

// Options returns the options of the generic pipeline for a run under e, for a caller that runs a
// pipeline of its own (tests).
func (o RunOptions) Options(e *Env) pass.Options[*code.Module] {
	return pass.Options[*code.Module]{Enabled: e.On, VerifyEach: o.VerifyEach, Dump: o.Dump, Stage: flow.Stage, BuiltVerified: true}
}
