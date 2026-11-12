package flow

import "zeroparse.dev/zp/internal/ir/pipeline"

// The passes of Flow IR as a pipeline (design 063): transformations of a lowered function that keep
// its behavior and that every backend printing from Flow IR then prints. A pass has the name of the
// flag that turns it off, and that name is distinct from the names of the Core passes, so that one
// name selects one pass in the flags and in the dumps. The unit is one function: a rule, a framed
// body, a line of an Operators Rule or an outlined node, as the lowering made it.

// Env is the context of a run of the Flow pipeline. The function records the rest of how it was
// lowered (Func.Budget, Node, Document, Frame).
type Env struct {
	// Observe reports that the parser reports the rules it enters and the alternatives that guards
	// skip, which a pass must keep.
	Observe bool
	// Flag reports whether the optimization of the given name is on. Nil means every one is.
	Flag func(name string) bool
}

// On reports whether the optimization of the given name is on.
func (e *Env) On(name string) bool { return e.Flag == nil || e.Flag(name) }

// Pipeline is the pipeline of the Flow passes. It has no pass yet: the decisions that the lowering
// makes from Core effects move into passes here, each with its flag, its obligation and the Flow
// verifier after it.
var Pipeline = &pipeline.Pipeline[*Func, *Env]{
	Analyses: map[pipeline.Analysis]func(*Func){},
	Verify:   Verify,
}

// RunOptions selects what a run of Pipeline does besides running the passes.
type RunOptions struct {
	// VerifyEach verifies the function after every pass.
	VerifyEach bool
	// Dump, if not nil, is called with "build" before the first pass and with the name of each pass
	// that ran, after it.
	Dump func(pass string, f *Func)
}

// Run runs the enabled passes of Pipeline that apply to e on the function f, which the lowering has
// verified, and verifies it again if a pass ran. It returns the names of the passes that ran.
func Run(f *Func, e *Env, o RunOptions) ([]string, error) {
	return Pipeline.Run(f, e, o.Options(e))
}

// Options returns the options of the generic pipeline for a run under e, for a caller that runs a
// pipeline of its own (tests).
func (o RunOptions) Options(e *Env) pipeline.Options[*Func] {
	return pipeline.Options[*Func]{Enabled: e.On, VerifyEach: o.VerifyEach, Dump: o.Dump, BuiltVerified: true}
}
