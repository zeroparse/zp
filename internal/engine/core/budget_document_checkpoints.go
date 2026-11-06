package core

import (
	"slices"
	"sort"
)

func (p *Parser) RunSearch(elems []RunElem, start, at int) int {
	if !BudgetControlsEnabled || p.Control == nil {
		return start + sort.Search(len(elems)-start, func(i int) bool { return elems[start+i].Start >= at })
	}
	lo, hi := start, len(elems)
	for lo < hi {
		p.Spend()
		mid := int(uint(lo+hi) >> 1)
		if elems[mid].Start < at {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}

func (p *Parser) RunAfter(elems []RunElem, end int) bool {
	if !BudgetControlsEnabled || p.Control == nil {
		return after(elems, end)
	}
	for _, el := range elems {
		p.Spend()
		if el.From < end {
			return false
		}
	}
	return true
}

// GrowSpend bounds the explicit copy of existing values when capacity must
// grow. Allocation and runtime zeroing remain nonpreemptible intervals.
func GrowSpend[T any](p *Parser, values []T, extra int) []T {
	if !BudgetControlsEnabled || p.Control == nil {
		return slices.Grow(values, extra)
	}
	if extra < 0 {
		return slices.Grow(values, extra) // reject negative growth before spare-capacity checks
	}
	if extra <= cap(values)-len(values) {
		return values
	}
	maxInt := int(^uint(0) >> 1)
	if extra > maxInt-len(values) {
		return slices.Grow(values, extra) // preserve the library's invalid-size panic
	}
	capacity := len(values) + extra
	if cap(values) <= maxInt/2 {
		// Keep repeated single-element additions amortized; allocating only
		// the exact new length would repeatedly copy the accumulated prefix.
		capacity = max(capacity, 2*cap(values))
	}
	out := make([]T, len(values), capacity)
	CopySpend(p, out, values)
	return out
}

// Successful movement cleanup is parse spend; fatal Document cleanup remains
// unbudgeted. Retain the ordinary map-clear path when controls are absent.
func (p *Parser) ClearMovementSpend() {
	if !BudgetControlsEnabled || p.Control == nil {
		if len(p.Moved) > 1024 {
			p.Moved = nil
		} else {
			clear(p.Moved)
		}
		clear(p.Shifts)
		return
	}
	if len(p.Moved) > 1024 {
		p.Moved = nil
	} else {
		for key := range p.Moved {
			p.Spend()
			delete(p.Moved, key)
		}
	}
	for key := range p.Shifts {
		p.Spend()
		delete(p.Shifts, key)
	}
}
