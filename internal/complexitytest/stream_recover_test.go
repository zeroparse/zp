package complexitytest

import (
	"errors"
	"strings"
	"testing"

	"zeroparse.dev/zp"
	"zeroparse.dev/zp/runtime/parse"
)

// streamSteps returns the smallest budget with which a stream parse of input finishes, found by
// bisection with a relative tolerance of 0.4%.
func streamSteps(t *testing.T, p *zp.Parser, input string, b zp.Backend) uint64 {
	t.Helper()
	finishes := func(budget uint64) bool {
		err := p.ParseStream(strings.NewReader(input), func(*parse.Node) error { return nil },
			zp.WithBackend(b), zp.WithLimits(parse.Limits{Compute: budget}))
		var le *parse.LimitError
		if errors.As(err, &le) && le.Kind == parse.LimitCompute {
			return false
		}
		var one *parse.SyntaxError
		var many parse.SyntaxErrors
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
// batch parse of the same grammar is: with a skip of constant size, and with the reused skips of
// the E8 controls, whose walks share their runs. Each recovery finds the line and column of its
// error in the buffer of the stream, which costs the units that no earlier error scanned, and
// builds an error value of its range and message, whatever the size of the skip.
func TestStreamRecoverLinear(t *testing.T) {
	t.Parallel()
	skipWithoutBudget(t)
	for _, c := range []struct {
		name, src string
		input     func(n int) string
		sizes     []int
	}{
		{"constant skip",
			"rule main = (x / \"a\")* #stream $$\nrule x = (y #recover(skip=\"a\")) \"c\"\nrule y = \"b\"\n",
			func(n int) string { return rep("a", n) }, double(1000)},
		{"E8 recovery skips",
			"rule main = (x / \"a\")* #stream $$\nrule x = (y #recover(skip=[a]+)) \"c\"\nrule y = \"b\"\n",
			func(n int) string { return rep("a", n) }, double(1000)},
		{"E8 recovery skips of two characters",
			"rule main = (x / \"a\")* #stream $$\nrule x = (y #recover(skip=(\"aa\" / \"a\")+)) \"c\"\nrule y = \"b\"\n",
			func(n int) string { return rep("a", n) }, double(1000)},
		{"E8 recovery skips in alternating regions",
			"rule main = (p / \"a\" / \"b\")* #stream $$\nrule p = x \"b\" x \"z\"\nrule x = \"c\" #recover(skip=[a]+)\n",
			func(n int) string { return rep("a", n) + "b" + rep("a", n) }, double(250)},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			p, err := Compile(c.src)
			if err != nil {
				t.Fatal(err)
			}
			for _, b := range Backends {
				var s Series
				s.Sizes = c.sizes
				for _, n := range c.sizes {
					s.Costs = append(s.Costs, Cost{Steps: streamSteps(t, p, c.input(n), b)})
				}
				if got := Classify(s); got != Linear {
					t.Errorf("%s: %v (slopes %.2f) classified %v, want %v", BackendName(b), s.Costs, s.Slopes(), got, Linear)
				}
				t.Logf("%s: steps %v slopes %.2f", BackendName(b), s.Costs, s.Slopes())
			}
		})
	}
}
