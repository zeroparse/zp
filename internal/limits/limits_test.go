package limits

import (
	"errors"
	"math"
	"testing"

	"zeroparse.dev/zp/runtime/parse"
)

func TestLimitError(t *testing.T) {
	for _, tc := range []struct {
		err  *parse.LimitError
		want string
	}{
		{nil, "parse limit exceeded"},
		{&parse.LimitError{Kind: parse.LimitCompute, Limit: 5, Used: 4, Requested: 2}, "parse compute limit exceeded: limit 5, used 4, requested 2"},
		{&parse.LimitError{Kind: parse.LimitInput, Limit: 5, Requested: 9}, "parse input limit exceeded: limit 5, length 9"},
		{&parse.LimitError{Kind: parse.LimitDepth, Limit: 7, Used: 7, Requested: 1}, "nesting too deep: more than 7 nested recursive rule calls"},
		{&parse.LimitError{Kind: parse.LimitMemory, Limit: 100, Used: 140}, "parse memory limit exceeded: limit 100, used 140"},
	} {
		if got := tc.err.Error(); got != tc.want {
			t.Errorf("Error() = %q, want %q", got, tc.want)
		}
	}
	var err error = &parse.LimitError{Kind: parse.LimitCompute}
	if !errors.Is(err, parse.ErrLimit) {
		t.Error("errors.Is(LimitError, ErrLimit) = false")
	}
	if errors.Is(err, errors.New("parse limit exceeded")) {
		t.Error("LimitError matched an unrelated error with the same text")
	}
}

// A later WithLimits overrides the nonzero fields only.
func TestMerge(t *testing.T) {
	for _, tc := range []struct{ l, o, want parse.Limits }{
		{parse.Limits{}, parse.Limits{}, parse.Limits{}},
		{parse.Limits{Compute: 1, Input: 2, Depth: 3}, parse.Limits{}, parse.Limits{Compute: 1, Input: 2, Depth: 3}},
		{parse.Limits{Compute: 1, Input: 2}, parse.Limits{Depth: 3}, parse.Limits{Compute: 1, Input: 2, Depth: 3}},
		{parse.Limits{Compute: 1, Input: 2, Depth: 3}, parse.Limits{Input: parse.Unlimited, Depth: 9}, parse.Limits{Compute: 1, Input: parse.Unlimited, Depth: 9}},
		{parse.Limits{Compute: 1, Memory: 4}, parse.Limits{Depth: 3}, parse.Limits{Compute: 1, Memory: 4, Depth: 3}},
		{parse.Limits{Memory: 4}, parse.Limits{Memory: parse.Unlimited}, parse.Limits{Memory: parse.Unlimited}},
	} {
		if got := Merge(tc.l, tc.o); got != tc.want {
			t.Errorf("Merge(%+v, %+v) = %+v, want %+v", tc.l, tc.o, got, tc.want)
		}
	}
}

func TestArithmetic(t *testing.T) {
	for _, tc := range []struct {
		used, requested, limit uint64
		want                   bool
	}{
		{0, 5, 0, false},
		{4, 1, 5, false},
		{4, 2, 5, true},
		{6, 0, 5, true},
		{1, math.MaxUint64, 5, true},
	} {
		if got := ExceedsLimit(tc.used, tc.requested, tc.limit); got != tc.want {
			t.Errorf("ExceedsLimit(%d, %d, %d) = %v, want %v", tc.used, tc.requested, tc.limit, got, tc.want)
		}
	}
	if !ReachesInterval(0, 256, 256) || ReachesInterval(0, 255, 256) || !ReachesInterval(300, 0, 256) || !ReachesInterval(1, math.MaxUint64, 256) {
		t.Error("ReachesInterval disagrees with used+requested >= interval")
	}
	if SaturatingAdd(math.MaxUint64-1, 2) != math.MaxUint64 || SaturatingAdd(2, 3) != 5 {
		t.Error("SaturatingAdd does not saturate exactly at overflow")
	}
}

func TestDefaultBudget(t *testing.T) {
	for _, tc := range []struct{ n, want uint64 }{
		{0, DefaultBudgetFloor},
		{1024, DefaultBudgetFloor},
		{1025, 1025 * 1024},
		{1 << 30, 1 << 40},
		{math.MaxUint64, math.MaxUint64 - 1},
		{math.MaxUint64 / 1024, (math.MaxUint64 / 1024) * 1024},
	} {
		if got := DefaultBudget(tc.n); got != tc.want {
			t.Errorf("DefaultBudget(%d) = %d, want %d", tc.n, got, tc.want)
		}
	}
}
