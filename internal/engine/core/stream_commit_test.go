package core

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/runtime/parse"
)

// TestStreamCommitWindow pins when a commit prunes and what the pruning leaves: it waits for a whole
// window past the last pruning, and everything from the committed position on stays.
func TestStreamCommitWindow(t *testing.T) {
	p := NewControlledStreamParser(&SharedProgram{}, strings.NewReader(strings.Repeat("a", 4096)), parse.CodePoints, NewBudgetControl(nil, parse.Limits{Compute: 1 << 40}))
	p.Fill(4096)
	at := func(pos int) bool {
		_, ok := p.MemoGet(MemoKey{Rule: 1, Pos: pos})
		return ok
	}
	for _, pos := range []int{1023, 1024, 2047, 2048} {
		p.MemoPut(MemoKey{Rule: 1, Pos: pos}, p.Memo.Alloc())
	}
	for _, step := range []struct {
		pos, floor int
		gone, kept []int
	}{
		{1023, 0, nil, []int{1023, 1024}},
		{1024, 1024, []int{1023}, []int{1024, 2047}},
		{2047, 1024, nil, []int{1024, 2047}},
		{2048, 2048, []int{1024, 2047}, []int{2048}},
	} {
		p.Commit(step.pos)
		if p.Memo.Base != step.floor {
			t.Errorf("after a commit at %d the floor is %d, not %d", step.pos, p.Memo.Base, step.floor)
		}
		for _, pos := range step.gone {
			if at(pos) {
				t.Errorf("after a commit at %d the memo holds position %d", step.pos, pos)
			}
		}
		for _, pos := range step.kept {
			if !at(pos) {
				t.Errorf("after a commit at %d the memo lost position %d", step.pos, pos)
			}
		}
	}
}

// TestStreamCommitDropsRecordsBeforeItsPosition pins the position of the records a pruning commit
// drops: a repetition may still resume from a record at the committed position.
func TestStreamCommitDropsRecordsBeforeItsPosition(t *testing.T) {
	p := NewControlledStreamParser(&SharedProgram{}, strings.NewReader(strings.Repeat("a", 2048)), parse.CodePoints, nil)
	p.Fill(2048)
	s := &skipSlots{}
	p.skipTables = []*skipSlots{s}
	s.put(1023, nil, skipRecord{state: skipRecorded})
	s.put(1024, nil, skipRecord{state: skipRecorded})
	p.Commit(1024)
	if got := s.get(1023, nil).state; got != skipFresh {
		t.Errorf("the record before the commit has the state %d", got)
	}
	if got := s.get(1024, nil).state; got != skipRecorded {
		t.Errorf("the record at the commit has the state %d", got)
	}
}
