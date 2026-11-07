//go:build !zp_budget_controls_reference

package core

import (
	"errors"
	"math"
	"testing"

	"zeroparse.dev/zp/internal/limits"
)

// The operations of a region of Document work declare a cost of zero: they open and close the region
// and reserve no unit. The units reserved inside are the work they cover, and a default limit does
// not count them, whatever their number; an explicit limit counts every one.
func TestFreeRegionChargesExactly(t *testing.T) {
	const limit = 1000
	for _, work := range []uint64{0, 1, 999, limit, limit + 1, 1 << 40, math.MaxUint64 / 4} {
		c := &BudgetControl{Limits: limits.Limits{Compute: limit}, DefaultCompute: true}
		mark := c.FreeBegin()
		if c.Used != 0 || c.Doc != 0 {
			t.Fatalf("work %d: FreeBegin reserved %d units", work, c.Used)
		}
		c.Charge(work)
		c.FreeEnd(mark)
		if c.Used != work || c.Doc != work {
			t.Fatalf("work %d: used %d, Document units %d", work, c.Used, c.Doc)
		}
		if c.Limits.Compute != limit+work {
			t.Fatalf("work %d: the limit is %d after the region, want %d", work, c.Limits.Compute, limit+work)
		}
		// Counted work has the room of the whole limit left: limit units pass, one more does not.
		c.Charge(limit)
		var le *limits.LimitError
		func() {
			defer func() {
				if f, ok := recover().(Fatal); ok {
					errors.As(f.Err, &le)
				}
			}()
			c.Charge(1)
		}()
		if le == nil || le.Limit != limit || le.Used != limit {
			t.Fatalf("work %d: counted work stopped with %+v, want limit and used %d", work, le, limit)
		}
	}
}

// ChargeDoc reserves exactly the units it is given, as Charge does, and counts them as Document work
// under a default limit only.
func TestChargeDocChargesExactly(t *testing.T) {
	c := &BudgetControl{Limits: limits.Limits{Compute: 100}, DefaultCompute: true}
	c.ChargeDoc(70)
	c.ChargeDoc(70)
	c.Charge(30)
	if c.Used != 170 || c.Doc != 140 {
		t.Fatalf("default limit: used %d (want 170), Document units %d (want 140)", c.Used, c.Doc)
	}
	// Under an explicit limit the units count: 70 + 70 passes 100 only by the limit.
	c = &BudgetControl{Limits: limits.Limits{Compute: 100}}
	c.ChargeDoc(70)
	if c.Used != 70 || c.Doc != 0 {
		t.Fatalf("explicit limit: used %d (want 70), Document units %d (want 0)", c.Used, c.Doc)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("an explicit limit of 100 let 140 units of Document work pass")
		}
	}()
	c.ChargeDoc(70)
}

// A region opens at most once, and none opens under an explicit limit or on no control.
func TestFreeRegionDoesNotNest(t *testing.T) {
	c := &BudgetControl{Limits: limits.Limits{Compute: 10}, DefaultCompute: true}
	outer := c.FreeBegin()
	inner := c.FreeBegin()
	if inner != noFree {
		t.Fatal("a region opened inside a region")
	}
	c.Charge(5)
	c.FreeEnd(inner)
	c.Charge(5)
	c.FreeEnd(outer)
	if c.Used != 10 || c.Doc != 10 || c.Limits.Compute != 20 {
		t.Fatalf("used %d, Document units %d, limit %d", c.Used, c.Doc, c.Limits.Compute)
	}
	explicit := &BudgetControl{Limits: limits.Limits{Compute: 10}}
	if explicit.FreeBegin() != noFree {
		t.Fatal("a region opened under an explicit limit")
	}
	var none *BudgetControl
	none.FreeEnd(none.FreeBegin())
}
