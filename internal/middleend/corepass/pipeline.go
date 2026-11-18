package corepass

import (
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/pass"
)

// The optimization passes of Core IR as a pipeline (design 063). Generators run it with Run; a
// pass has the name of the flag that turns it off. The passes know no target and no generation
// mode: what depends on them is decided by the Flow passes (internal/middleend/flowpass).

// AttributesAnalysis is the analysis that core.Analyze computes: Nullable, First and Eff of every node.
// A pass that adds or replaces nodes makes it stale.
const AttributesAnalysis pass.Analysis = "attributes"

// Stage is the name of the Core IR module as the front end built it, before the first Core pass:
// the dumps and -print-after name it so.
const Stage = "core"

// Names of the passes. A name is also the name of the flag that turns the pass off.
const (
	PassGuards = "guards"
	PassScans  = "scans"
)

// Env is the context of a run of the pipeline.
type Env struct {
	// Flag reports whether the optimization of the given name is on. Nil means every one is.
	Flag func(name string) bool
	// PrepareScans, if not nil, is called with the module before the scans pass, and the function
	// it returns after it. The Flow route uses it to keep the origin of each repetition that a scan
	// replaces.
	PrepareScans func(*core.Module) (finish func())
}

// On reports whether the optimization of the given name is on.
func (e *Env) On(name string) bool { return e.Flag == nil || e.Flag(name) }

// Pipeline is the pipeline of the Core passes: choice guards and scans.
var Pipeline = &pass.Pipeline[*core.Module, *Env]{
	Passes: []pass.Pass[*core.Module, *Env]{
		{
			Name: PassGuards,
			// Guards read the First sets that Analyze computes. Without them the alternatives of a choice
			// get no first-character guard and the Flow form calls the rules of the later alternatives at
			// every position (duckdb: block_comment, 84,000 calls against none), and nothing reports it.
			Requires:   []pass.Analysis{AttributesAnalysis},
			Obligation: "A guard is the first terminal of its alternative, which the alternative cannot match without.",
			Run: func(m *core.Module, _ *Env) error {
				Guards(m)
				return nil
			},
		},
		{
			Name: PassScans,
			// A Stream is replaced as its repetition only where it delivers no elements (Eff).
			Requires:    []pass.Analysis{AttributesAnalysis},
			Invalidates: []pass.Analysis{AttributesAnalysis},
			Obligation:  "Consumes the same input and records the same expectations as the repetition it replaces.",
			Run: func(m *core.Module, e *Env) error {
				var finish func()
				if e.PrepareScans != nil {
					finish = e.PrepareScans(m)
				}
				Scans(m)
				if finish != nil {
					finish()
				}
				return nil
			},
		},
	},
	Analyses:       map[pass.Analysis]func(*core.Module){AttributesAnalysis: core.Analyze},
	Verify:         core.Verify,
	VerifyRequires: []pass.Analysis{AttributesAnalysis},
}

// RunOptions selects what a run of Pipeline does besides running the passes.
type RunOptions struct {
	// VerifyEach verifies the module after every pass.
	VerifyEach bool
	// Dump, if not nil, is called with "core" before the first pass and with the name of each pass
	// that ran, after it.
	Dump func(pass string, m *core.Module)
}

// Run runs the enabled passes of Pipeline on m, and verifies the module. It
// returns the names of the passes that ran.
func Run(m *core.Module, e *Env, o RunOptions) ([]string, error) {
	return Pipeline.Run(m, e, o.Options(e))
}

// Options returns the options of the generic pipeline for a run under e, for a caller that runs a
// pipeline of its own (tests).
func (o RunOptions) Options(e *Env) pass.Options[*core.Module] {
	return pass.Options[*core.Module]{Enabled: e.On, VerifyEach: o.VerifyEach, Dump: o.Dump, Stage: Stage}
}
