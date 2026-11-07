//go:build !zp_budget_controls_reference

package engine

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/limits"
)

// recoverGrammar copies and replays the list of recovered errors in proportion to its length at each
// position (see the recovered errors in spec/proofs/linear-cost.md): its rule s is memoized by a parse
// from scratch too, so the copies count against the default limit of a Document parse as they do there.
const recoverGrammar = "rule main = s $$\nrule s = item s \"!\" / item s / item\nrule item = (\"a\" \";\") #recover(skip=[^;]* \";\")"

// A Document parse stays stopped by the default limit on a text of recovered errors that is several
// times as long as the longest one a parse from scratch admits: the copy and the replay of the errors in
// the memo count in a Document as they count in a parse from scratch.
func TestBudgetDocumentCountsRecoveredErrorCopies(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, recoverGrammar)
	for _, back := range []Backend{Closure, Bytecode, BytecodeIterative} {
		t.Run(fmt.Sprint(back), func(t *testing.T) {
			t.Parallel()
			o := ParseOptions{Backend: back}
			fits := func(n int) bool {
				_, err := prog.ParseWith("main", strings.Repeat("b;", n), o)
				if err != nil && !errors.Is(err, limits.ErrLimit) {
					return true // the syntax errors are the result of a completed parse
				}
				return err == nil
			}
			lo, hi := 1, 1<<12
			if !fits(lo) || fits(hi) {
				t.Fatalf("the boundary of a parse from scratch is outside %d..%d", lo, hi)
			}
			for hi-lo > 1 {
				if mid := (lo + hi) / 2; fits(mid) {
					lo = mid
				} else {
					hi = mid
				}
			}
			n := 4 * lo
			d, err := prog.NewDocumentWith("main", strings.Repeat("b;", n), o)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := d.ParseWith(o); !errors.Is(err, limits.ErrLimit) {
				t.Fatalf("Document parse of %d items, 4 times the %d that a parse from scratch admits: got %.80v, want the limit error", n, lo, err)
			}
		})
	}
}

// memoHitsAfter returns the memo hits of the parse of a Document of items after count edits of the last
// item, none of which lies in the input another item examined. Every other item's entry is stored by
// the first parse and lags behind count edits.
func memoHitsAfter(t *testing.T, prog *Program, o ParseOptions, count int) int {
	t.Helper()
	d, err := prog.NewDocumentWith("main", strings.Repeat("ab ", 40), o)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.ParseWith(o); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < count; i++ {
		if err := d.Edit(3*39, 3*39+1, string(rune('a'+i%2))); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := d.ParseWith(o); err != nil {
		t.Fatal(err)
	}
	return d.Stats().Reused
}

// A memo entry that lags behind 128 pending edits is reused; one that lags behind 129 is a miss.
func TestDocumentMemoLagBoundary(t *testing.T) {
	t.Parallel()
	item := strings.TrimSpace(strings.Repeat("item ", 40))
	prog := mustCompile(t, fmt.Sprintf("rule main = %s $$\nrule item = [a-z]+ \" \" #atomic", item))
	for _, back := range []Backend{Closure, Bytecode, BytecodeIterative} {
		t.Run(fmt.Sprint(back), func(t *testing.T) {
			t.Parallel()
			o := ParseOptions{Backend: back}
			hits := map[int]int{}
			for _, k := range []int{127, 128, 129, 130} {
				hits[k] = memoHitsAfter(t, prog, o, k)
			}
			t.Logf("memo hits by pending edits: %v", hits)
			if hits[127] < 30 || hits[128] < 30 {
				t.Errorf("entries that lag behind 127 and 128 edits are not reused: %v", hits)
			}
			if hits[129] > 5 || hits[130] > 5 {
				t.Errorf("entries that lag behind 129 and 130 edits are reused: %v", hits)
			}
		})
	}
}
