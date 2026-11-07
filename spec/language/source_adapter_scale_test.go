package language_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"zeroparse.dev/zp"
)

// The recovery of a declaration skips to the next one over the brackets it opens. A bracket that is not
// closed makes it try the group and then go on without it, so a source of many open brackets must not
// make the skip grow faster than the source. These sources fail, and the parse time of 8 times as many
// brackets is compared with the time of the first size.
var scaleSources = []struct {
	name string
	make func(n int) string
}{
	{"open parentheses", func(n int) string { return "rule a = " + strings.Repeat("(", n) + "\n" }},
	{"open braces", func(n int) string { return "rule a = x " + strings.Repeat("{", n) + "\nrule b = c\n" }},
	{"open brackets", func(n int) string { return "rule a = x" + strings.Repeat("[", n) + "\nrule b = c\n" }},
	{"mixed open brackets", func(n int) string { return "rule a = " + strings.Repeat("([{", n/3) + "\nrule b = c\n" }},
	{"balanced and an error", func(n int) string { return "rule a = " + strings.Repeat("(", n) + strings.Repeat(")", n) + " )\n" }},
	{"balanced brackets and an error", func(n int) string {
		return "rule a = " + strings.Repeat("({[ ", n/4) + strings.Repeat("]}) ", n/4) + " )\n"
	}},
	{"a declaration inside the groups", func(n int) string {
		return "rule a = " + strings.Repeat("(", n) + "\nrule z = 1\n" + strings.Repeat(")", n) + "\n"
	}},
	{"open group on each line", func(n int) string { return strings.Repeat("rule a = (\n var x\n", n/4) }},
	{"classes", func(n int) string { return "rule a = " + strings.Repeat("[a-z]", n/5) + " )\n" }},
	{"broken declarations", func(n int) string { return strings.Repeat("rule a = )\n", n/10) }},
	{"strings", func(n int) string { return "rule a = " + strings.Repeat("\"(\" ", n/4) + " )\n" }},
}

// bestTime returns the least time over a few runs of run.
func bestTime(run func()) time.Duration {
	best := time.Duration(1 << 62)
	for i := 0; i < 5; i++ {
		start := time.Now()
		run()
		best = min(best, time.Since(start))
	}
	return best
}

// scaleFactor is how many times larger the big source is than the small one.
const (
	scaleFactor   = 8
	scaleFloor    = 20 * time.Millisecond
	scaleMaxSmall = 64000
)

// scalesLinearly measures the work at a size and at scaleFactor times that size, and reports the ratio of
// the two times. The work is linear if the ratio is about scaleFactor and quadratic if it is about its
// square, so a ratio over 24 is a growth that is not linear. A time of a few milliseconds is mostly
// noise, so the sizes double until the time of the big size is over scaleFloor, as far as scaleMaxSmall;
// a family that stays under the floor there is reported as such and passes. Other work on the machine
// slows a run, so the best of a few attempts counts: a quadratic time fails every attempt.
func scalesLinearly(t *testing.T, small int, measure func(n int) time.Duration) (ratio float64, ok bool) {
	t.Helper()
	const attempts = 3
	for attempt := 0; attempt < attempts; attempt++ {
		a, b := measure(small), measure(small*scaleFactor)
		for b <= scaleFloor && small < scaleMaxSmall {
			small *= 2
			a, b = measure(small), measure(small*scaleFactor)
		}
		ratio = float64(b) / float64(max(a, time.Microsecond))
		t.Logf("%d: %v, %d: %v, ratio %.1f", small, a, small*scaleFactor, b, ratio)
		if b <= scaleFloor {
			t.Logf("the time of the big size is under %v at the largest size: nothing is measured", scaleFloor)
			return ratio, true
		}
		if ratio <= 24 {
			return ratio, true
		}
	}
	return ratio, false
}

func TestRecoveryScalesLinearly(t *testing.T) {
	p, _ := sourceGrammar(t)
	for _, c := range scaleSources {
		t.Run(c.name, func(t *testing.T) {
			ratio, ok := scalesLinearly(t, 2000, func(n int) time.Duration {
				src := c.make(n)
				return bestTime(func() { p.Parse(src) })
			})
			if !ok {
				t.Errorf("the time grows %.1f times for %d times as many brackets in each attempt (limit 24)", ratio, scaleFactor)
			}
		})
	}
}

// scaleSink keeps the work of the controls below from being removed.
var scaleSink int

func linearWork(n int) {
	s := 0
	for i := 0; i < 5000*n; i++ {
		s += i ^ (i >> 3)
	}
	scaleSink = s
}

func quadraticWork(n int) {
	s := 0
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			s += i ^ j
		}
	}
	scaleSink = s
}

// TestScaleDetectorSeesQuadraticTime is the negative control of TestRecoveryScalesLinearly. A change of the
// grammar that makes the recovery quadratic did not turn up: lookaheads to the end of the source that were
// added to the recovery rules left the time of these sources linear. So the control applies the same
// measure to work with a known growth: work that grows with the square of the size must be reported as a
// failure, and linear work must not.
func TestScaleDetectorSeesQuadraticTime(t *testing.T) {
	measure := func(work func(n int)) func(n int) time.Duration {
		return func(n int) time.Duration { return bestTime(func() { work(n) }) }
	}
	t.Run("quadratic", func(t *testing.T) {
		ratio, ok := scalesLinearly(t, 2000, measure(quadraticWork))
		if ok {
			t.Errorf("quadratic work passed with ratio %.1f", ratio)
		}
	})
	t.Run("linear", func(t *testing.T) {
		if ratio, ok := scalesLinearly(t, 2000, measure(linearWork)); !ok {
			t.Errorf("linear work failed with ratio %.1f", ratio)
		}
	})
}

// TestScaleAttemptsAbsorbNoise checks the attempts of scalesLinearly with times that are given, not
// measured: a linear run that another process slowed in the first two attempts passes in the third, and
// quadratic times fail in all three whatever the noise.
func TestScaleAttemptsAbsorbNoise(t *testing.T) {
	const unit = 100 * time.Microsecond
	run := func(sizeTime func(call, n int) time.Duration) (float64, bool) {
		call := 0
		return scalesLinearly(t, 2000, func(n int) time.Duration {
			call++
			return sizeTime(call, n)
		})
	}
	t.Run("noisy linear", func(t *testing.T) {
		// Calls 1 to 4 are the first two attempts, the big size of each slowed by 10 times.
		if ratio, ok := run(func(call, n int) time.Duration {
			d := time.Duration(n) * unit
			if call <= 4 && n > 2000 {
				d *= 10
			}
			return d
		}); !ok {
			t.Errorf("linear time with two noisy attempts failed with ratio %.1f", ratio)
		}
	})
	t.Run("quadratic", func(t *testing.T) {
		calls := 0
		ratio, ok := run(func(call, n int) time.Duration {
			calls = call
			return time.Duration(n) * time.Duration(n) * unit / 2000
		})
		if ok || calls != 6 {
			t.Errorf("quadratic time: ok %v after %d calls with ratio %.1f, want failure after 6 calls", ok, calls, ratio)
		}
	})
}

// BenchmarkCanonicalFrontend parses grammars with the generated parser and builds the grammar from its
// tree, which is the work of the generated frontend of ParseGrammar. The sources are valid, so the
// recovery does not run; the measure shows the cost of the lookahead that ends a list at a declaration.
func BenchmarkCanonicalFrontend(b *testing.B) {
	self, err := os.ReadFile("../../zp.zp")
	if err != nil {
		b.Fatal(err)
	}
	p, err := zp.CompileSource(string(self), "main")
	if err != nil {
		b.Fatal(err)
	}
	for _, c := range []struct{ name, path string }{
		{"zp.zp", "../../zp.zp"},
		{"golang", "../../testdata/grammars/golang/golang.zp"},
		{"python", "../../testdata/grammars/python/python.zp"},
	} {
		bs, err := os.ReadFile(c.path)
		if err != nil {
			b.Fatal(err)
		}
		src := string(bs)
		b.Run(c.name, func(b *testing.B) {
			b.SetBytes(int64(len(src)))
			for i := 0; i < b.N; i++ {
				if _, err := adaptCanonical(p, src); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
