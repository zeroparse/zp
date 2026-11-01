package main

import (
	"flag"
	"io"
	"testing"

	"zeroparse.dev/zp"
)

func TestLimitFlagParsing(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want zp.Limits
		err  bool
	}{
		{nil, zp.Limits{}, false},
		{[]string{"-max-compute", "0"}, zp.Limits{}, false},
		{[]string{"-max-compute", "5000", "-max-input", "7"}, zp.Limits{Compute: 5000, Input: 7}, false},
		{[]string{"-max-compute", "unlimited"}, zp.Limits{Compute: zp.Unlimited}, false},
		{[]string{"-max-input", "Unlimited"}, zp.Limits{Input: zp.Unlimited}, false},
		{[]string{"-max-compute", "UNLIMITED", "-max-input", "unlimited"}, zp.Limits{Compute: zp.Unlimited, Input: zp.Unlimited}, false},
		{[]string{"-max-compute", "18446744073709551615"}, zp.Limits{Compute: zp.Unlimited}, false},
		{[]string{"-max-compute", "-1"}, zp.Limits{}, true},
		{[]string{"-max-compute", "1.5"}, zp.Limits{}, true},
		{[]string{"-max-input", "many"}, zp.Limits{}, true},
		{[]string{"-max-compute", "18446744073709551616"}, zp.Limits{}, true},
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
	v = limitValue(zp.Unlimited)
	if v.String() != "unlimited" {
		t.Errorf("Unlimited prints %q", v.String())
	}
	v = 42
	if v.String() != "42" {
		t.Errorf("42 prints %q", v.String())
	}
}
