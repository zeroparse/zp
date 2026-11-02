package complexitytest

import (
	"errors"
	"strings"
	"testing"

	"zeroparse.dev/zp"
)

// streamSteps returns the smallest budget with which a stream parse of input finishes, found by
// bisection with a relative tolerance of 0.4%.
func streamSteps(t *testing.T, p *zp.Parser, input string, b zp.Backend) uint64 {
	t.Helper()
	finishes := func(budget uint64) bool {
		err := p.ParseStream(strings.NewReader(input), func(*zp.Node) error { return nil },
			zp.WithBackend(b), zp.WithLimits(zp.Limits{Compute: budget}))
		var le *zp.LimitError
		if errors.As(err, &le) && le.Kind == zp.LimitCompute {
			return false
		}
		var one *zp.SyntaxError
		var many zp.SyntaxErrors
		if err != nil && !errors.As(err, &one) && !errors.As(err, &many) {
			t.Fatalf("%s: %v", BackendName(b), err)
		}
		return true
	}
	lo, hi := uint64(0), uint64(Cap)
	if !finishes(hi) {
		t.Fatalf("%s: stream parse exceeds the cap", BackendName(b))
	}
	for hi-lo > 1 && hi-lo > hi/256 {
		mid := lo + (hi-lo)/2
		if finishes(mid) {
			hi = mid
		} else {
			lo = mid
		}
	}
	return hi
}

// TestStreamRecoverLinear checks that a stream parse of a grammar with #recover is linear when the
// skip of each recovery is constant, as the batch parse of the same grammar is. Each recovery
// finds the line and column of its error in the buffer of the stream, which costs the units
// that no earlier error scanned, not the size of the buffer.
func TestStreamRecoverLinear(t *testing.T) {
	t.Parallel()
	const src = "rule main = (x / \"a\")* #stream $$\nrule x = (y #recover(skip=\"a\")) \"c\"\nrule y = \"b\"\n"
	p, err := Compile(src)
	if err != nil {
		t.Fatal(err)
	}
	skipWithoutBudget(t)
	sizes := double(1000)
	for _, b := range Backends {
		var s Series
		s.Sizes = sizes
		for _, n := range sizes {
			s.Costs = append(s.Costs, Cost{Steps: streamSteps(t, p, rep("a", n), b)})
		}
		if got := Classify(s); got != Linear {
			t.Errorf("%s: %v (slopes %.2f) classified %v, want %v", BackendName(b), s.Costs, s.Slopes(), got, Linear)
		}
		t.Logf("%s: steps %v slopes %.2f", BackendName(b), s.Costs, s.Slopes())
	}
}
