// Package pipeline runs declared passes over an IR unit (design 063).
//
// A Pass names itself, states the analyses it reads and the analyses it makes stale, the passes it
// must follow, the generation modes it applies to and what it preserves. A Pipeline is an ordered list
// of passes. Run executes the enabled, applicable passes in that order: before each pass it makes the
// pass's required analyses valid, after it marks the invalidated ones stale, and it verifies the unit,
// once at the end or after every pass. A pass that breaks an invariant is named in the error.
//
// The package knows nothing about the unit M or the context X that its callers choose: Core IR is the
// first user (internal/ir/coreir, Pipeline). It holds no state between runs.
package pipeline

import "fmt"

// Analysis names data about a unit that passes read and that other passes make stale. The
// pipeline's Analyses say how to compute each.
type Analysis string

// A Pass is one transformation of a unit M under the context X of the generation mode.
type Pass[M, X any] struct {
	// Name identifies the pass. It is also the name of the flag that turns the pass off and the
	// name that the dump and the errors give.
	Name string
	// Requires lists the analyses that must be valid when the pass starts.
	Requires []Analysis
	// Invalidates lists the analyses that the pass makes stale.
	Invalidates []Analysis
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
	// Dump, if not nil, is called with "build" before the first pass and with the name of each pass
	// after it ran, before it is verified.
	Dump func(pass string, m M)
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
	// Every analysis starts stale: a unit comes from the builder without them.
	valid := map[Analysis]bool{}
	ensure := func(as []Analysis) {
		for _, a := range as {
			if !valid[a] {
				p.Analyses[a](m)
				valid[a] = true
			}
		}
	}
	verify := func(who string) error {
		if p.Verify == nil {
			return nil
		}
		ensure(p.VerifyRequires)
		if err := p.Verify(m); err != nil {
			return fmt.Errorf("%s: %w", who, err)
		}
		return nil
	}
	if o.Dump != nil {
		o.Dump("build", m)
	}
	var ran []string
	for _, ps := range p.Passes {
		if o.Enabled != nil && !o.Enabled(ps.Name) || ps.Applies != nil && !ps.Applies(x) {
			continue
		}
		ensure(ps.Requires)
		if err := ps.Run(m, x); err != nil {
			return ran, fmt.Errorf("pass %s: %w", ps.Name, err)
		}
		for _, a := range ps.Invalidates {
			valid[a] = false
		}
		ran = append(ran, ps.Name)
		if o.Dump != nil {
			o.Dump(ps.Name, m)
		}
		if o.VerifyEach {
			if err := verify("after pass " + ps.Name); err != nil {
				return ran, err
			}
		}
	}
	if err := verify("verification"); err != nil {
		return ran, err
	}
	return ran, nil
}
