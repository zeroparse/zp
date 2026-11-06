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

// parseTime returns the least time over a few runs of the generated parser on src.
func parseTime(t *testing.T, src string) time.Duration {
	t.Helper()
	p, _ := sourceGrammar(t)
	best := time.Duration(1 << 62)
	for i := 0; i < 5; i++ {
		start := time.Now()
		p.Parse(src)
		best = min(best, time.Since(start))
	}
	return best
}

func TestRecoveryScalesLinearly(t *testing.T) {
	const small, factor, attempts = 2000, 8, 3
	for _, c := range scaleSources {
		t.Run(c.name, func(t *testing.T) {
			// The best of several runs of each size, and the best of a few attempts, so that other work on the
			// machine does not fail the test: a quadratic time fails every attempt.
			var ratio float64
			for attempt := 0; attempt < attempts; attempt++ {
				a, b := parseTime(t, c.make(small)), parseTime(t, c.make(small*factor))
				ratio = float64(b) / float64(max(a, time.Microsecond))
				t.Logf("%d: %v, %d: %v, ratio %.1f", small, a, small*factor, b, ratio)
				// Linear is about 8 and quadratic about 64.
				if ratio <= 24 || b <= 20*time.Millisecond {
					return
				}
			}
			t.Errorf("the time grows %.1f times for %d times as many brackets in each of %d attempts (limit 24)", ratio, factor, attempts)
		})
	}
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
