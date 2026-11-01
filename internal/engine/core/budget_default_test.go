//go:build !zp_budget_controls_reference

package core

import (
	"testing"

	"zeroparse.dev/zp/internal/limits"
)

func TestResolveLimits(t *testing.T) {
	l, adaptive := ResolveLimits(limits.Limits{}, 10, false)
	if l.Compute != limits.DefaultBudgetFloor || adaptive {
		t.Fatalf("short input: %+v adaptive=%t", l, adaptive)
	}
	if l, _ := ResolveLimits(limits.Limits{}, 1<<20, false); l.Compute != 1<<30 {
		t.Fatalf("large input: %+v", l)
	}
	if l, _ := ResolveLimits(limits.Limits{Compute: 7}, 1<<20, false); l.Compute != 7 {
		t.Fatalf("explicit: %+v", l)
	}
	if l, _ := ResolveLimits(limits.Limits{Compute: limits.Unlimited}, 1<<20, false); l.Compute != 0 {
		t.Fatalf("unlimited: %+v", l)
	}
	l, adaptive = ResolveLimits(limits.Limits{}, 0, true)
	if l.Compute != limits.DefaultBudgetFloor || !adaptive {
		t.Fatalf("stream: %+v adaptive=%t", l, adaptive)
	}
}

// A stream's default budget follows the furthest position demanded, and never shrinks.
func TestDemand(t *testing.T) {
	l, adaptive := ResolveLimits(limits.Limits{}, 0, true)
	c := NewBudgetControl(nil, l)
	c.Adaptive = adaptive
	c.Demand(100)
	if c.Limits.Compute != limits.DefaultBudgetFloor {
		t.Fatalf("below the floor: %d", c.Limits.Compute)
	}
	c.Demand(3 << 20)
	if c.Limits.Compute != 3<<30 {
		t.Fatalf("grown: %d", c.Limits.Compute)
	}
	c.Demand(5)
	if c.Limits.Compute != 3<<30 {
		t.Fatalf("shrunk: %d", c.Limits.Compute)
	}
	c.Demand(3<<20 + 1)
	if c.Limits.Compute != (3<<20+1)*1024 {
		t.Fatalf("grown again: %d", c.Limits.Compute)
	}
	var none *BudgetControl
	none.Demand(5)
	fixed := NewBudgetControl(nil, limits.Limits{Compute: 9})
	fixed.Demand(1 << 30)
	if fixed.Limits.Compute != 9 {
		t.Fatalf("explicit budget grew: %d", fixed.Limits.Compute)
	}
}
