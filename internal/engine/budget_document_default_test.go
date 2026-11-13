//go:build !zp_budget_controls_reference

package engine

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"zeroparse.dev/zp/internal/engine/core"
	"zeroparse.dev/zp/internal/gentest"
	"zeroparse.dev/zp/runtime/parse"
)

// docLines is a grammar of the class Linear whose rules are all transient, so that a fresh parse does
// not memoize them and a Document does.
const docLines = `rule main = line* $$
rule line = ^ word " " word "\n"
rule word = [a-z]+ #atomic`

// usedBy returns the units a parse reserves, counting every unit, for the parse that run starts under
// a limit it cannot reach.
func usedBy(t *testing.T, o ParseOptions, run func(ParseOptions) error) uint64 {
	t.Helper()
	var used uint64
	o.Limits = parse.Limits{Compute: 1 << 50}
	o.Trace = func(e TraceEvent) { used = core.TraceParser(e).Control.Used }
	if err := run(o); err != nil {
		t.Fatal(err)
	}
	return used
}

// editScenario edits the text of a Document before its second parse.
type editScenario struct {
	name  string
	lines int
	edit  func(t *testing.T, d *Document)
}

func insertEvery(count, stride int) func(*testing.T, *Document) {
	return func(t *testing.T, d *Document) {
		for i := 0; i < count; i++ {
			if err := d.Edit(i*stride+1, i*stride+1, "x"); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// A Document parse under the default limits succeeds whenever a parse of the same text from scratch
// does, whatever edits are pending, and returns what that parse returns.
func TestBudgetDocumentDefaultSucceedsWhereFreshDoes(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, docLines)
	scenarios := []editScenario{
		{"20000 lines and 6000 edits", 20000, insertEvery(6000, 18)},
		{"one edit", 20000, insertEvery(1, 18)},
		{"many small edits", 3000, insertEvery(2999, 6)},
		{"one huge edit", 20000, func(t *testing.T, d *Document) {
			if err := d.Edit(6, len(d.Text())/2, strings.Repeat("zz yy\n", 7000)); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, back := range []Backend{Closure, Bytecode, BytecodeIterative} {
		for _, unit := range []parse.PositionUnit{parse.Bytes, parse.CodePoints} {
			for _, sc := range scenarios {
				t.Run(fmt.Sprintf("%s/%s/%s", back, unit, sc.name), func(t *testing.T) {
					o := ParseOptions{Backend: back, Unit: unit}
					d, err := prog.NewDocumentWith("main", strings.Repeat("ab cd\n", sc.lines), o)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := d.Parse(); err != nil {
						t.Fatalf("first parse: %v", err)
					}
					sc.edit(t, d)
					got, err := d.Parse()
					if err != nil {
						t.Fatalf("document parse of %d units: %v", len(d.Text()), err)
					}
					want, werr := prog.ParseWith("main", d.Text(), o)
					if dump(t, got, err) != dump(t, want, werr) {
						t.Fatal("document result differs from the parse from scratch")
					}
				})
			}
		}
	}
}

// countedBy returns the units that a parse counts against its default limit: the units it reserves
// less those of work that only a Document does. The parse runs under the default limit.
func countedBy(t *testing.T, o ParseOptions, run func(ParseOptions) error) uint64 {
	t.Helper()
	var counted uint64
	o.Limits = parse.Limits{}
	o.Trace = func(e TraceEvent) {
		c := core.TraceParser(e).Control
		counted = c.Used - c.Doc
	}
	if err := run(o); err != nil {
		t.Fatal(err)
	}
	return counted
}

// Of the work a Document parse does, the part that counts against the default limit is no more than
// what a parse of the same text from scratch counts, for the first parse and after edits, on every
// backend and unit.
func TestBudgetDocumentCountedWorkWithinFresh(t *testing.T) {
	t.Parallel()
	grammars := []struct {
		name, src string
		text      func(n int) string
		edits     []string
	}{
		{"lines", docLines, func(n int) string { return strings.Repeat("ab cd\n", n) }, []string{"x", "\n", "ab cd\n", ""}},
		{"incremental", gentest.IncrementalGrammar, func(n int) string {
			return strings.Repeat("x = 1+2*(3-4)\nab,cd,ef\n@pos\n\n", n)
		}, []string{"a", "1", "\n", " = ", "(", ")", ""}},
		{"recover", `rule main = ("a" @error("need a") #recover(skip=.))* $$`, func(n int) string { return strings.Repeat("aab", n) }, []string{"a", "b", ""}},
	}
	for _, gc := range grammars {
		prog := mustCompile(t, gc.src)
		for _, back := range []Backend{Closure, Bytecode, BytecodeIterative} {
			for _, unit := range []parse.PositionUnit{parse.Bytes, parse.CodePoints} {
				for _, n := range []int{3, 200, 1500} {
					t.Run(fmt.Sprintf("%s/%s/%s/%d", gc.name, back, unit, n), func(t *testing.T) {
						o := ParseOptions{Backend: back, Unit: unit}
						d, err := prog.NewDocumentWith("main", gc.text(n), o)
						if err != nil {
							t.Fatal(err)
						}
						rng := rand.New(rand.NewSource(int64(n)))
						for round := 0; round < 12; round++ {
							if round > 0 {
								for k := rng.Intn(1 + round*round); k >= 0; k-- {
									size := len([]rune(d.Text()))
									at := rng.Intn(size + 1)
									end := at + rng.Intn(min(3, size-at+1))
									if err := d.Edit(at, end, gc.edits[rng.Intn(len(gc.edits))]); err != nil {
										t.Fatal(err)
									}
								}
							}
							text := d.Text()
							var perr error
							doc := countedBy(t, o, func(o ParseOptions) error { _, perr = d.ParseWith(o); return nil })
							fresh := countedBy(t, o, func(o ParseOptions) error { _, err := prog.ParseWith("main", text, o); _ = err; return nil })
							if errors.Is(perr, parse.ErrLimit) {
								t.Fatalf("round %d: document parse: %v", round, perr)
							}
							if doc > fresh {
								t.Fatalf("round %d: the document counts %d units against the default limit, the parse from scratch %d", round, doc, fresh)
							}
						}
					})
				}
			}
		}
	}
}

// scanQuad scans the rest of the input from every position, so its cost grows with the square of the
// input length and meets the default limit, which is proportional to it, at a length of a few thousand
// units. The bound of the repetition in rest keeps it from being a run site, whose records would make
// it linear. Its rule rest calls no rule, so a Document memoizes it though a parse from scratch does not.
const scanQuad = `rule main = (. &rest)* $$
rule rest = . #repeat(0, 1000000)`

// sameLengthEdits replaces each of the first count characters of a text of "a" with "b": edits that
// leave the length of the text and the cost of a parse from scratch unchanged.
func sameLengthEdits(t *testing.T, d *Document, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		if err := d.Edit(i, i+1, "b"); err != nil {
			t.Fatal(err)
		}
	}
}

// largestFreshLength returns the largest length of a text of "a" that a parse from scratch parses
// under the default limit.
func largestFreshLength(t *testing.T, prog *Program, o ParseOptions) int {
	t.Helper()
	fits := func(n int) bool {
		_, err := prog.ParseWith("main", strings.Repeat("a", n), o)
		if err != nil && !errors.Is(err, parse.ErrLimit) {
			t.Fatal(err)
		}
		return err == nil
	}
	lo, hi := 1, 1<<14
	if !fits(lo) || fits(hi) {
		t.Fatal("the search range does not hold the boundary")
	}
	for hi-lo > 1 {
		if mid := (lo + hi) / 2; fits(mid) {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo
}

// A Document parse succeeds under the default limit at the largest input that a parse from scratch
// parses under it, with no edit, one edit, and more pending edits than a memo entry may lag behind.
func TestBudgetDocumentDefaultAtBoundary(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, scanQuad)
	for _, back := range []Backend{Closure, Bytecode, BytecodeIterative} {
		for _, unit := range []parse.PositionUnit{parse.Bytes, parse.CodePoints} {
			o := ParseOptions{Backend: back, Unit: unit}
			t.Run(fmt.Sprintf("%s/%s", back, unit), func(t *testing.T) {
				n := largestFreshLength(t, prog, o)
				for _, edits := range []int{0, 1, 50, 129, 1000} {
					d, err := prog.NewDocumentWith("main", strings.Repeat("a", n), o)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := d.Parse(); err != nil {
						t.Fatalf("%d units, first parse: %v", n, err)
					}
					sameLengthEdits(t, d, edits)
					got, err := d.Parse()
					if err != nil {
						t.Fatalf("%d units, %d edits: %v", n, edits, err)
					}
					want, werr := prog.ParseWith("main", d.Text(), o)
					if dump(t, got, err) != dump(t, want, werr) {
						t.Fatalf("%d edits: document result differs from the parse from scratch", edits)
					}
				}
			})
		}
	}
}

// minimalLimit returns the smallest explicit Compute limit under which run succeeds.
func minimalLimit(t *testing.T, run func(limit uint64) error) uint64 {
	t.Helper()
	lo, hi := uint64(1), uint64(1)<<30
	if run(lo) == nil || run(hi) != nil {
		t.Fatal("the search range does not hold the minimal limit")
	}
	for hi-lo > 1 {
		if mid := (lo + hi) / 2; run(mid) == nil {
			hi = mid
		} else {
			lo = mid
		}
	}
	return hi
}

// An explicit limit counts all the work of a Document parse, the part that only a Document does too:
// the first parse of a Document needs a larger limit than a parse of its text from scratch, and a
// limit one unit short of what it needs stops it with that limit in the error.
func TestBudgetDocumentExplicitLimitCountsDocumentWork(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, docLines)
	text := strings.Repeat("ab cd\n", 2000)
	for _, back := range []Backend{Closure, Bytecode, BytecodeIterative} {
		t.Run(back.String(), func(t *testing.T) {
			o := ParseOptions{Backend: back, Unit: parse.Bytes}
			parseFn := func(limit uint64) error {
				oo := o
				oo.Limits = parse.Limits{Compute: limit}
				_, err := prog.ParseWith("main", text, oo)
				return err
			}
			docParse := func(limit uint64) error {
				d, err := prog.NewDocumentWith("main", text, o)
				if err != nil {
					t.Fatal(err)
				}
				oo := o
				oo.Limits = parse.Limits{Compute: limit}
				_, err = d.ParseWith(oo)
				return err
			}
			fresh, doc := minimalLimit(t, parseFn), minimalLimit(t, docParse)
			if doc <= fresh {
				t.Fatalf("a document needs a limit of %d units for its first parse, a parse from scratch %d: an explicit limit leaves Document work out", doc, fresh)
			}
			if got := limitOf(t, docParse(doc-1)); got != doc-1 {
				t.Fatalf("limit %d reported as %d", doc-1, got)
			}
		})
	}
}

// runawayTimeout bounds the time a parse of a runaway grammar may take: the default limit stops it
// within a few seconds.
const runawayTimeout = 20 * time.Second

// A grammar whose cost is far above the default limit stays stopped on a Document, also when the
// Document has more pending edits than its log holds and a memo that lags behind all of them. With
// fewer edits the text is short and the parse is cheap.
func TestBudgetDocumentRunawayStoppedByDefaultWithPendingEdits(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, scanGrammar)
	for _, back := range []Backend{Closure, Bytecode, BytecodeIterative} {
		for _, pending := range []int{0, 1, 70000} {
			t.Run(fmt.Sprintf("%s/%d", back, pending), func(t *testing.T) {
				o := ParseOptions{Backend: back}
				d, err := prog.NewDocumentWith("main", strings.Repeat("a", 30), o)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := d.Parse(); err != nil {
					t.Fatalf("first parse: %v", err)
				}
				for i := 0; i < pending; i++ {
					n := len(d.Text())
					if err := d.Edit(n, n, "a"); err != nil {
						t.Fatal(err)
					}
				}
				// A parse that the default does not stop would run for hours: bound the time it may take,
				// so that a regression fails at the bound.
				done := make(chan error, 1)
				go func() { _, err := d.Parse(); done <- err }()
				select {
				case err = <-done:
				case <-time.After(runawayTimeout):
					t.Fatalf("the parse of a runaway grammar was not stopped within %v", runawayTimeout)
				}
				if pending == 0 {
					if err != nil {
						t.Fatal(err)
					}
					return
				}
				if pending == 1 {
					if err != nil {
						t.Fatal(err)
					}
					return
				}
				limit := uint64(max(1<<20, 1024*len(d.Text())))
				if got := limitOf(t, err); got != limit {
					t.Fatalf("default limit %d, want %d", got, limit)
				}
				var le *parse.LimitError
				if errors.As(err, &le) && le.Used > le.Limit {
					t.Fatalf("stopped at %d units used under a limit of %d", le.Used, le.Limit)
				}
			})
		}
	}
}

// documentWorkFactor is the factor by which the units of a Document parse after pending edits, all
// counted as an explicit limit counts them, may exceed those of a parse of the same text from
// scratch, for a grammar whose Document retains one memo entry per call. It does not depend on the
// number of pending edits: a lookup applies at most ops.PendingEditLimit edits to the entry it finds.
const documentWorkFactor = 6

// letterEdits replaces count letters of the text of a Document, spread over it, by another letter:
// edits that leave the text a valid input and its length unchanged. count is at most the capacity of
// the edit log.
func letterEdits(t *testing.T, d *Document, count int) {
	t.Helper()
	text := []byte(d.Text())
	var at []int
	for i, c := range text {
		if c >= 'a' && c <= 'z' {
			at = append(at, i)
		}
	}
	for i := 0; i < count; i++ {
		p := at[(i*7919)%len(at)]
		if err := d.Edit(p, p+1, string(rune('a'+(i%25)+1))); err != nil {
			t.Fatal(err)
		}
	}
}

// With any number of pending edits, up to the capacity of the edit log, the units of a Document
// parse stay within documentWorkFactor times those of a parse from scratch, all units counted
// (an explicit limit). Without the limit on the edits a lookup applies the bound grows with the
// number of edits.
func TestBudgetDocumentWorkWithinFactorOfFresh(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, docLines)
	for _, back := range []Backend{Closure, Bytecode, BytecodeIterative} {
		for _, edits := range []int{0, 128, 129, 6000, 65536} {
			t.Run(fmt.Sprintf("%s/%d", back, edits), func(t *testing.T) {
				o := ParseOptions{Backend: back, Unit: parse.Bytes}
				d, err := prog.NewDocumentWith("main", strings.Repeat("ab cd\n", 20000), o)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := d.Parse(); err != nil {
					t.Fatalf("first parse: %v", err)
				}
				letterEdits(t, d, edits)
				fresh := usedBy(t, o, func(o ParseOptions) error { _, err := prog.ParseWith("main", d.Text(), o); return err })
				// The document parse runs under the limit the bound allows, so that a regression stops
				// at it instead of running to the end.
				limit := documentWorkFactor * fresh
				oo := o
				oo.Limits = parse.Limits{Compute: limit}
				var used uint64
				oo.Trace = func(e TraceEvent) { used = core.TraceParser(e).Control.Used }
				if _, err := d.ParseWith(oo); err != nil {
					t.Fatalf("%d pending edits: a document parse needs more than %d units, %d times the %d of a parse from scratch: %v",
						edits, limit, documentWorkFactor, fresh, err)
				}
				t.Logf("%d pending edits: %d units against %d from scratch (%.2f)", edits, used, fresh, float64(used)/float64(fresh))
			})
		}
	}
}

// The missing-token mode is not available to a Document parse, so no rule is excluded from the
// memoization of a Document by it (docOnlyMemo tests the mode only for the sake of completeness, and
// the unit test of docOnlyMemo covers it).
func TestDocumentRejectsMissingTokens(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, "rule main = item* $$\nrule item = word sep\nrule word = \"a\"+ #atomic\nrule sep = \";\" #missing")
	o := ParseOptions{Backend: Closure, MissingTokens: 10}
	d, err := prog.NewDocumentWith("main", "aa;a;", o)
	if err == nil {
		_, err = d.ParseWith(o)
	}
	if err == nil || !strings.Contains(err.Error(), "not supported for Document parsing") {
		t.Fatalf("a Document parse with missing tokens: got %v", err)
	}
}
