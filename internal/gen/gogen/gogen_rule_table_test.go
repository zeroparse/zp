package gogen_test

import (
	"math"
	"testing"

	"zeroparse.dev/zp/internal/gen/gogen"
)

func TestRuleTableLimits(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		strs, seen int
		ok         bool
	}{
		{1, 0, true},
		{math.MaxUint16 + 1, math.MaxInt32, true},
		{math.MaxUint16 + 2, 0, false},
		{1, math.MaxInt32 + 1, false},
	} {
		if err := gogen.RuleTableLimits(c.strs, c.seen); (err == nil) != c.ok {
			t.Errorf("ruleTableLimits(%d, %d) = %v, want ok=%v", c.strs, c.seen, err, c.ok)
		}
	}
}
