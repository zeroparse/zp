package pipeline

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

func logging(name string, requires, invalidates []Analysis, after ...string) Pass[*unit, mode] {
	return Pass[*unit, mode]{Name: name, Requires: requires, Invalidates: invalidates, After: after, Obligation: name,
		Run: func(u *unit, _ mode) error { u.log = append(u.log, name); return nil }}
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
		logging("a", nil, []Analysis{"attrs"}),
		logging("b", []Analysis{"attrs"}, nil, "a"),
		logging("c", []Analysis{"attrs"}, nil),
		logging("d", nil, []Analysis{"attrs"}),
	)
	u := &unit{}
	ran, err := p.Run(u, mode{}, Options[*unit]{})
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
	lean := logging("lean-only", nil, []Analysis{"attrs"})
	lean.Applies = func(x mode) bool { return x.lean }
	p := fakePipeline(logging("a", []Analysis{"attrs"}, nil), lean, logging("b", nil, nil))
	u := &unit{}
	ran, err := p.Run(u, mode{}, Options[*unit]{Enabled: func(name string) bool { return name != "b" }})
	if err != nil || !slices.Equal(ran, []string{"a"}) || !slices.Equal(u.log, []string{"analyze", "a"}) {
		t.Fatalf("ran %v, log %v, err %v", ran, u.log, err)
	}
	u = &unit{}
	ran, err = p.Run(u, mode{lean: true}, Options[*unit]{})
	if err != nil || !slices.Equal(ran, []string{"a", "lean-only", "b"}) || !slices.Equal(u.log, []string{"analyze", "a", "lean-only", "b", "analyze"}) {
		t.Fatalf("ran %v, log %v, err %v", ran, u.log, err)
	}
}

// Dump sees the build, then each pass that ran, in order, before the pass is verified.
func TestDumpCallsEachPassInOrder(t *testing.T) {
	p := fakePipeline(logging("a", nil, nil), logging("b", nil, nil), logging("c", nil, nil))
	var seen []string
	_, err := p.Run(&unit{}, mode{}, Options[*unit]{
		Enabled: func(name string) bool { return name != "b" },
		Dump:    func(pass string, u *unit) { seen = append(seen, pass+":"+strings.Join(u.log, ",")) },
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"build:", "a:a", "c:a,c"}
	if !slices.Equal(seen, want) {
		t.Fatalf("dump %v, want %v", seen, want)
	}
}

// With VerifyEach the error names the first pass that broke an invariant; without it the break is
// reported at the end.
func TestVerificationNamesThePass(t *testing.T) {
	breaker := Pass[*unit, mode]{Name: "breaker", Obligation: "breaks", Run: func(u *unit, _ mode) error { u.broken = "breaker"; return nil }}
	p := fakePipeline(logging("a", nil, nil), breaker, logging("c", nil, nil))
	ran, err := p.Run(&unit{}, mode{}, Options[*unit]{VerifyEach: true})
	if err == nil || !strings.Contains(err.Error(), "after pass breaker") || !slices.Equal(ran, []string{"a", "breaker"}) {
		t.Fatalf("ran %v, err %v", ran, err)
	}
	ran, err = p.Run(&unit{}, mode{}, Options[*unit]{})
	if err == nil || !strings.HasPrefix(err.Error(), "verification:") || len(ran) != 3 {
		t.Fatalf("ran %v, err %v", ran, err)
	}
}

// A pass that fails is named, and a run does not go on.
func TestPassErrorIsNamed(t *testing.T) {
	bad := Pass[*unit, mode]{Name: "bad", Run: func(*unit, mode) error { return errors.New("boom") }}
	p := fakePipeline(logging("a", nil, nil), bad, logging("c", nil, nil))
	u := &unit{}
	_, err := p.Run(u, mode{}, Options[*unit]{})
	if err == nil || err.Error() != "pass bad: boom" || slices.Contains(u.log, "c") {
		t.Fatalf("err %v, log %v", err, u.log)
	}
}

// Declarations that cannot be run are reported with the pass.
func TestValidate(t *testing.T) {
	cases := []struct {
		name string
		p    *Pipeline[*unit, mode]
		want string
	}{
		{"order", fakePipeline(logging("b", nil, nil, "a"), logging("a", nil, nil)), `pass "b" must follow "a"`},
		{"duplicate", fakePipeline(logging("a", nil, nil), logging("a", nil, nil)), `pass "a" is declared twice`},
		{"unknown analysis", fakePipeline(logging("a", []Analysis{"nope"}, nil)), `pass a requires the analysis "nope"`},
		{"unnamed", fakePipeline(Pass[*unit, mode]{}), "no name"},
	}
	for _, c := range cases {
		err := c.p.Validate()
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", c.name, err, c.want)
		}
		if _, err := c.p.Run(&unit{}, mode{}, Options[*unit]{}); err == nil {
			t.Errorf("%s: Run accepted the pipeline", c.name)
		}
	}
	if err := fakePipeline(logging("a", nil, nil), logging("b", nil, nil, "a")).Validate(); err != nil {
		t.Errorf("valid pipeline: %v", err)
	}
	p := fakePipeline()
	p.VerifyRequires = []Analysis{"nope"}
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "verification requires") {
		t.Errorf("verification requirement: %v", err)
	}
}
