package corepass

import (
	"slices"
	"testing"

	"zeroparse.dev/zp/internal/ir/core"
)

// The declarations of Pipeline can be run: names are distinct, every pass states what it preserves,
// the analyses exist, and the order satisfies every After.
func TestPipelineIsValid(t *testing.T) {
	if err := Pipeline.Validate(); err != nil {
		t.Fatal(err)
	}
	want := []string{PassGuards, PassScans}
	if got := Pipeline.Names(); !slices.Equal(got, want) {
		t.Fatalf("passes %v, want %v", got, want)
	}
	for _, p := range Pipeline.Passes {
		if p.Obligation == "" {
			t.Errorf("pass %s states no obligation", p.Name)
		}
	}
}

// The passes know no mode: a flag that is off removes its pass, and nothing else does.
func TestPipelineAppliesByFlag(t *testing.T) {
	all := []string{PassGuards, PassScans}
	off := func(names ...string) func(string) bool {
		return func(name string) bool { return !slices.Contains(names, name) }
	}
	cases := []struct {
		name string
		env  Env
		want []string
	}{
		{"default", Env{}, all},
		{"without guards", Env{Flag: off(PassGuards)}, []string{PassScans}},
		{"without scans", Env{Flag: off(PassScans)}, []string{PassGuards}},
		{"nothing", Env{Flag: func(string) bool { return false }}, nil},
	}
	for _, c := range cases {
		env := c.env
		m := &core.Module{}
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
