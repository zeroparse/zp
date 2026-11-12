package coreir

import (
	"slices"
	"testing"
)

// The declarations of Pipeline can be run: names are distinct, every pass states what it preserves,
// the analyses exist, and the order satisfies every After.
func TestPipelineIsValid(t *testing.T) {
	if err := Pipeline.Validate(); err != nil {
		t.Fatal(err)
	}
	want := []string{PassTerminals, PassGuards, PassScans, PassClassRuns}
	if got := Pipeline.Names(); !slices.Equal(got, want) {
		t.Fatalf("passes %v, want %v", got, want)
	}
	for _, p := range Pipeline.Passes {
		if p.Obligation == "" {
			t.Errorf("pass %s states no obligation", p.Name)
		}
	}
}

// The passes that run depend on the mode, and a flag that is off removes its pass but never adds a
// pass that does not apply.
func TestPipelineAppliesByMode(t *testing.T) {
	all := []string{PassTerminals, PassGuards, PassScans, PassClassRuns}
	off := func(names ...string) func(string) bool {
		return func(name string) bool { return !slices.Contains(names, name) }
	}
	cases := []struct {
		name string
		env  Env
		want []string
	}{
		{"lean", Env{Mode: Mode{Lean: true, Ordinary: true}}, all},
		{"lean observable", Env{Mode: Mode{Lean: true, Ordinary: true, Observe: true}}, []string{PassGuards, PassScans, PassClassRuns}},
		{"controlled", Env{}, []string{PassGuards, PassScans}},
		{"recognizer", Env{Mode: Mode{Ordinary: true}}, []string{PassGuards, PassScans}},
		{"lean without guards", Env{Mode: Mode{Lean: true, Ordinary: true}, Flag: off(PassGuards)}, []string{PassTerminals, PassScans, PassClassRuns}},
		{"controlled with class runs on", Env{Flag: func(string) bool { return true }}, []string{PassGuards, PassScans}},
		{"nothing", Env{Mode: Mode{Lean: true, Ordinary: true}, Flag: func(string) bool { return false }}, nil},
	}
	for _, c := range cases {
		env := c.env
		env.Local = func(*Rule) bool { return true }
		m := &Module{}
		got, err := Run(m, &env, RunOptions{})
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: ran %v, want %v", c.name, got, c.want)
		}
	}
}
