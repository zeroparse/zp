package corepass_test

import (
	"fmt"
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/frontend/elaborate"
	"zeroparse.dev/zp/internal/middleend/corepass"
	"zeroparse.dev/zp/internal/testing/compiletest"
)

// TestNullableRepetitionFacts requires a repetition of an empty match to be nullable when its
// minimum is 0 or 1, and a left call after it to make a left-recursion cycle exactly then.
func TestNullableRepetitionFacts(t *testing.T) {
	t.Parallel()
	facts := func(src string) *corepass.Facts {
		t.Helper()
		g, err := compiletest.Parse(src)
		if err != nil {
			t.Fatal(err)
		}
		m, err := elaborate.Structural(compile.StructuralInput(g, g.Rules(), nil), nil)
		if err != nil {
			t.Fatal(err)
		}
		return corepass.Compute(m, nil)
	}
	for _, minimum := range []int{0, 1, 2, 3} {
		if got := facts(fmt.Sprintf("rule main = _ #repeat(%d, )", minimum)).Rules[0].Nullable; got != (minimum <= 1) {
			t.Errorf("minimum %d: nullable=%v", minimum, got)
		}
		if got := facts(fmt.Sprintf("rule main = _ #repeat(%d, ) r / \"a\"\nrule r = main", minimum)).Rules[0].Cyclic(); got != (minimum <= 1) {
			t.Errorf("minimum %d: left-call cycle=%v", minimum, got)
		}
	}
}
