package pass

import (
	"fmt"
	"io"
)

// The pipeline of a compilation (design pass-pipeline, "Declared Passes").
//
// A compilation is one pipeline, a typed composition of stages: a Stage[I, O] takes a unit of I and
// returns a unit of O. Transforms makes the passes of a Pipeline[M, X] a Stage[M, M]; Lower makes a
// lowering, a required pass from one IR or stage to another, a Stage[I, O]; Then composes two stages
// whose types meet, so the Go type checker ensures that every pass receives the unit that the
// previous one returns. Every step of a stage has a Step, a description that does not depend on its
// types, which the observers, the dumps and the listing of the pipeline read.
//
// One Run manages a compilation: it says which optional passes are on, verifies after each pass on
// request, numbers the runs of optional passes in the order they happen and tells its observers
// about each step. After the first failure of a unit's run, only the required steps of the rest of
// its stages run, so that the unit still reaches the type of the last stage; the first error is the
// one returned.

// Step describes one step of a pipeline whatever its types.
type Step struct {
	// Name names the step: the name of a pass, or of a required step such as a lowering.
	Name string
	// Required reports that the step always runs: it has no flag, and no setting skips it.
	Required bool
	// In and Out are the stages of the unit that the step takes and returns, such as "core", "flow"
	// or "exec"; they are equal for a transform.
	In, Out string
	// Dump is the name under which the unit after the step is dumped (-print-after): the name of a
	// pass, the output stage of a lowering, or "" for a step that is never dumped.
	Dump string
}

// Label names the unit of a run in the dumps and the events: the table it belongs to and, for a
// unit that is one function, the function.
type Label struct {
	Table, Func string
}

// IR describes a unit type M at a stage to the manager: how to verify it and how to dump it.
type IR[M any] struct {
	// Stage names the stage, as Step.In and Step.Out do.
	Stage string
	// Verify checks the invariants of a unit at the stage; nil checks none.
	Verify func(m M) error
	// Print writes the dump of the unit m with the label l after the step whose dump name is after.
	Print func(w io.Writer, l Label, after string, m M)
}

// Event is one step of a run, as the observers see it.
type Event struct {
	// Seq numbers the runs of optional passes of the compilation from 1, in the order they are
	// decided; it is 0 for a required step and for the dump of a unit as it reaches its stage.
	Seq   int
	Step  Step
	Label Label
}

// An Observer watches the runs of a compilation. Dumps and test hooks are observers.
type Observer interface {
	// Before is asked before an optional pass that is enabled and applies runs; false skips the run.
	Before(e Event) bool
	// After is told of each step that ran, and of the unit as it reaches a stage that dumps it then
	// (Transform.Entry, an Event whose Step has the stage as its Dump and no Name). print writes the
	// dump of the unit, and unit is the unit itself, for an observer that knows its type.
	After(e Event, print func(w io.Writer), unit any)
}

// Run manages one compilation. The zero value runs every applicable pass, verifies where a stage
// ends and tells no one.
type Run struct {
	// Enabled, if not nil, reports whether the optional pass of the given name may run.
	Enabled func(name string) bool
	// VerifyEach verifies the unit after every pass that ran.
	VerifyEach bool
	// Observers are told of every step in order.
	Observers []Observer
	// seq is the number of the last run of an optional pass.
	seq int
}

// Runs returns how many runs of optional passes the compilation has numbered.
func (r *Run) Runs() int { return r.seq }

func (r *Run) after(e Event, print func(io.Writer), unit any) {
	for _, o := range r.Observers {
		o.After(e, print, unit)
	}
}

// state is the state of the run of one unit through the stages.
type state struct {
	err    error // the first failure
	broken bool  // a required step failed, so the unit has no value of the next type
}

func (st *state) fail(err error) {
	if st.err == nil {
		st.err = err
	}
}

// Stage is a typed segment of the pipeline of a compilation: it takes a unit of I and returns a unit
// of O.
type Stage[I, O any] struct {
	steps []Step
	run   func(r *Run, l Label, in I, st *state) O
}

// Steps returns the steps of the stage in order.
func (s Stage[I, O]) Steps() []Step { return append([]Step(nil), s.steps...) }

// Check checks the composition: each step takes the stage that the previous one returns, and names
// are distinct and non-empty.
func (s Stage[I, O]) Check() error {
	seen := map[string]bool{}
	for i, st := range s.steps {
		if st.Name == "" {
			return fmt.Errorf("step %d of the pipeline has no name", i+1)
		}
		if seen[st.Name] {
			return fmt.Errorf("step %s is in the pipeline twice", st.Name)
		}
		seen[st.Name] = true
		if i > 0 && s.steps[i-1].Out != st.In {
			return fmt.Errorf("step %s takes the stage %q but follows %s, which returns %q", st.Name, st.In, s.steps[i-1].Name, s.steps[i-1].Out)
		}
	}
	return nil
}

// Run runs the stage on the unit in, labeled l, under r. After a failure only the required steps
// run; the first error is returned, with the unit as the steps left it, which is the zero O if a
// required step failed.
func (s Stage[I, O]) Run(r *Run, l Label, in I) (O, error) {
	var st state
	out := s.run(r, l, in, &st)
	return out, st.err
}

// Then composes a and b: b runs on the unit that a returns.
func Then[A, B, C any](a Stage[A, B], b Stage[B, C]) Stage[A, C] {
	return Stage[A, C]{
		steps: append(append([]Step(nil), a.steps...), b.steps...),
		run: func(r *Run, l Label, in A, st *state) C {
			mid := a.run(r, l, in, st)
			if st.broken {
				var zero C
				return zero
			}
			return b.run(r, l, mid, st)
		},
	}
}

// Transform declares a Stage of the passes of a Pipeline.
type Transform[M, X any] struct {
	// IR is the unit type at the stage of the pipeline.
	IR IR[M]
	// Pipeline holds the passes.
	Pipeline *Pipeline[M, X]
	// Env builds the context of the passes for the unit m.
	Env func(r *Run, l Label, m M) X
	// Entry, if not "", dumps the unit as it reaches the first pass under that name: the stage of a
	// unit that no lowering made, such as "core".
	Entry string
	// BuiltVerified reports that what made the unit verified it, so that a run in which no pass ran
	// does not verify it again.
	BuiltVerified bool
	// Analyses, if not nil, returns the manager of the analyses of the unit, which outlives the run;
	// nil runs with a manager of its own.
	Analyses func(m M) *AnalysisManager[M]
}

// Transforms makes the passes of t.Pipeline a stage. It skips them after a failure; otherwise it
// runs them as Pipeline.Run does, under the settings and the verification of the Run, numbering each
// run of an optional pass and telling the observers.
func Transforms[M, X any](t Transform[M, X]) Stage[M, M] {
	var steps []Step
	for _, ps := range t.Pipeline.Passes {
		steps = append(steps, Step{Name: ps.Name, In: t.IR.Stage, Out: t.IR.Stage, Dump: ps.Name})
	}
	return Stage[M, M]{steps: steps, run: func(r *Run, l Label, m M, st *state) M {
		if st.err != nil {
			return m
		}
		print := func(after string) func(io.Writer) {
			return func(w io.Writer) {
				if t.IR.Print != nil {
					t.IR.Print(w, l, after, m)
				}
			}
		}
		if t.Entry != "" {
			r.after(Event{Step: Step{In: t.IR.Stage, Out: t.IR.Stage, Dump: t.Entry}, Label: l}, print(t.Entry), m)
		}
		seqs := map[string]int{}
		step := func(name string) Step {
			return Step{Name: name, In: t.IR.Stage, Out: t.IR.Stage, Dump: name}
		}
		o := Options[M]{
			Enabled:       r.Enabled,
			VerifyEach:    r.VerifyEach,
			BuiltVerified: t.BuiltVerified,
			gate: func(name string) bool {
				r.seq++
				seqs[name] = r.seq
				e := Event{Seq: r.seq, Step: step(name), Label: l}
				for _, ob := range r.Observers {
					if !ob.Before(e) {
						return false
					}
				}
				return true
			},
			ran: func(name string, m M) {
				r.after(Event{Seq: seqs[name], Step: step(name), Label: l}, print(name), m)
			},
		}
		if t.Analyses != nil {
			o.Analyses = t.Analyses(m)
		}
		if _, err := t.Pipeline.Run(m, t.Env(r, l, m), o); err != nil {
			st.fail(err)
		}
		return m
	}}
}

// Lower makes a lowering a required stage from I to O: f lowers the unit, the verifier of out, the
// postcondition of the lowering, checks the result, and the result is dumped under out.Stage. A
// failure of either is a failure of the lowering (StepError), after which no later step runs.
func Lower[I, O any](name, in string, out IR[O], f func(I) (O, error)) Stage[I, O] {
	s := Step{Name: name, Required: true, In: in, Out: out.Stage, Dump: out.Stage}
	return Stage[I, O]{steps: []Step{s}, run: func(r *Run, l Label, u I, st *state) O {
		var zero O
		o, err := f(u)
		if err == nil && out.Verify != nil {
			err = out.Verify(o)
		}
		if err != nil {
			st.fail(&StepError{Step: s, Err: err})
			st.broken = true
			return zero
		}
		r.after(Event{Step: s, Label: l}, func(w io.Writer) {
			if out.Print != nil {
				out.Print(w, l, s.Dump, o)
			}
		}, o)
		return o
	}}
}

// Do makes f a required step on a unit at the stage of ir that changes the unit in place and is
// never dumped. It runs after a failure too.
func Do[M any](name string, ir IR[M], f func(m M)) Stage[M, M] {
	s := Step{Name: name, Required: true, In: ir.Stage, Out: ir.Stage}
	return Stage[M, M]{steps: []Step{s}, run: func(r *Run, l Label, m M, st *state) M {
		f(m)
		r.after(Event{Step: s, Label: l}, func(io.Writer) {}, m)
		return m
	}}
}

// Project runs the stage s on the part of a unit M that get returns, and returns the unit.
func Project[M, P any](get func(m M) P, s Stage[P, P]) Stage[M, M] {
	return Stage[M, M]{steps: s.steps, run: func(r *Run, l Label, m M, st *state) M {
		s.run(r, l, get(m), st)
		return m
	}}
}

// StepError is the failure of a required step, which it names.
type StepError struct {
	Step Step
	Err  error
}

// Error is the message of the failure itself: the lowering adds no words to it.
func (e *StepError) Error() string { return e.Err.Error() }

func (e *StepError) Unwrap() error { return e.Err }
