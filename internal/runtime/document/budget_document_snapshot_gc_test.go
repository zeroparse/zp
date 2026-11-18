//go:build !zp_budget_controls_reference

package document_test

import (
	"runtime"
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	. "zeroparse.dev/zp/internal/engine"
)

// minSecondSnapshotCompute returns the least Compute limit under which the snapshot after an edit
// succeeds, with the first snapshot kept alive or released to the collector.
func minSecondSnapshotCompute(t *testing.T, prog *Program, text string, keep bool) uint64 {
	t.Helper()
	try := func(limit uint64) bool {
		d, err := prog.NewDocument("main", text)
		if err != nil {
			t.Fatal(err)
		}
		first, err := d.SnapshotWith(ParseOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if err := d.Edit(len(text)/2, len(text)/2+1, "b"); err != nil {
			t.Fatal(err)
		}
		if keep {
			defer runtime.KeepAlive(first)
		} else {
			first = nil
			for i := 0; i < 10 && d.HasPreviousSnapshot(); i++ {
				runtime.GC()
			}
			if d.HasPreviousSnapshot() {
				t.Fatal("the first snapshot was not collected")
			}
		}
		_, err = d.SnapshotWith(ParseOptions{Limits: core.Limits{Compute: limit}})
		return err == nil
	}
	lo, hi := uint64(1), uint64(1<<24)
	if !try(hi) {
		t.Fatal("the snapshot fails under a generous limit")
	}
	for lo < hi {
		mid := (lo + hi) / 2
		if try(mid) {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo
}

// The Compute that a snapshot charges is a function of the document's logical state: it does not
// depend on whether the collector has released the previous snapshot.
func TestBudgetSnapshotChargeIndependentOfCollection(t *testing.T) {
	prog := mustCompile(t, `rule main = item* $$
rule item = [a-z]`)
	text := strings.Repeat("a", 600)
	kept := minSecondSnapshotCompute(t, prog, text, true)
	released := minSecondSnapshotCompute(t, prog, text, false)
	if kept != released {
		t.Fatalf("the least Compute differs: %d with the first snapshot alive, %d once collected", kept, released)
	}
}
