package exec

import "zeroparse.dev/zp/internal/ir/pipeline"

// The passes of Exec IR as a pipeline (design 063): transformations of a lowered module that keep the
// behavior of its functions and that the backend printing it then prints. A pass has the name of the
// flag that turns it off, distinct from the names of the Core and Flow passes. The unit is the module
// that the lowering of one function made, with its declarations.

// Env is the context of a run of the Exec pipeline.
type Env struct {
	// Observe reports that the parser reports the rules it enters and the alternatives that guards
	// skip, which a pass must keep.
	Observe bool
	// Flag reports whether the optimization of the given name is on. Nil means every one is.
	Flag func(name string) bool
}

// On reports whether the optimization of the given name is on.
func (e *Env) On(name string) bool { return e.Flag == nil || e.Flag(name) }

// Pipeline is the pipeline of the Exec passes. It has no pass yet.
var Pipeline = &pipeline.Pipeline[*Module, *Env]{
	Analyses: map[pipeline.Analysis]func(*Module){},
	Verify:   Verify,
}

// RunOptions selects what a run of Pipeline does besides running the passes.
type RunOptions struct {
	// VerifyEach verifies the module after every pass.
	VerifyEach bool
	// Dump, if not nil, is called with "build" before the first pass and with the name of each pass
	// that ran, after it.
	Dump func(pass string, m *Module)
}

// RunPasses runs the enabled passes of Pipeline that apply to e on the module m, which the lowering has
// verified, and verifies it again if a pass ran. It returns the names of the passes that ran.
func RunPasses(m *Module, e *Env, o RunOptions) ([]string, error) {
	return Pipeline.Run(m, e, o.Options(e))
}

// Options returns the options of the generic pipeline for a run under e, for a caller that runs a
// pipeline of its own (tests).
func (o RunOptions) Options(e *Env) pipeline.Options[*Module] {
	return pipeline.Options[*Module]{Enabled: e.On, VerifyEach: o.VerifyEach, Dump: o.Dump, BuiltVerified: true}
}
