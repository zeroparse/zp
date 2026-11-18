package main

import (
	"flag"
	"io"
	"testing"

	"zeroparse.dev/zp/pkg/runtime"
)

func TestLimitFlagParsing(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want runtime.Limits
		err  bool
	}{
		{nil, runtime.Limits{}, false},
		{[]string{"-max-compute", "0"}, runtime.Limits{}, false},
		{[]string{"-max-compute", "5000", "-max-input", "7"}, runtime.Limits{Compute: 5000, Input: 7}, false},
		{[]string{"-max-compute", "unlimited"}, runtime.Limits{Compute: runtime.Unlimited}, false},
		{[]string{"-max-input", "Unlimited"}, runtime.Limits{Input: runtime.Unlimited}, false},
		{[]string{"-max-compute", "UNLIMITED", "-max-input", "unlimited"}, runtime.Limits{Compute: runtime.Unlimited, Input: runtime.Unlimited}, false},
		{[]string{"-max-compute", "18446744073709551615"}, runtime.Limits{Compute: runtime.Unlimited}, false},
		{[]string{"-max-compute", "-1"}, runtime.Limits{}, true},
		{[]string{"-max-compute", "1.5"}, runtime.Limits{}, true},
		{[]string{"-max-input", "many"}, runtime.Limits{}, true},
		{[]string{"-max-memory", "65536", "-max-compute", "9"}, runtime.Limits{Compute: 9, Memory: 65536}, false},
		{[]string{"-max-memory", "unlimited"}, runtime.Limits{Memory: runtime.Unlimited}, false},
		{[]string{"-max-memory", "-1"}, runtime.Limits{}, true},
		{[]string{"-max-compute", "18446744073709551616"}, runtime.Limits{}, true},
		{[]string{"-max-depth", "40", "-max-input", "7"}, runtime.Limits{Input: 7, Depth: 40}, false},
		{[]string{"-max-depth", "unlimited"}, runtime.Limits{Depth: runtime.Unlimited}, false},
		{[]string{"-max-depth", "-1"}, runtime.Limits{}, true},
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
	v = limitValue(runtime.Unlimited)
	if v.String() != "unlimited" {
		t.Errorf("Unlimited prints %q", v.String())
	}
	v = 42
	if v.String() != "42" {
		t.Errorf("42 prints %q", v.String())
	}
}
