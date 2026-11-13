package main

import (
	"flag"
	"io"
	"testing"

	"zeroparse.dev/zp/core"
)

func TestLimitFlagParsing(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want core.Limits
		err  bool
	}{
		{nil, core.Limits{}, false},
		{[]string{"-max-compute", "0"}, core.Limits{}, false},
		{[]string{"-max-compute", "5000", "-max-input", "7"}, core.Limits{Compute: 5000, Input: 7}, false},
		{[]string{"-max-compute", "unlimited"}, core.Limits{Compute: core.Unlimited}, false},
		{[]string{"-max-input", "Unlimited"}, core.Limits{Input: core.Unlimited}, false},
		{[]string{"-max-compute", "UNLIMITED", "-max-input", "unlimited"}, core.Limits{Compute: core.Unlimited, Input: core.Unlimited}, false},
		{[]string{"-max-compute", "18446744073709551615"}, core.Limits{Compute: core.Unlimited}, false},
		{[]string{"-max-compute", "-1"}, core.Limits{}, true},
		{[]string{"-max-compute", "1.5"}, core.Limits{}, true},
		{[]string{"-max-input", "many"}, core.Limits{}, true},
		{[]string{"-max-memory", "65536", "-max-compute", "9"}, core.Limits{Compute: 9, Memory: 65536}, false},
		{[]string{"-max-memory", "unlimited"}, core.Limits{Memory: core.Unlimited}, false},
		{[]string{"-max-memory", "-1"}, core.Limits{}, true},
		{[]string{"-max-compute", "18446744073709551616"}, core.Limits{}, true},
		{[]string{"-max-depth", "40", "-max-input", "7"}, core.Limits{Input: 7, Depth: 40}, false},
		{[]string{"-max-depth", "unlimited"}, core.Limits{Depth: core.Unlimited}, false},
		{[]string{"-max-depth", "-1"}, core.Limits{}, true},
	} {
		fs := flag.NewFlagSet("t", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var lim limitFlags
		lim.register(fs)
		err := fs.Parse(tc.args)
		if (err != nil) != tc.err {
			t.Errorf("%v: error %v", tc.args, err)
			continue
		}
		if err == nil && lim.limits() != tc.want {
			t.Errorf("%v: limits %+v, want %+v", tc.args, lim.limits(), tc.want)
		}
	}
	var v limitValue
	if v.String() != "0" {
		t.Errorf("zero prints %q", v.String())
	}
	v = limitValue(core.Unlimited)
	if v.String() != "unlimited" {
		t.Errorf("Unlimited prints %q", v.String())
	}
	v = 42
	if v.String() != "42" {
		t.Errorf("42 prints %q", v.String())
	}
}
