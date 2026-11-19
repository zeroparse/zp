package pass

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// unit is a unit whose analysis and passes append to a log, and whose invariant a pass can break.
type unit struct {
	log    []string
	broken string
}

type mode struct{ lean bool }

func (u *unit) analyze() { u.log = append(u.log, "analyze") }

// all and none are the preserved sets of a pass that leaves every analysis valid and of one that
// changes the unit.
var all, none = PreserveAll(), PreserveNone()

// with returns a copy of u that a pass changed: its log has name at the end.
func (u *unit) with(name string) *unit {
	v := *u
	v.log = append(slices.Clone(u.log), name)
	return &v
}

func logging(name string, requires []Analysis, preserves PreservedSet, after ...string) Pass[*unit, mode] {
	return Pass[*unit, mode]{Name: name, Requires: requires, After: after, Obligation: name,
		Run: func(u *unit, _ mode) (*unit, PreservedSet, error) { return u.with(name), preserves, nil }}
}

func fakePipeline(passes ...Pass[*unit, mode]) *Pipeline[*unit, mode] {
	return &Pipeline[*unit, mode]{
		Passes:         passes,
		Analyses:       map[Analysis]func(*unit){"attrs": (*unit).analyze},
		VerifyRequires: []Analysis{"attrs"},
		Verify: func(u *unit) error {
			if u.broken != "" {
				return errors.New("broken by " + u.broken)
			}
			return nil
		},
	}
}

// A required analysis that is stale is computed before the pass, and one a pass invalidated is
// computed again for the next pass that requires it, or for the final verification.
func TestStaleAnalysesAreRecomputed(t *testing.T) {
	p := fakePipeline(
		logging("a", nil, none),
		logging("b", []Analysis{"attrs"}, all, "a"),
		logging("c", []Analysis{"attrs"}, all),
		logging("d", nil, none),
	)
	u, ran, err := p.Run(&unit{}, mode{}, Options[*unit]{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a", "analyze", "b", "c", "d", "analyze"}
	if !slices.Equal(u.log, want) {
		t.Fatalf("log %v, want %v", u.log, want)
	}
	if !slices.Equal(ran, []string{"a", "b", "c", "d"}) {
		t.Fatalf("ran %v", ran)
	}
}

// A pass that is disabled or does not apply to the mode never runs, and what it would have
// invalidated stays valid.
func TestDisabledAndInapplicablePassesDoNotRun(t *testing.T) {
	lean := logging("lean-only", nil, none)
	lean.Applies = func(x mode) bool { return x.lean }
	p := fakePipeline(logging("a", []Analysis{"attrs"}, all), lean, logging("b", nil, all))
	u, ran, err := p.Run(&unit{}, mode{}, Options[*unit]{Enabled: func(name string) bool { return name != "b" }})
	if err != nil || !slices.Equal(ran, []string{"a"}) || !slices.Equal(u.log, []string{"analyze", "a"}) {
		t.Fatalf("ran %v, log %v, err %v", ran, u.log, err)
	}
	u, ran, err = p.Run(&unit{}, mode{lean: true}, Options[*unit]{})
	if err != nil || !slices.Equal(ran, []string{"a", "lean-only", "b"}) || !slices.Equal(u.log, []string{"analyze", "a", "lean-only", "b", "analyze"}) {
		t.Fatalf("ran %v, log %v, err %v", ran, u.log, err)
	}
}

// Dump sees the stage, then each pass that ran, in order, before the pass is verified.
func TestDumpCallsEachPassInOrder(t *testing.T) {
	p := fakePipeline(logging("a", nil, all), logging("b", nil, all), logging("c", nil, all))
	var seen []string
	_, _, err := p.Run(&unit{}, mode{}, Options[*unit]{
		Enabled: func(name string) bool { return name != "b" },
		Dump:    func(pass string, u *unit) { seen = append(seen, pass+":"+strings.Join(u.log, ",")) },
		Stage:   "built",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"built:", "a:a", "c:a,c"}
	if !slices.Equal(seen, want) {
		t.Fatalf("dump %v, want %v", seen, want)
	}
	// A dump without a stage would print the unit as built unnamed: the run refuses it before any pass.
	u, ran, err := p.Run(&unit{}, mode{}, Options[*unit]{Dump: func(string, *unit) {}})
	if err == nil || !strings.Contains(err.Error(), "names no stage") || len(ran) != 0 || len(u.log) != 0 {
		t.Errorf("a dump without a stage: ran %v, log %v, err %v", ran, u.log, err)
	}
}

// With VerifyEach the error names the first pass that broke an invariant, and the run returns the
// unit before it; without it the break is reported at the end, and the run returns its input,
// unchanged.
func TestVerificationNamesThePass(t *testing.T) {
	breaker := Pass[*unit, mode]{Name: "breaker", Obligation: "breaks", Run: func(u *unit, _ mode) (*unit, PreservedSet, error) {
		v := u.with("breaker")
		v.broken = "breaker"
		return v, none, nil
	}}
	p := fakePipeline(logging("a", nil, all), breaker, logging("c", nil, all))
	u, ran, err := p.Run(&unit{}, mode{}, Options[*unit]{VerifyEach: true})
	if err == nil || !strings.Contains(err.Error(), "after pass breaker") || !slices.Equal(ran, []string{"a", "breaker"}) {
		t.Fatalf("ran %v, err %v", ran, err)
	}
	if u.broken != "" || !slices.Equal(u.log, []string{"a", "analyze"}) {
		t.Errorf("after the broken pass the run returned the unit with log %v, broken %q; want the unit before it", u.log, u.broken)
	}
	in := &unit{}
	u, ran, err = p.Run(in, mode{}, Options[*unit]{})
	if err == nil || !strings.HasPrefix(err.Error(), "verification:") || len(ran) != 3 {
		t.Fatalf("ran %v, err %v", ran, err)
	}
	if u != in || len(in.log) != 0 {
		t.Errorf("after the final verification failed the run returned %v with log %v; want its input, unchanged", u, in.log)
	}
}

// A pass that fails is named, a run does not go on, and it returns the unit as the passes before
// the failed one left it, which no pass changed: neither the input of the run nor the input of the
// failed pass.
func TestPassErrorIsNamed(t *testing.T) {
	var seen *unit
	bad := Pass[*unit, mode]{Name: "bad", Run: func(u *unit, _ mode) (*unit, PreservedSet, error) {
		seen = u
		return u.with("bad"), none, errors.New("boom")
	}}
	p := fakePipeline(logging("a", nil, all), bad, logging("c", nil, all))
	in := &unit{}
	u, _, err := p.Run(in, mode{}, Options[*unit]{})
	if err == nil || err.Error() != "pass bad: boom" || slices.Contains(u.log, "c") {
		t.Fatalf("err %v, log %v", err, u.log)
	}
	if u != seen || !slices.Equal(u.log, []string{"a"}) || len(in.log) != 0 {
		t.Errorf("returned the log %v (the input of bad: %v), input log %v; want [a], the input of bad, and []", u.log, u == seen, in.log)
	}
}

// Declarations that cannot be run are reported with the pass.
func TestValidate(t *testing.T) {
	cases := []struct {
		name string
		p    *Pipeline[*unit, mode]
		want string
	}{
		{"order", fakePipeline(logging("b", nil, all, "a"), logging("a", nil, all)), `pass "b" must follow "a"`},
		{"duplicate", fakePipeline(logging("a", nil, all), logging("a", nil, all)), `pass "a" is declared twice`},
		{"unknown analysis", fakePipeline(logging("a", []Analysis{"nope"}, all)), `pass a requires the analysis "nope"`},
		{"unnamed", fakePipeline(Pass[*unit, mode]{}), "no name"},
	}
	for _, c := range cases {
		err := c.p.Validate()
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", c.name, err, c.want)
		}
		if _, _, err := c.p.Run(&unit{}, mode{}, Options[*unit]{}); err == nil {
			t.Errorf("%s: Run accepted the pipeline", c.name)
		}
	}
	if err := fakePipeline(logging("a", nil, all), logging("b", nil, all, "a")).Validate(); err != nil {
		t.Errorf("valid pipeline: %v", err)
	}
	p := fakePipeline()
	p.VerifyRequires = []Analysis{"nope"}
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "verification requires") {
		t.Errorf("verification requirement: %v", err)
	}
}

// A unit that its builder verified is not verified again when no pass runs, and is verified at the
// end as usual once a pass has run.
func TestBuiltVerifiedSkipsOnlyAnUnchangedUnit(t *testing.T) {
	p := fakePipeline(logging("a", nil, all))
	off := func(string) bool { return false }
	if _, _, err := p.Run(&unit{broken: "builder"}, mode{}, Options[*unit]{Enabled: off, BuiltVerified: true}); err != nil {
		t.Errorf("an unchanged unit was verified again: %v", err)
	}
	if _, _, err := p.Run(&unit{broken: "builder"}, mode{}, Options[*unit]{Enabled: off}); err == nil {
		t.Error("without BuiltVerified the final verification did not run")
	}
	if _, _, err := p.Run(&unit{broken: "builder"}, mode{}, Options[*unit]{BuiltVerified: true}); err == nil || !strings.HasPrefix(err.Error(), "verification:") {
		t.Errorf("a unit that a pass ran on was not verified: %v", err)
	}
}

// A pass that declares nothing preserves nothing: the next pass that requires an analysis gets it
// computed again.
func TestUndeclaredPreservationRecomputes(t *testing.T) {
	undeclared := Pass[*unit, mode]{Name: "undeclared", Obligation: "undeclared",
		Run: func(u *unit, _ mode) (*unit, PreservedSet, error) { return u.with("undeclared"), PreservedSet{}, nil }}
	p := fakePipeline(logging("a", []Analysis{"attrs"}, all), undeclared, logging("b", []Analysis{"attrs"}, all))
	u, _, err := p.Run(&unit{}, mode{}, Options[*unit]{})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"analyze", "a", "undeclared", "analyze", "b"}; !slices.Equal(u.log, want) {
		t.Fatalf("log %v, want %v", u.log, want)
	}
}

// A run with the caller's manager keeps its analyses there, for the unit the run returns: a later
// run on that unit that preserves them does not compute them again, and the manager counts the
// computations. A run on another unit computes them for it.
func TestRunSharesTheCallersManager(t *testing.T) {
	am := NewAnalysisManager[*unit](nil)
	p := fakePipeline(logging("a", []Analysis{"attrs"}, all))
	in := &unit{}
	u := in
	for range 2 {
		var err error
		if u, _, err = p.Run(u, mode{}, Options[*unit]{Analyses: am}); err != nil {
			t.Fatal(err)
		}
	}
	if want := []string{"analyze", "a", "a"}; !slices.Equal(u.log, want) {
		t.Fatalf("log %v, want %v", u.log, want)
	}
	if am.Computations("attrs") != 1 {
		t.Fatalf("computations %d, want 1", am.Computations("attrs"))
	}
	if _, _, err := p.Run(in, mode{}, Options[*unit]{Analyses: am}); err != nil {
		t.Fatal(err)
	}
	if am.Computations("attrs") != 2 {
		t.Fatalf("computations after a run on another unit %d, want 2", am.Computations("attrs"))
	}
}
