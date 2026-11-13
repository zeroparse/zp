//go:build !zp_budget_controls_reference

package engine

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"zeroparse.dev/zp/runtime/parse"
)

// recoverGrammar recovers an error at each item and shares the list of the errors between the results
// of its rule s, which a parse from scratch memoizes too (see the recovered errors in
// spec/proofs/linear-cost.md).
const recoverGrammar = "rule main = s $$\nrule s = item s \"!\" / item s / item\nrule item = (\"a\" \";\") #recover(skip=[^;]* \";\")"

// A text of recovered errors that a parse from scratch completes under the default limit is completed by
// a Document parse as well, with a constant factor of its units: the lists of errors are shared, so
// replaying them in the memo costs a constant for each entry, counted or not.
func TestBudgetDocumentRecoveredErrorsWithinFresh(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, recoverGrammar)
	for _, back := range []Backend{Closure, Bytecode, BytecodeIterative} {
		t.Run(fmt.Sprint(back), func(t *testing.T) {
			t.Parallel()
			o := ParseOptions{Backend: back}
			for _, n := range []int{1 << 12, 1 << 14} {
				fresh, doc, ferr, derr := freshAndDocumentUnits(t, prog, strings.Repeat("b;", n), o)
				if errors.Is(ferr, parse.ErrLimit) || errors.Is(derr, parse.ErrLimit) {
					t.Fatalf("n=%d: parse from scratch: %.80v; Document parse: %.80v; want neither to exceed the limit", n, ferr, derr)
				}
				if doc > 3*fresh {
					t.Errorf("n=%d: the Document reserves %d units, more than 3 times the %d of a parse from scratch", n, doc, fresh)
				}
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
