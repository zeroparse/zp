package complexitytest

import (
	"math"
	"runtime"
	"testing"

	"zeroparse.dev/zp"
	"zeroparse.dev/zp/internal/engine/core"
)

// skipRunControls returns the E8 controls whose walks a parse without skip-run records repeats.
func skipRunControls(t *testing.T) []Control {
	t.Helper()
	var cs []Control
	for _, c := range Controls() {
		if c.Family == "E8" {
			cs = append(cs, c)
		}
	}
	if len(cs) != 4 {
		t.Fatalf("%d E8 controls, want 4", len(cs))
	}
	return cs
}

// TestSkipRunsNeverRecordControl checks that the E8 controls measure linear only through the
// recording of walks: with walks that never record, every one is quadratic on every backend. It
// sets package state, so it does not run in parallel.
func TestSkipRunsNeverRecordControl(t *testing.T) {
	core.SkipRecordMutation = core.SkipNeverRecord
	defer func() { core.SkipRecordMutation = 0 }()
	controls := skipRunControls(t)
	progs := make([]*zp.Parser, len(controls))
	for i, c := range controls {
		p, err := Compile(c.Source)
		if err != nil {
			t.Fatal(err)
		}
		progs[i] = p
	}
	skipWithoutBudget(t)
	for i, c := range controls {
		p := progs[i]
		for _, b := range Backends {
			s, err := Measure(p, c.Input, c.ShortSizes, b, Cap)
			if err != nil {
				t.Fatalf("%s, %s: %v", c.Name, BackendName(b), err)
			}
			if got := Classify(s); got != Poly(2) {
				t.Errorf("%s, %s: %v (slopes %.2f) classified %v, want %v", c.Name, BackendName(b), s.Costs, s.Slopes(), got, Poly(2))
			}
		}
	}
}

// TestSkipRunsLinearMemory checks that the memory of a parse of the E8 controls grows linearly:
// in the nested family every walk starts before the slots of the site, which grow at the front.
func TestSkipRunsLinearMemory(t *testing.T) {
	if testing.Short() {
		t.Skip("measures large inputs")
	}
	for _, c := range skipRunControls(t) {
		p, err := Compile(c.Source)
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range Backends {
			var bytes []float64
			for _, n := range []int{4000, 8000, 16000} {
				in := c.Input(n)
				var before, after runtime.MemStats
				runtime.GC()
				runtime.ReadMemStats(&before)
				if _, err := p.Parse(in, zp.WithBackend(b), zp.WithLimits(zp.Limits{Compute: zp.Unlimited})); err != nil {
					t.Fatalf("%s, %s: %v", c.Name, BackendName(b), err)
				}
				runtime.ReadMemStats(&after)
				bytes = append(bytes, float64(after.TotalAlloc-before.TotalAlloc))
			}
			for i := 1; i < len(bytes); i++ {
				if slope := math.Log2(bytes[i] / bytes[i-1]); slope > 1.3 {
					t.Errorf("%s, %s: allocated %v bytes, slope %.2f", c.Name, BackendName(b), bytes, slope)
				}
			}
			t.Logf("%s, %s: allocated %v bytes", c.Name, BackendName(b), bytes)
		}
	}
}

// alternatingWalks are grammars whose repetition w is walked, at each position, from two or three
// positions whose walks do not overlap the walk before them (the binding of m decides where each
// walk ends). A site that compared a walk only with the one before it would stay cold; the site
// must turn hot. The cost is still quadratic: the binding of m is a new environment at each
// position, and records are keyed by the identity of the environment, so a walk never finds the
// walk of the position before. The test records both.
var alternatingWalks = []struct {
	Name, Source string
	Input        func(n int) string
}{
	{
		"two walks",
		"var m int\nrule main = (a / \"x\")* [xyz]* $$\nrule a = { m := 0 } w \"!\" / r { m := 1 } w \"!\"\nrule r = \"x\" r / \"\"\nrule w = (\"x\" / \"y\" { m == 1 })*\n",
		func(n int) string { return rep("x", n) + rep("y", n) },
	},
	{
		"three walks",
		"var m int\nrule main = (a / \"x\")* [xyz]* $$\nrule a = { m := 0 } w \"!\" / r { m := 1 } w \"!\" / r s { m := 2 } w \"!\"\nrule r = \"x\" r / \"\"\nrule s = \"y\" s / \"\"\nrule w = (\"x\" / \"y\" { m >= 1 } / \"z\" { m == 2 })*\n",
		func(n int) string { return rep("x", n) + rep("y", n) + rep("z", n) },
	},
}

// TestAlternatingWalksTurnHot checks that the site of alternating walks turns hot on every backend,
// and that the cost stays quadratic through the environment keys (a negative control: it moves to
// the linear controls when equal environments share one identity).
func TestAlternatingWalksTurnHot(t *testing.T) {
	skipWithoutBudget(t)
	for _, c := range alternatingWalks {
		p, err := Compile(c.Source)
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range Backends {
			turns := core.RunHotTurns
			s, err := Measure(p, c.Input, double(100), b, Cap)
			if err != nil {
				t.Fatalf("%s, %s: %v", c.Name, BackendName(b), err)
			}
			if core.RunHotTurns == turns {
				t.Errorf("%s, %s: no run site turned hot", c.Name, BackendName(b))
			}
			if got := Classify(s); got != Poly(2) {
				t.Errorf("%s, %s: %v (slopes %.2f) classified %v, want %v", c.Name, BackendName(b), s.Costs, s.Slopes(), got, Poly(2))
			}
		}
	}
}
