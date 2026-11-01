package main

// The -max-compute, -max-input and -max-depth flags of the commands that run a parse.

import (
	"flag"
	"fmt"
	"strconv"
	"strings"

	"zeroparse.dev/zp"
)

// limitValue is the value of a limit flag: a decimal number, where 0 selects the default of the
// limit, or the word unlimited, which disables it.
type limitValue uint64

func (v *limitValue) String() string {
	if v == nil {
		return "0"
	}
	if uint64(*v) == zp.Unlimited {
		return "unlimited"
	}
	return strconv.FormatUint(uint64(*v), 10)
}

func (v *limitValue) Set(s string) error {
	if strings.EqualFold(s, "unlimited") {
		*v = limitValue(zp.Unlimited)
		return nil
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return fmt.Errorf("want a non-negative integer or unlimited, got %q", s)
	}
	*v = limitValue(n)
	return nil
}

// limitFlags are the flags that bound a parse.
type limitFlags struct {
	compute, maxInput, depth limitValue
}

// register adds the limit flags to fs.
func (l *limitFlags) register(fs *flag.FlagSet) {
	fs.Var(&l.compute, "max-compute", "limit on the cost of the parse: a number of units, 0 for the default of max(2^20, 1024 times the input length), or unlimited")
	fs.Var(&l.maxInput, "max-input", "longest input accepted, in the position unit: a number, 0 or unlimited for no bound")
	fs.Var(&l.depth, "max-depth", "limit on the nesting of recursive rule calls: a number, 0 for the backend's default (100000; 10000000 for bytecode-iterative), or unlimited (bytecode-iterative only)")
}

// limits returns the limits the flags select.
func (l *limitFlags) limits() zp.Limits {
	return zp.Limits{Compute: uint64(l.compute), Input: uint64(l.maxInput), Depth: uint64(l.depth)}
}

// option returns the parse option that applies the limits.
func (l *limitFlags) option() zp.ParseOption { return zp.WithLimits(l.limits()) }
