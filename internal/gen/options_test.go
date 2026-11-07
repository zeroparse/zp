package gen_test

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gen"
)

// Optimizations applies the profile, then Opt, then NoOpt, so an optimization that is both given a
// value and turned off is off, and a value outside what the optimization takes is an error.
func TestGenOptionsOptimizationsOrder(t *testing.T) {
	t.Parallel()
	s, err := gen.GenOptions{}.Optimizations()
	if err != nil {
		t.Fatal(err)
	}
	if !s.On("guards") || s.Value("class-tables") != 5 || s.On("ts-literal-sharing") {
		t.Errorf("the default profile: guards %v, class-tables %d, ts-literal-sharing %v", s.On("guards"), s.Value("class-tables"), s.On("ts-literal-sharing"))
	}

	s, err = gen.GenOptions{
		Opt:   map[string]int{"class-tables": 3, "ts-literal-sharing": 1, " scans ": 0, "inline-literals": 2},
		NoOpt: []string{"inline-literals", " guards"},
	}.Optimizations()
	if err != nil {
		t.Fatal(err)
	}
	if s.Value("class-tables") != 3 || !s.On("ts-literal-sharing") {
		t.Errorf("Opt did not give its values: class-tables %d, ts-literal-sharing %v", s.Value("class-tables"), s.On("ts-literal-sharing"))
	}
	if s.On("scans") {
		t.Errorf("Opt scans=0 left scans on")
	}
	if s.On("inline-literals") || s.On("guards") {
		t.Errorf("NoOpt did not win: inline-literals %d, guards %v", s.Value("inline-literals"), s.On("guards"))
	}

	for _, c := range []struct {
		opts gen.GenOptions
		want string
	}{
		{gen.GenOptions{Opt: map[string]int{"nosuch": 1}}, `unknown optimization "nosuch"`},
		{gen.GenOptions{Opt: map[string]int{"guards": 2}}, "does not take the value 2"},
		{gen.GenOptions{Opt: map[string]int{"class-tables": -1}}, "does not take the value -1"},
		{gen.GenOptions{Opt: map[string]int{"guards": 1, " guards ": 0}}, "optimization guards is given twice"},
		{gen.GenOptions{NoOpt: []string{"nosuch"}}, `unknown optimization "nosuch"`},
		{gen.GenOptions{Optimize: "nosuch"}, "unknown optimization profile"},
	} {
		if _, err := c.opts.Optimizations(); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%+v: %v, want an error with %q", c.opts, err, c.want)
		}
	}
}
