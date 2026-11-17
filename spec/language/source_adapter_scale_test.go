package language_test

import (
	"errors"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/frontend/parser"
	"zeroparse.dev/zp/internal/frontend/parser/zpparser"
	"zeroparse.dev/zp/spec/language/internal/classretry"
)

// The recovery of a declaration skips to the next one over the brackets it opens. A bracket that is not
// closed makes it try the group and then go on without it, so a source of many open brackets must not
// make the skip grow faster than the source. A "[" that does not close on its line is a class that the
// skip ends at the end of the line, unless it follows a name, ")" or "]", so the skip must not try the
// class again from every later "[". These sources fail, and the compute cost of 8 times as many brackets
// is compared with the cost of the first size. The cost is the count of units that the generated parser
// charges, not a time, so the result does not depend on the load of the machine. The generated parser of
// the grammar source keeps no records of the walks that the engine reuses, so the grammar itself must
// keep the recovery linear. The sources are parsed by the generated parser alone, with the options of the
// source APIs; the lexer and the tree builder are outside this measure.
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
	{"open classes with escapes", func(n int) string { return "rule a = x" + strings.Repeat("[\\", n/2) + "\nrule b = c\n" }},
	{"open indexes", func(n int) string { return "rule a = " + strings.Repeat("a[", n/2) + "\nrule b = c\n" }},
	{"empty brackets and an open one", func(n int) string { return "rule a = x" + strings.Repeat("[]", n/2) + "[\nrule b = c\n" }},
	{"indexes of groups", func(n int) string { return "rule a = " + strings.Repeat("(a)[", n/4) + "\nrule b = c\n" }},
	{"classes in open groups", func(n int) string { return "rule a = " + strings.Repeat("( [", n/3) + "\nrule b = c\n" }},
	{"open index on each line", func(n int) string { return "rule a = " + strings.Repeat("x[\n", n/3) + "\nrule b = c\n" }},
	{"attribute lines in an open group", func(n int) string { return "rule a = (x\n" + strings.Repeat("@ q\n", n/4) }},
	{"attribute lines after a broken declaration", func(n int) string { return "rule a = x )\n" + strings.Repeat("@ q\n", n/4) }},
	{"keyword attribute lines in an open group", func(n int) string { return "rule a = (x\n" + strings.Repeat("@type\n", n/6) }},
	{"attribute arguments on lines in an open group", func(n int) string { return "rule a = (x\n" + strings.Repeat("@q(\n)\n", n/6) }},
	{"prefix operators on lines in a group", func(n int) string { return "rule a = (" + strings.Repeat("!\n", 4*n) + "x)" }},
	{"prefix operators on lines in braces", func(n int) string { return "rule a = {" + strings.Repeat("!\n", 4*n) + "x}" }},
	{"negations on lines in a value group", func(n int) string { return "rule a = x => (" + strings.Repeat("-\n", 4*n) + "1)" }},
}

// computeCost returns the compute units that a parse takes: the least Compute limit under which run
// does not fail with a limit error. run parses with the given limit. The parser charges its work (input
// units inspected, rule calls and loop iterations) to one counter and rejects the charge that passes
// the limit, so the cost is exact and does not depend on the time or the load of the machine. A parse
// that ends with another error, such as a syntax error, has completed.
func computeCost(run func(limit uint64) error) uint64 {
	completes := func(limit uint64) bool { return !errors.Is(run(limit), core.ErrLimit) }
	hi := uint64(1 << 10)
	for !completes(hi) {
		hi *= 2
	}
	lo := hi / 2 // lo fails, or is zero; hi completes
	for hi-lo > 1 {
		if mid := lo + (hi-lo)/2; completes(mid) {
			hi = mid
		} else {
			lo = mid
		}
	}
	return hi
}

// scaleFactor is how many times larger the big source is than the small one, and scaleLimit is the
// largest ratio of their costs that counts as linear.
const (
	scaleFactor = 8
	scaleLimit  = 24
)

// scalesLinearly compares the cost of the work at a size and at scaleFactor times that size, and reports
// the ratio of the two. The cost is linear if the ratio is about scaleFactor and quadratic if it is
// about its square, so a ratio over scaleLimit, the middle of the two on a logarithmic scale, is a growth
// that is not linear. The costs are counts of the parser, not times, so the ratio of a size is the same
// on every run and under any load, and a single measure decides.
func scalesLinearly(t *testing.T, small int, cost func(n int) uint64) (ratio float64, ok bool) {
	t.Helper()
	a, b := cost(small), cost(small*scaleFactor)
	ratio = float64(b) / float64(max(a, 1))
	t.Logf("%d: %d units, %d: %d units, ratio %.2f", small, a, small*scaleFactor, b, ratio)
	return ratio, ratio <= scaleLimit
}

// sourceCost is the compute cost of the generated parser of ZeroParse sources on src, with the options
// and the limits of the source APIs except for Compute.
func sourceCost(src string) uint64 {
	return computeCost(func(limit uint64) error {
		_, err := zpparser.ParseValue(src, zpparser.WithExpectations(), zpparser.WithLimits(zpparser.Limits{Compute: limit}))
		return err
	})
}

func TestRecoveryScalesLinearly(t *testing.T) {
	for _, c := range scaleSources {
		t.Run(c.name, func(t *testing.T) {
			ratio, ok := scalesLinearly(t, 2000, func(n int) uint64 { return sourceCost(c.make(n)) })
			if !ok {
				t.Errorf("the cost grows %.1f times for %d times as many brackets (limit %d)", ratio, scaleFactor, scaleLimit)
			}
		})
	}
}

// allocCost returns what building the result of parser.Parse on src takes in allocations: the number of
// objects and of bytes. It is the load-independent measure of the whole source API, with the lexer, the
// error messages and the builder of the grammar, which the compute counter of the generated parser does
// not cover. A parse is warmed up first, so that one-time tables are not counted. Then the parse runs with
// the collector off, after two collections that empty the pools, so that no freed buffer is allocated
// again and the counters of the runtime are exact: one run gives the same count every time, whatever the
// load. The test does not run in parallel, so that the counters hold the work of this goroutine. It does
// not see a growth that allocates nothing, such as a loop that rescans the tokens; the compute cost
// covers the generated parser, and the lexer and the builder have no counter of their work.
func allocCost(src string) (mallocs, bytes uint64) {
	parser.Parse(src)
	runtime.GC()
	runtime.GC()
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	parser.Parse(src)
	runtime.ReadMemStats(&after)
	return after.Mallocs - before.Mallocs, after.TotalAlloc - before.TotalAlloc
}

// scaleAllocSources are the sources of scaleSources and valid grammars, which reach the builder of the
// grammar, since the others end in errors.
var scaleAllocSources = append([]struct {
	name string
	make func(n int) string
}{
	{"valid rules", func(n int) string { return strings.Repeat("rule a = x (y / z)* \"s\"\n", n/8) }},
	{"valid rules with comments", func(n int) string { return strings.Repeat("// c\nrule a = x /* d */ y\n", n/8) }},
}, scaleSources...)

// TestSourceAPIAllocationsScaleLinearly checks that the lexer, the messages and the builder of the grammar
// scale linearly with the whole source API: the objects and the bytes that parser.Parse allocates for 8
// times as large a source grow at most scaleLimit times.
func TestSourceAPIAllocationsScaleLinearly(t *testing.T) {
	for _, c := range scaleAllocSources {
		t.Run(c.name, func(t *testing.T) {
			m1, b1 := allocCost(c.make(2000))
			m2, b2 := allocCost(c.make(2000 * scaleFactor))
			mr, br := float64(m2)/float64(max(m1, 1)), float64(b2)/float64(max(b1, 1))
			t.Logf("objects %d -> %d (%.2f), bytes %d -> %d (%.2f)", m1, m2, mr, b1, b2, br)
			if mr > scaleLimit || br > scaleLimit {
				t.Errorf("allocations grow %.1f times in objects and %.1f times in bytes for %d times as large a source (limit %d)", mr, br, scaleFactor, scaleLimit)
			}
		})
	}
}

func classRetryCost(t *testing.T, src string) uint64 {
	t.Helper()
	return computeCost(func(limit uint64) error {
		_, err := classretry.Parse(src, classretry.WithLimits(core.Limits{Compute: limit}))
		return err
	})
}

// synthetic costs of known growth, counted like the parser counts: one unit for each step.
func linearSteps(n int) uint64 { return uint64(5 * n) }
func quadraticSteps(n int) uint64 {
	var s uint64
	for i := 0; i < n; i++ {
		s += uint64(n)
	}
	return s
}

// TestScaleDetectorSeesQuadraticTime is the negative control of TestRecoveryScalesLinearly: a change of
// the grammar that makes the recovery quadratic did not turn up once, because the time of the sources
// stayed linear. So the control applies the same measure to costs of a known growth: a cost that grows
// with the square of the size must be reported as a failure, and a linear cost must not.
func TestScaleDetectorSeesQuadraticTime(t *testing.T) {
	t.Run("quadratic", func(t *testing.T) {
		if ratio, ok := scalesLinearly(t, 2000, func(n int) uint64 { return quadraticSteps(n) }); ok {
			t.Errorf("quadratic cost passed with ratio %.1f", ratio)
		}
	})
	t.Run("linear", func(t *testing.T) {
		if ratio, ok := scalesLinearly(t, 2000, func(n int) uint64 { return linearSteps(n) }); !ok {
			t.Errorf("linear cost failed with ratio %.1f", ratio)
		}
	})
}

// TestScaleDetectorSeesGeneratedClassRetries is the negative control of TestRecoveryScalesLinearly on a
// generated parser: the parser of a repetition that tries a class from every "[" and falls back to any
// character, as the recovery of declarations did before it ended an open class at the end of its line,
// takes a cost that grows with the square of the line, and the measure must report it. The same parser
// on a line without "[" is the linear control: the class fails at its first character.
func TestScaleDetectorSeesGeneratedClassRetries(t *testing.T) {
	t.Run("quadratic", func(t *testing.T) {
		ratio, ok := scalesLinearly(t, 250, func(n int) uint64 { return classRetryCost(t, strings.Repeat("[", n)+"\n") })
		if ok {
			t.Errorf("class retries of the generated parser passed with ratio %.1f", ratio)
		}
	})
	t.Run("linear", func(t *testing.T) {
		ratio, ok := scalesLinearly(t, 250, func(n int) uint64 { return classRetryCost(t, strings.Repeat("x", n)+"\n") })
		if !ok {
			t.Errorf("linear parse of the generated parser failed with ratio %.1f", ratio)
		}
	})
}

// TestScaleCostIgnoresLoad checks that the cost does not depend on the load: the cost of a parse is the
// same while every CPU is busy with other work as when it is idle, so no run can fail by noise.
func TestScaleCostIgnoresLoad(t *testing.T) {
	src := "rule a = x" + strings.Repeat("[", 2000) + "\nrule b = c\n"
	idle := sourceCost(src)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 2*runtime.NumCPU(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for s := 0; ; s++ {
				select {
				case <-stop:
					return
				default:
					scaleSink += s
				}
			}
		}()
	}
	loaded := sourceCost(src)
	close(stop)
	wg.Wait()
	if idle != loaded {
		t.Errorf("cost %d when idle, %d under load", idle, loaded)
	}
}

// scaleSink keeps the work of the load above from being removed.
var scaleSink int

// BenchmarkCanonicalFrontend parses grammars with the generated parser and builds the grammar from its
// tree, which is the work of the generated frontend of ParseGrammar. The sources are valid, so the
// recovery does not run; the measure shows the cost of the lookahead that ends a list at a declaration.
func BenchmarkCanonicalFrontend(b *testing.B) {
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
				if _, err := adaptCanonical(src); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
