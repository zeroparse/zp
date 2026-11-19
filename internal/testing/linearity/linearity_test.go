package linearity

import (
	"runtime"
	"sync/atomic"
	"testing"
)

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

var finalizerSink atomic.Pointer[[64]byte]

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

// TestAllocsIgnoresPendingFinalizers checks that finalizers queued before the measurement, which
// allocate when they run, do not run inside it. Without the wait for them, the finalizer goroutine
// runs them during the measured work and its allocations are counted as the work's.
func TestAllocsIgnoresPendingFinalizers(t *testing.T) {
	var ran atomic.Int64
	const garbage = 2000
	for range garbage {
		o := new([32]byte)
		runtime.SetFinalizer(o, func(*[32]byte) {
			finalizerSink.Store(new([64]byte)) // allocates, as a finalizer that closes a file does
			ran.Add(1)
		})
	}
	work := func() {
		sink = make([]*[64]byte, 0, 100)
		for range 100 {
			sink = append(sink, new([64]byte))
		}
	}
	if objects, _ := Allocs(work); objects != 101 {
		t.Errorf("%d objects with finalizers pending; want 101", objects)
	}
	if n := ran.Load(); n != garbage {
		t.Errorf("%d of %d finalizers had run when Allocs returned", n, garbage)
	}
}

// TestDrainFinalizersWaitsForEveryFinalizer checks that drainFinalizers returns only after all the
// finalizers of the garbage have run, however the finalizer goroutine is scheduled. The runtime does
// not run a list of finalizers in the order of queueing, so a single marker finalizer can return
// while the batch is still pending; many short trials make that visible.
func TestDrainFinalizersWaitsForEveryFinalizer(t *testing.T) {
	const garbage = 2000
	trials := 200
	if testing.Short() {
		trials = 20
	}
	for trial := range trials {
		var ran atomic.Int64
		for range garbage {
			o := new([32]byte)
			runtime.SetFinalizer(o, func(*[32]byte) {
				finalizerSink.Store(new([64]byte))
				ran.Add(1)
			})
		}
		drainFinalizers()
		if n := ran.Load(); n != garbage {
			t.Fatalf("trial %d: drainFinalizers returned with %d of %d finalizers run", trial, n, garbage)
		}
	}
}
