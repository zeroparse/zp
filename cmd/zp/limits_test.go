package main

import (
	"flag"
	"io"
	"testing"

	"zeroparse.dev/zp/runtime/parse"
)

func TestLimitFlagParsing(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want parse.Limits
		err  bool
	}{
		{nil, parse.Limits{}, false},
		{[]string{"-max-compute", "0"}, parse.Limits{}, false},
		{[]string{"-max-compute", "5000", "-max-input", "7"}, parse.Limits{Compute: 5000, Input: 7}, false},
		{[]string{"-max-compute", "unlimited"}, parse.Limits{Compute: parse.Unlimited}, false},
		{[]string{"-max-input", "Unlimited"}, parse.Limits{Input: parse.Unlimited}, false},
		{[]string{"-max-compute", "UNLIMITED", "-max-input", "unlimited"}, parse.Limits{Compute: parse.Unlimited, Input: parse.Unlimited}, false},
		{[]string{"-max-compute", "18446744073709551615"}, parse.Limits{Compute: parse.Unlimited}, false},
		{[]string{"-max-compute", "-1"}, parse.Limits{}, true},
		{[]string{"-max-compute", "1.5"}, parse.Limits{}, true},
		{[]string{"-max-input", "many"}, parse.Limits{}, true},
		{[]string{"-max-memory", "65536", "-max-compute", "9"}, parse.Limits{Compute: 9, Memory: 65536}, false},
		{[]string{"-max-memory", "unlimited"}, parse.Limits{Memory: parse.Unlimited}, false},
		{[]string{"-max-memory", "-1"}, parse.Limits{}, true},
		{[]string{"-max-compute", "18446744073709551616"}, parse.Limits{}, true},
		{[]string{"-max-depth", "40", "-max-input", "7"}, parse.Limits{Input: 7, Depth: 40}, false},
		{[]string{"-max-depth", "unlimited"}, parse.Limits{Depth: parse.Unlimited}, false},
		{[]string{"-max-depth", "-1"}, parse.Limits{}, true},
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
	v = limitValue(parse.Unlimited)
	if v.String() != "unlimited" {
		t.Errorf("Unlimited prints %q", v.String())
	}
	v = 42
	if v.String() != "42" {
		t.Errorf("42 prints %q", v.String())
	}
}
