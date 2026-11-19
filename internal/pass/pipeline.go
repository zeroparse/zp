// Package pass runs declared passes over an IR unit, and holds the registry of the optimizations that
// turn passes and other choices of the pipeline on and off (design 063).
//
// A Pass names itself, states the analyses it reads and the analyses it leaves valid, the passes it
// must follow, the generation modes it applies to and what it preserves. A Pipeline is an ordered list
// of passes. Run executes the enabled, applicable passes in that order: before each pass it makes the
// pass's required analyses valid, after it discards every analysis that the pass does not preserve
// (an AnalysisManager, which the run may share with its caller), and it verifies the unit, once at
// the end or after every pass. A pass that breaks an invariant is named in the error.
//
// The package knows nothing about the unit M or the context X that its callers choose: Core IR
// (internal/middleend/corepass, Pipeline), Flow IR (internal/ir/code/flow, Pipeline) and Exec IR
// (internal/middleend/codepass, Pipeline) use it. It holds no state between runs; the registry is filled once,
// by the init functions of the parts that register flags, and the analysis keys are declared once.
// A compilation chains the pipelines of its IRs and the lowerings between them into one typed
// composition of stages under one manager (Stage, Then, Lower, Run).
//
// It imports no other package of the module.
package pass

import "fmt"

// Analysis names data about a unit that the passes of a pipeline read and that other passes make
// stale. The pipeline's Analyses say how to compute each; the run computes them through its
// AnalysisManager under their names, which no AnalysisKey may take.
type Analysis string

// A Pass is one transformation of a unit M under the context X of the generation mode.
type Pass[M, X any] struct {
	// Name identifies the pass. It is also the name of the flag that turns the pass off and the
	// name that the dump and the errors give.
	Name string
	// Requires lists the analyses that must be valid when the pass starts.
	Requires []Analysis
	// Preserves is the set of analyses, of the pipeline and of the AnalysisManager of the run,
	// that the pass leaves valid. The zero value preserves none: an analysis that a pass does not
	// declare is computed again, never read stale.
	Preserves PreservedSet
	// After lists the passes that must precede this one in the pipeline. It orders passes that
	// share no data (an analysis would otherwise order them).
	After []string
	// Obligation states in one sentence what the pass preserves.
	Obligation string
	// Applies reports whether the pass is sound in the mode x; nil means always. A pass that does not
	// apply never runs, whatever its flag says.
	Applies func(x X) bool
	// Run transforms the unit.
	Run func(m M, x X) error
}

// A Pipeline is an ordered list of passes with the analyses they use.
type Pipeline[M, X any] struct {
	// Passes in the order they run.
	Passes []Pass[M, X]
	// Analyses says how to compute each analysis that a pass or the final verification requires. A
	// computation is deterministic over an unchanged unit, so how often it runs never changes the
	// unit.
	Analyses map[Analysis]func(m M)
	// Verify checks the invariants of a unit, after the analyses in VerifyRequires are valid.
	Verify func(m M) error
	// VerifyRequires lists the analyses that Verify reads.
	VerifyRequires []Analysis
}

// Options selects what a run does. The zero value runs every applicable pass and verifies once, at
// the end.
type Options[M any] struct {
	// Enabled, if not nil, reports whether the pass of the given name may run.
	Enabled func(name string) bool
	// VerifyEach verifies the unit after every pass that ran, so that the error names the first pass
	// that broke an invariant.
	VerifyEach bool
	// Dump, if not nil, is called with Stage before the first pass and with the name of each pass
	// after it ran, before it is verified.
	Dump func(pass string, m M)
	// Stage names the unit as it reaches the first pass: the stage of the pipeline that made it,
	// such as "core", "flow" or "exec". A run with a Dump requires it, so that every dump is named.
	Stage string
	// BuiltVerified reports that the builder of the unit has verified it, so that a run in which no
	// pass ran does not verify it again.
	BuiltVerified bool
	// Analyses, if not nil, is the manager of the analyses of the unit that the run shares with
	// its caller: the run reads and keeps its analyses there, and discards those that a pass does
	// not preserve. Nil runs with a manager of its own.
	Analyses *AnalysisManager[M]

	// gate, if not nil, is asked before an enabled pass that applies runs; false skips it. A stage
	// of the compilation numbers the runs of optional passes with it (Run).
	gate func(name string) bool
	// ran, if not nil, is called after each pass that ran, before it is verified.
	ran func(name string, m M)
}

// Validate checks the declarations: names are distinct and non-empty, every analysis a pass or
// the verification requires can be computed, every pass in After precedes the pass that names it.
func (p *Pipeline[M, X]) Validate() error {
	seen := map[string]bool{}
	need := func(who string, as []Analysis) error {
		for _, a := range as {
			if p.Analyses[a] == nil {
				return fmt.Errorf("%s requires the analysis %q, which the pipeline cannot compute", who, a)
			}
		}
		return nil
	}
	for _, ps := range p.Passes {
		if ps.Name == "" || ps.Run == nil {
			return fmt.Errorf("pass %q has no name or no Run", ps.Name)
		}
		if seen[ps.Name] {
			return fmt.Errorf("pass %q is declared twice", ps.Name)
		}
		for _, a := range ps.After {
			if !seen[a] {
				return fmt.Errorf("pass %q must follow %q, which does not precede it", ps.Name, a)
			}
		}
		if err := need("pass "+ps.Name, ps.Requires); err != nil {
			return err
		}
		seen[ps.Name] = true
	}
	if p.Verify != nil {
		return need("verification", p.VerifyRequires)
	}
	return nil
}

// Names returns the names of the passes in order.
func (p *Pipeline[M, X]) Names() []string {
	out := make([]string, len(p.Passes))
	for i, ps := range p.Passes {
		out[i] = ps.Name
	}
	return out
}

// Run runs the passes on m under the mode x and returns the names of the passes that ran, in order.
// An error names the pass that failed, or "verification" for the final check.
func (p *Pipeline[M, X]) Run(m M, x X, o Options[M]) ([]string, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if o.Dump != nil && o.Stage == "" {
		return nil, fmt.Errorf("a run with a dump names no stage for the unit as it reaches the first pass")
	}
	am := o.Analyses
	if am == nil {
		am = NewAnalysisManager[M](nil)
	}
	ensure := func(as []Analysis) error {
		for _, a := range as {
			run := p.Analyses[a]
			if _, err := getNamed(am, m, string(a), func(m M, _ *AnalysisManager[M]) (any, error) {
				run(m)
				return nil, nil
			}); err != nil {
				return err
			}
		}
		return nil
	}
	verify := func(who string) error {
		if p.Verify == nil {
			return nil
		}
		if err := ensure(p.VerifyRequires); err != nil {
			return fmt.Errorf("%s: %w", who, err)
		}
		if err := p.Verify(m); err != nil {
			return fmt.Errorf("%s: %w", who, err)
		}
		return nil
	}
	if o.Dump != nil {
		o.Dump(o.Stage, m)
	}
	var ran []string
	for _, ps := range p.Passes {
		if o.Enabled != nil && !o.Enabled(ps.Name) || ps.Applies != nil && !ps.Applies(x) {
			continue
		}
		if o.gate != nil && !o.gate(ps.Name) {
			continue
		}
		if err := ensure(ps.Requires); err != nil {
			return ran, fmt.Errorf("pass %s: %w", ps.Name, err)
		}
		if err := ps.Run(m, x); err != nil {
			return ran, fmt.Errorf("pass %s: %w", ps.Name, err)
		}
		am.Invalidate(ps.Preserves)
		ran = append(ran, ps.Name)
		if o.Dump != nil {
			o.Dump(ps.Name, m)
		}
		if o.ran != nil {
			o.ran(ps.Name, m)
		}
		if o.VerifyEach {
			if err := verify("after pass " + ps.Name); err != nil {
				return ran, err
			}
		}
	}
	if o.BuiltVerified && len(ran) == 0 {
		return ran, nil
	}
	if err := verify("verification"); err != nil {
		return ran, err
	}
	return ran, nil
}
