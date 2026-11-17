package libgo

import (
	"math/rand"
	"testing"
)

// scanModelRun is a run of the model that TestScanTableModel compares the table with.
type scanModelRun struct {
	id         int32
	first, end int
}

// TestScanTableModel makes and extends the runs of one table in random orders, over stretches of
// random lengths, and prunes it as a stream does; after each step every position must find the run that
// covers it, by the number that the run got when it was made, and no other.
func TestScanTableModel(t *testing.T) {
	for seed := int64(1); seed <= 100; seed++ {
		rng := rand.New(rand.NewSource(seed))
		// Stretches [s, e) of characters of the class, each followed by its end e.
		type stretch struct{ s, e int }
		var stretches []stretch
		pos := rng.Intn(200)
		for range 1 + rng.Intn(40) {
			n := 16 + rng.Intn(150)
			stretches = append(stretches, stretch{pos, pos + n})
			pos += n + 1 + rng.Intn(3)*rng.Intn(100)
		}
		size := pos + 1
		tab := &scanTable{}
		p := &Parser{scanTabs: []*scanTable{tab}}
		runs := map[int]*scanModelRun{} // by stretch
		pruned := 0
		check := func(step string) {
			t.Helper()
			for q := pruned; q < size; q++ {
				var want int32
				for _, r := range runs {
					if r.first <= q && q <= r.end && r.end > pruned {
						want = r.id
					}
				}
				if got := tab.at(q); got != want {
					t.Fatalf("seed %d, %s: At(%d) = %d, want %d", seed, step, q, got, want)
				}
			}
		}
		for step := 0; step < 3*len(stretches); step++ {
			i := rng.Intn(len(stretches))
			st := stretches[i]
			if st.e <= pruned {
				continue
			}
			r := runs[i]
			switch {
			case r == nil:
				first := max(st.s+rng.Intn(st.e-st.s-15), pruned)
				if first > st.e-16 {
					continue
				}
				id := tab.insert(scanRun{first: first, end: st.e, count: st.e - first})
				runs[i] = &scanModelRun{id: id, first: first, end: st.e}
			case r.first > max(st.s, pruned):
				start := max(st.s, pruned) + rng.Intn(r.first-max(st.s, pruned))
				tr := &tab.runs[r.id-1]
				old := tr.first
				tr.first, tr.count = start, tr.count+old-start
				tab.extend(r.id, start, old)
				r.first = start
			case rng.Intn(4) == 0:
				pruned = max(pruned, st.s+rng.Intn(st.e-st.s))
				p.pruneScanRecords(pruned)
				// Pruning renumbers the runs it keeps, in the order they were made.
				var kept []*scanModelRun
				for j, mr := range runs {
					if mr.end <= pruned {
						delete(runs, j)
					} else {
						kept = append(kept, mr)
					}
				}
				for n := range tab.runs {
					for _, mr := range kept {
						if tab.runs[n].end == mr.end {
							mr.id = int32(n + 1)
						}
					}
				}
			}
			check("step")
		}
	}
}

// TestScanTableReverseOrder makes runs from the last stretch back to the first, as the levels of a
// failing recursion do: no run moves, so each keeps its number.
func TestScanTableReverseOrder(t *testing.T) {
	tab := &scanTable{}
	const n = 2000
	ids := make([]int32, n)
	for i := n - 1; i >= 0; i-- {
		first := 20 * i
		ids[i] = tab.insert(scanRun{first: first, end: first + 17, count: 17})
	}
	for i := range n {
		for q := 20 * i; q <= 20*i+17; q++ {
			if got := tab.at(q); got != ids[i] {
				t.Fatalf("At(%d) = %d, want %d", q, got, ids[i])
			}
		}
		for q := 20*i + 18; q < 20*i+20; q++ {
			if got := tab.at(q); got != 0 {
				t.Fatalf("At(%d) = %d, want 0", q, got)
			}
		}
	}
}
