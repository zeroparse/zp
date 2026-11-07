package runtime

import (
	"errors"
	"math"
	"testing"
)

// The operations of a region of Document work declare a cost of zero: they open and close the region
// and reserve no unit. The units reserved inside are the work they cover, and a default limit does
// not count them, whatever their number; an explicit limit counts every one.
func TestFreeRegionChargesExactly(t *testing.T) {
	const limit = 1000
	for _, work := range []uint64{0, 1, 999, limit, limit + 1, 1 << 40, math.MaxUint64 / 4} {
		c := newBudgetControl(nil, Limits{Compute: limit})
		c.defaultCompute = true
		mark := c.FreeBegin(true)
		if c.budgetUsed() != 0 || c.doc != 0 {
			t.Fatalf("work %d: FreeBegin reserved %d units", work, c.budgetUsed())
		}
		c.Charge(work)
		c.FreeEnd(mark)
		if c.budgetUsed() != work || c.doc != work {
			t.Fatalf("work %d: used %d, Document units %d", work, c.budgetUsed(), c.doc)
		}
		if c.limits.Compute != limit+work {
			t.Fatalf("work %d: the limit is %d after the region, want %d", work, c.limits.Compute, limit+work)
		}
		c.Charge(limit)
		var le *LimitError
		func() {
			defer func() {
				if f, ok := recover().(fatal); ok {
					errors.As(f.err, &le)
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
	c := newBudgetControl(nil, Limits{Compute: 100})
	c.defaultCompute = true
	c.ChargeDoc(70)
	c.ChargeDoc(70)
	c.Charge(30)
	if c.budgetUsed() != 170 || c.doc != 140 {
		t.Fatalf("default limit: used %d (want 170), Document units %d (want 140)", c.budgetUsed(), c.doc)
	}
	c = newBudgetControl(nil, Limits{Compute: 100})
	c.ChargeDoc(70)
	if c.budgetUsed() != 70 || c.doc != 0 {
		t.Fatalf("explicit limit: used %d (want 70), Document units %d (want 0)", c.budgetUsed(), c.doc)
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
	c := newBudgetControl(nil, Limits{Compute: 10})
	c.defaultCompute = true
	outer := c.FreeBegin(true)
	inner := c.FreeBegin(true)
	if inner != noFree {
		t.Fatal("a region opened inside a region")
	}
	c.Charge(5)
	c.FreeEnd(inner)
	c.Charge(5)
	c.FreeEnd(outer)
	if c.budgetUsed() != 10 || c.doc != 10 || c.limits.Compute != 20 {
		t.Fatalf("used %d, Document units %d, limit %d", c.budgetUsed(), c.doc, c.limits.Compute)
	}
	explicit := newBudgetControl(nil, Limits{Compute: 10})
	if explicit.FreeBegin(true) != noFree {
		t.Fatal("a region opened under an explicit limit")
	}
	if c.FreeBegin(false) != noFree {
		t.Fatal("a region opened with on false")
	}
	var none *BudgetControl
	none.FreeEnd(none.FreeBegin(true))
}
