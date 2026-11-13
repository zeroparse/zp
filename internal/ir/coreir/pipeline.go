package coreir

import (
	"fmt"

	"zeroparse.dev/zp/internal/ir/pipeline"
)

// The optimization passes of Core IR as a pipeline (design 063). Generators run it with Run; a
// pass has the name of the flag that turns it off.

// AttributesAnalysis is the analysis that Analyze computes: Nullable, First and Eff of every node.
// A pass that adds or replaces nodes makes it stale.
const AttributesAnalysis pipeline.Analysis = "attributes"

// Names of the passes. A name is also the name of the flag that turns the pass off.
const (
	PassTerminals = "terminals"
	PassGuards    = "guards"
	PassScans     = "scans"
	PassClassRuns = "class-runs"
)

// Names of the flags that select a variant of a pass. A variant is on unless its flag, or the flag of
// the pass, says otherwise.
const (
	// FlagGuardedClasses selects the GuardedClass marks that guards sets.
	FlagGuardedClasses = "guarded-classes"
)

// Mode is what a pass's applicability depends on: how the module's parser is generated.
type Mode struct {
	// Lean reports that the module is the typed rules of a parser that prints bodies in their ordinary
	// form (no budget controls, no Documents). Terminals and class runs are sound there only: budget
	// controls and Documents charge for the input that they read.
	Lean bool
	// Ordinary reports that the parser prints bodies in their ordinary form, which guarded classes
	// need: the consuming advance of a guarded class charges no budget for the input.
	Ordinary bool
	// Observe reports that the parser reports the rules it enters and the alternatives that guards
	// skip. A Terminal's program does not.
	Observe bool
}

// Env is the context of a run of the pipeline.
type Env struct {
	Mode Mode
	// Flag reports whether the optimization of the given name is on. Nil means every one is.
	Flag func(name string) bool
	// Local reports the rules that terminals regions are made in: those lowered to Go-local code.
	Local func(*Rule) bool
	// PrepareScans, if not nil, is called with the module before the scans pass, and the function
	// it returns after it. The Flow route uses it to keep the origin of each repetition that a scan
	// replaces.
	PrepareScans func(*Module) (finish func())
}

// On reports whether the optimization of the given name is on.
func (e *Env) On(name string) bool { return e.Flag == nil || e.Flag(name) }

// Pipeline is the pipeline of the Core passes: terminal regions, choice guards, scans and class runs. A run on the recognizer's or the Node table's module has no Lean mode, so
// the first and last do not apply.
var Pipeline = &pipeline.Pipeline[*Module, *Env]{
	Passes: []pipeline.Pass[*Module, *Env]{
		{
			Name:        PassTerminals,
			Invalidates: []pipeline.Analysis{AttributesAnalysis},
			Obligation:  "Consumes the same input and records the same expectations as the expression it replaces.",
			Applies:     func(e *Env) bool { return e.Mode.Lean && !e.Mode.Observe },
			Run: func(m *Module, e *Env) error {
				if e.Local == nil {
					return fmt.Errorf("no rule predicate")
				}
				Terminals(m, e.Local)
				return nil
			},
		},
		{
			Name: PassGuards,
			// Guards read the First sets that Analyze computes. Without them the alternatives of a choice
			// get no first-character guard and the Flow form calls the rules of the later alternatives at
			// every position (duckdb: block_comment, 84,000 calls against none), and nothing reports it.
			Requires:   []pipeline.Analysis{AttributesAnalysis},
			After:      []string{PassTerminals},
			Obligation: "A guard is the first terminal of its alternative, which the alternative cannot match without.",
			Run: func(m *Module, e *Env) error {
				Guards(m, e.Mode.Ordinary && e.On(FlagGuardedClasses))
				return nil
			},
		},
		{
			Name: PassScans,
			// A Stream is replaced as its repetition only where it delivers no elements (Eff).
			Requires:    []pipeline.Analysis{AttributesAnalysis},
			Invalidates: []pipeline.Analysis{AttributesAnalysis},
			Obligation:  "Consumes the same input and records the same expectations as the repetition it replaces.",
			Run: func(m *Module, e *Env) error {
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
		{
			Name:        PassClassRuns,
			After:       []string{PassGuards},
			Invalidates: []pipeline.Analysis{AttributesAnalysis},
			Obligation:  "Consumes the same input and records the same expectations as the classes it replaces; the first class of a guarded alternative is left alone.",
			Applies:     func(e *Env) bool { return e.Mode.Lean },
			Run: func(m *Module, _ *Env) error {
				ClassRuns(m)
				return nil
			},
		},
	},
	Analyses:       map[pipeline.Analysis]func(*Module){AttributesAnalysis: Analyze},
	Verify:         Verify,
	VerifyRequires: []pipeline.Analysis{AttributesAnalysis},
}

// RunOptions selects what a run of Pipeline does besides running the passes.
type RunOptions struct {
	// VerifyEach verifies the module after every pass.
	VerifyEach bool
	// Dump, if not nil, is called with "build" before the first pass and with the name of each pass
	// that ran, after it.
	Dump func(pass string, m *Module)
}

// Run runs the enabled passes of Pipeline that apply to e.Mode on m, and verifies the module. It
// returns the names of the passes that ran.
func Run(m *Module, e *Env, o RunOptions) ([]string, error) {
	return Pipeline.Run(m, e, pipeline.Options[*Module]{Enabled: e.On, VerifyEach: o.VerifyEach, Dump: o.Dump})
}
