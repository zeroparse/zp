package linearity

import "testing"

// TestLinearSeesQuadraticCost checks the verdict on costs of a known growth: a cost that grows with
// the size passes, and one that grows with its square fails.
func TestLinearSeesQuadraticCost(t *testing.T) {
	const n = 1000
	if r, ok := Linear(5*n+30, 5*Factor*n+30); !ok {
		t.Errorf("linear cost failed with ratio %.1f", r)
	}
	if r, ok := Linear(n*n, Factor*n*Factor*n); ok {
		t.Errorf("quadratic cost passed with ratio %.1f", r)
	}
	if r, ok := Linear(0, Factor); !ok || r != Factor {
		t.Errorf("a cost of 0 at the small size: ratio %.1f, ok %v", r, ok)
	}
}

// TestLeastLimitIsExact checks that the search finds the exact cost of work that stops at the first
// charge past its limit, for costs at and between the steps of the search.
func TestLeastLimitIsExact(t *testing.T) {
	for _, cost := range []uint64{1, 2, 1023, 1024, 1025, 4097, 1_000_003} {
		runs := 0
		got := LeastLimit(t, func(limit uint64) bool {
			runs++
			return cost <= limit
		})
		if got != cost {
			t.Errorf("cost %d: found %d", cost, got)
		}
		if runs > 64 {
			t.Errorf("cost %d: %d runs", cost, runs)
		}
	}
}

var sink []*[64]byte

// TestAllocsCountsExactly checks that the counts are those of the work, and the same on every run.
func TestAllocsCountsExactly(t *testing.T) {
	work := func() {
		sink = make([]*[64]byte, 0, 100)
		for range 100 {
			sink = append(sink, new([64]byte))
		}
	}
	objects, bytes := Allocs(work)
	if objects != 101 || bytes < 100*64 {
		t.Errorf("%d objects, %d bytes; want 101 objects and at least %d bytes", objects, bytes, 100*64)
	}
	for range 3 {
		if o, b := Allocs(work); o != objects || b != bytes {
			t.Errorf("a run counted %d objects and %d bytes, the first %d and %d", o, b, objects, bytes)
		}
	}
}
