package runtime

import (
	"math/rand"
	"reflect"
	"slices"
	"testing"
)

// referenceInvalidate is a copying splice, written from the contract of memo.splice. It returns
// the retained chains as rule ids by position, and the metadata of the retained entries by rule id.
func referenceInvalidate(d *typedDocumentState, ed documentEdit) ([][]int32, map[int32]typedDocumentMeta) {
	kept := make([]*tmemoEntry, len(d.slots)+max(ed.delta, 0))
	meta := make(map[*tmemoEntry]typedDocumentMeta)
	for pos, head := range d.slots {
		for e := head; e != nil; e = e.next {
			m := d.meta[e]
			from := pos
			if m.before {
				from--
			}
			to := pos
			switch {
			case !m.recorded:
				continue
			case m.examined <= ed.start:
			case from >= ed.end && !m.pinned && e.errs.none():
				to += ed.delta
				m.examined += ed.delta
				m.shift += ed.delta
			default:
				continue
			}
			out := &tmemoEntry{}
			*out = *e
			out.next = kept[to]
			kept[to] = out
			meta[out] = m
		}
	}
	// The table ends after the last position that keeps an entry.
	for len(kept) > 0 && kept[len(kept)-1] == nil {
		kept = kept[:len(kept)-1]
	}
	ids := make([][]int32, len(kept))
	byID := make(map[int32]typedDocumentMeta)
	for pos, head := range kept {
		for e := head; e != nil; e = e.next {
			ids[pos] = append(ids[pos], e.rule)
			byID[e.rule] = meta[e]
		}
		slices.Sort(ids[pos])
	}
	return ids, byID
}

// inPlaceResult reads the chains and metadata of d in the same form as referenceInvalidate.
func inPlaceResult(d *typedDocumentState) ([][]int32, map[int32]typedDocumentMeta, int) {
	ids := make([][]int32, len(d.slots))
	byID := make(map[int32]typedDocumentMeta)
	for pos, head := range d.slots {
		for e := head; e != nil; e = e.next {
			ids[pos] = append(ids[pos], e.rule)
			byID[e.rule] = d.meta[e]
		}
		slices.Sort(ids[pos])
	}
	return ids, byID, len(d.meta)
}

// TestInvalidateMatchesCopyingReference pins the relinking splice to a copying one: the same entries
// survive at the same positions, with the same metadata, and no metadata is left for an entry that
// is no longer linked. The copying version reverses each chain; chain order does not affect lookups,
// which match entries by key, so the order is compared as a set.
func TestInvalidateMatchesCopyingReference(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for round := 0; round < 500; round++ {
		n := 1 + rng.Intn(12)
		build := func() *typedDocumentState {
			r := rand.New(rand.NewSource(int64(round)))
			d := &typedDocumentState{meta: make(map[*tmemoEntry]typedDocumentMeta)}
			d.slots = make([]*tmemoEntry, n)
			id := int32(0)
			for pos := range n {
				for k := r.Intn(4); k > 0; k-- {
					e := &tmemoEntry{rule: id}
					id++
					e.next = d.slots[pos]
					d.slots[pos] = e
					if r.Intn(8) == 0 {
						e.errs = recEntry{err: &SyntaxError{}}
					}
					switch r.Intn(6) {
					case 0: // incomplete: no metadata
					case 1: // pinned without a record of the examined input
						d.meta[e] = typedDocumentMeta{pinned: true}
					default:
						// An entry that examined no input records the empty range at its position.
						d.meta[e] = typedDocumentMeta{examined: pos + r.Intn(7), recorded: true, before: pos > 0 && r.Intn(3) == 0, pinned: r.Intn(6) == 0}
					}
				}
			}
			return d
		}
		start := rng.Intn(n + 3)
		end := start + rng.Intn(4)
		ed := documentEdit{start, end, start - end + rng.Intn(4)}
		ref := build()
		wantIDs, wantMeta := referenceInvalidate(ref, ed)
		got := build()
		got.splice(ed)
		gotIDs, gotMeta, metaLen := inPlaceResult(got)
		if !reflect.DeepEqual(gotIDs, wantIDs) {
			t.Fatalf("round %d edit %v: chains %v, want %v", round, ed, gotIDs, wantIDs)
		}
		if !reflect.DeepEqual(gotMeta, wantMeta) {
			t.Fatalf("round %d edit %v: metadata %v, want %v", round, ed, gotMeta, wantMeta)
		}
		if metaLen != len(wantMeta) {
			t.Fatalf("round %d edit %v: %d metadata entries kept, want %d", round, ed, metaLen, len(wantMeta))
		}
		for pos, head := range got.slots[len(got.slots):cap(got.slots)] {
			if head != nil {
				t.Fatalf("round %d edit %v: stale chain at %d beyond the kept length", round, ed, len(got.slots)+pos)
			}
		}
	}
}
