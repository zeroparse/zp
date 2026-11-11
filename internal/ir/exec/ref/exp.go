package ref

import (
	"fmt"
	"slices"

	"zeroparse.dev/zp/internal/ir/exec"
)

// The reference model of the expectation record (component expected): the current record is an
// insertion-ordered list of entries at the farthest position, isolations set records aside in a
// stack, and kept sets are immutable lists that outlive the record. The observable contract is the
// farthest position and the exact order of the entries, which error messages expose.

// ExpID is the model's handle of an expectation: a conformance identifier, or the placeholder that a
// parse without expectations keeps in the record of a failure.
type ExpID struct{ ID int64 }

// Placeholder is the model's placeholder entry. It is not an expectation; no conformance identifier
// is negative.
var Placeholder = ExpID{ID: -1}

// ExpMark is the model's handle of the record that exp.isolate set aside.
type ExpMark struct{ ended bool }

// ExpSet is the model's handle of the expectations that exp.unisolate returned. The model keeps them as
// a list of their own; a target may keep them where the record grows next, so the model allows reading
// entry i only while every write since exp.unisolate, of the record it restored, was at an index
// before restored + i, and no other isolation began or ended. Merging the set into that record entry
// by entry meets this: each write follows the read of the entry at its index or after.
type ExpSet struct {
	ids []ExpID
	// restored is the number of entries of the restored record when exp.unisolate returned the set;
	// high is one more than the largest index of that record written since, at least restored.
	restored, high int
	// dead reports that an isolation began or ended after exp.unisolate returned the set.
	dead bool
}

// IDs returns the entries of s.
func (s *ExpSet) IDs() []ExpID { return slices.Clone(s.ids) }

// ExpKept is the model's handle of a set kept beyond the record. It never changes once made.
type ExpKept struct{ ids []ExpID }

// emptyKept is the set that exp.empty returns, and exp.last before the first exp.copy.
var emptyKept = &ExpKept{}

// expRecord is one record of expectations.
type expRecord struct {
	far int64
	ids []ExpID
}

// expState is the state of the component expected.
type expState struct {
	cur   expRecord
	saved []expRecord
	marks []*ExpMark
	// set is the set that exp.unisolate returned last, while no isolation began or ended since.
	set  *ExpSet
	last *ExpKept
}

// ExpRecord returns the farthest position and the entries of the current record, for tests.
func (m *Model) ExpRecord() (int64, []ExpID) { return m.exp.cur.far, slices.Clone(m.exp.cur.ids) }

// write records that the entry of the current record at index i is written: the set that exp.unisolate
// returned last loses its entries up to the one that a target may keep at that index (ExpSet).
func (s *expState) write(i int) {
	if s.set != nil {
		s.set.high = max(s.set.high, i+1)
	}
}

// endSet makes the set that exp.unisolate returned last invalid.
func (s *expState) endSet() {
	if s.set != nil {
		s.set.dead = true
		s.set = nil
	}
}

// allowsExp reports whether a call of an operation of the expectation record meets its precondition.
func (m *Model) allowsExp(o *exec.Op, args []any) bool {
	s := &m.exp
	switch o.Name {
	case "exp.unisolate":
		mk := args[0].(*ExpMark)
		return !mk.ended && len(s.marks) > 0 && s.marks[len(s.marks)-1] == mk
	case "exp.maybe":
		// The placeholder is not an expectation, and the filter need not hold it.
		return args[0].(ExpID) != Placeholder
	case "exp.at":
		i := args[0].(int64)
		return 0 <= i && i < int64(len(s.cur.ids))
	case "exp.setAt":
		x, i := args[0].(*ExpSet), args[1].(int64)
		return 0 <= i && i < int64(len(x.ids)) && !x.dead && x.high <= x.restored+int(i)
	case "exp.keptAt":
		k, i := args[0].(*ExpKept), args[1].(int64)
		return 0 <= i && i < int64(len(k.ids))
	case "exp.copy":
		x := args[0].(*ExpSet)
		return len(x.ids) > 0 && !x.dead && x.high <= x.restored
	case "memo.far", "memo.expected":
		e := args[0].(*Entry)
		return !e.pruned && e.expected != nil
	case "memo.setExpected":
		return !args[0].(*Entry).pruned
	}
	return true
}

// callExp runs an operation of the expectation record, and reports false for any other.
func (m *Model) callExp(o *exec.Op, args []any) ([]any, bool) {
	s := &m.exp
	switch o.Name {
	case "exp.isolate":
		s.endSet()
		s.saved = append(s.saved, s.cur)
		s.cur = expRecord{far: args[0].(int64)}
		mk := &ExpMark{}
		s.marks = append(s.marks, mk)
		return []any{mk}, true
	case "exp.unisolate":
		s.endSet()
		mk := s.marks[len(s.marks)-1]
		mk.ended = true
		s.marks = s.marks[:len(s.marks)-1]
		ended := s.cur
		s.cur, s.saved = s.saved[len(s.saved)-1], s.saved[:len(s.saved)-1]
		x := &ExpSet{ids: ended.ids, restored: len(s.cur.ids), high: len(s.cur.ids)}
		s.set = x
		return []any{ended.far, x}, true
	case "exp.farthest":
		return []any{s.cur.far}, true
	case "exp.recording":
		return []any{!m.cfg.NoExpectations}, true
	case "exp.reset":
		s.cur = expRecord{far: args[0].(int64)}
		return nil, true
	case "exp.fresh":
		return []any{len(s.cur.ids) == 0}, true
	case "exp.placeholder":
		s.write(len(s.cur.ids))
		s.cur.ids = append(slices.Clip(s.cur.ids), Placeholder)
		return nil, true
	case "exp.maybe":
		// The model's filter is exact; an implementation may answer true for an id that is not in the
		// record (conform accepts it).
		return []any{slices.Contains(s.cur.ids, args[0].(ExpID))}, true
	case "exp.count":
		return []any{int64(len(s.cur.ids))}, true
	case "exp.at":
		return []any{s.cur.ids[args[0].(int64)]}, true
	case "exp.add":
		s.write(len(s.cur.ids))
		s.cur.ids = append(slices.Clip(s.cur.ids), args[0].(ExpID))
		return nil, true
	case "exp.setCount":
		return []any{int64(len(args[0].(*ExpSet).ids))}, true
	case "exp.setAt":
		return []any{args[0].(*ExpSet).ids[args[1].(int64)]}, true
	case "exp.keptCount":
		return []any{int64(len(args[0].(*ExpKept).ids))}, true
	case "exp.keptAt":
		return []any{args[0].(*ExpKept).ids[args[1].(int64)]}, true
	case "exp.empty":
		return []any{emptyKept}, true
	case "exp.last":
		if s.last == nil {
			return []any{emptyKept}, true
		}
		return []any{s.last}, true
	case "exp.copy":
		s.last = &ExpKept{ids: slices.Clone(args[0].(*ExpSet).ids)}
		return []any{s.last}, true
	case "memo.far":
		return []any{args[0].(*Entry).far}, true
	case "memo.expected":
		return []any{args[0].(*Entry).expected}, true
	case "memo.setExpected":
		e := args[0].(*Entry)
		e.far, e.expected = args[1].(int64), args[2].(*ExpKept)
		return nil, true
	}
	return nil, false
}

// setSize returns ops.QSet for a call whose first argument is a set.
func setSize(args []any) int64 {
	switch x := args[0].(type) {
	case *ExpSet:
		return int64(len(x.ids))
	case *ExpKept:
		return int64(len(x.ids))
	}
	panic(fmt.Sprintf("ref: %T is not a set", args[0]))
}
