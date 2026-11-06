package core

import (
	"fmt"
	"testing"

	"zeroparse.dev/zp/internal/limits"
	"zeroparse.dev/zp/internal/value"
)

// movedKept is the number of copies of empty nodes that moveResult keeps in Parser.Moved for the next
// one: a larger map is dropped instead of cleared.
const movedKept = 1024

// wantMovementState checks the state that ClearMovementSpend leaves: the map of copies is empty and kept
// when it held at most movedKept entries and nil otherwise, and the map of shifts is empty.
func wantMovementState(t *testing.T, p *Parser, moved int) {
	t.Helper()
	if moved > movedKept {
		if p.Moved != nil {
			t.Fatalf("%d copies were kept: %d left", moved, len(p.Moved))
		}
	} else if p.Moved == nil || len(p.Moved) != 0 {
		t.Fatalf("%d copies: the map is %v, want it empty and kept", moved, p.Moved)
	}
	if len(p.Shifts) != 0 {
		t.Fatalf("%d shifts left", len(p.Shifts))
	}
}

// TestClearMovementSpend runs the cleanup that ends every move of a value on a parser without a control
// (a parse without limits, and every build for the reference) and on one with a control, around the size
// at which the map of copies is dropped instead of cleared.
func TestClearMovementSpend(t *testing.T) {
	t.Parallel()
	fill := func(p *Parser, moved, shifts int) {
		p.Moved = map[*value.Node]*value.Node{}
		for range moved {
			n := value.Ptr(value.Data{})
			p.Moved[n] = n
		}
		p.Shifts = map[uint32]int{}
		for i := range shifts {
			p.Shifts[uint32(i)] = i
		}
	}
	for _, moved := range []int{0, 1, movedKept - 1, movedKept, movedKept + 1} {
		for _, shifts := range []int{0, 3} {
			t.Run(fmt.Sprintf("%d copies and %d shifts without a control", moved, shifts), func(t *testing.T) {
				p := &Parser{}
				fill(p, moved, shifts)
				p.ClearMovementSpend()
				wantMovementState(t, p, moved)
			})
			t.Run(fmt.Sprintf("%d copies and %d shifts with a control", moved, shifts), func(t *testing.T) {
				p := &Parser{Input: Input{Control: NewBudgetControl(nil, limits.Limits{Compute: 1 << 30})}}
				fill(p, moved, shifts)
				used := p.Control.Used
				p.ClearMovementSpend()
				wantMovementState(t, p, moved)
				// One unit for each copy forgotten, unless the map is dropped whole, and for each shift.
				want := uint64(shifts)
				if moved <= movedKept {
					want += uint64(moved)
				}
				if !BudgetControlsEnabled {
					want = 0
				}
				if got := p.Control.Used - used; got != want {
					t.Fatalf("%d copies and %d shifts charged %d, want %d", moved, shifts, got, want)
				}
			})
		}
	}
}

// TestDocumentMoveWithoutControl moves a reused result on a parser without a control, as a parse
// without limits does, and checks that the move leaves no state for the next one: the tree has its new
// positions, and the maps of copies and of shifts are empty, the first dropped when it grew past the
// size that is kept.
func TestDocumentMoveWithoutControl(t *testing.T) {
	t.Parallel()
	moves := func(empty int) (*Parser, *value.Node, []*value.Node) {
		p := &Parser{}
		for range 3 {
			p.Edits = append(p.Edits, DocEdit{Delta: 1})
		}
		p.Gen = 3
		// A child moved through another result after one edit has a shift of its own to replay.
		late := value.Ptr(value.Data{Start: 3, End: 4, Gen: 1})
		children := []*value.Node{late}
		var empties []*value.Node
		for range empty {
			c := value.Ptr(value.Data{Start: 3, End: 3})
			empties = append(empties, c)
			children = append(children, c)
		}
		n := value.Ptr(value.Data{Start: 2, End: 5, Children: children})
		return p, n, empties
	}
	for _, empty := range []int{1, movedKept, movedKept + 1} {
		p, n, empties := moves(empty)
		e := &MemoEntry{Node: n, shift: 1, shifted: true, Gen: 0}
		p.moveResult(e)
		if value.D(n).Start != 3 || value.D(n).End != 6 {
			t.Fatalf("%d empty children: the root is [%d,%d), want [3,6)", empty, value.D(n).Start, value.D(n).End)
		}
		// The child that another result moved replays the two edits since its generation.
		if c := value.D(n).Children[0]; value.D(c).Start != 5 || value.D(c).End != 6 {
			t.Fatalf("%d empty children: the late child is [%d,%d), want [5,6)", empty, value.D(c).Start, value.D(c).End)
		}
		for i, c := range value.D(n).Children[1:] {
			if c == empties[i] || value.D(c).Start != 4 || value.D(c).End != 4 {
				t.Fatalf("%d empty children: child %d is %p [%d,%d), want a copy at [4,4)", empty, i, c, value.D(c).Start, value.D(c).End)
			}
		}
		wantMovementState(t, p, empty)
	}
}
