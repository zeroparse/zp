package libgo

import "testing"

// An edit drops the entries that more than pendingEditLimit edits have passed since a parse stored them
// or found them to answer a lookup (seen, against the edit count of the Document, memoTable.epoch): a lookup treats such an
// entry as a miss, and the entries of earlier texts that a Document retains stay bounded by that many edits.
func TestSpliceDropsEntriesBeyondTheEditLimit(t *testing.T) {
	const epoch = 200
	seen := []uint32{epoch - pendingEditLimit - 1, epoch - pendingEditLimit, epoch}
	want := []bool{false, true, true}

	var tbl memoTable
	for i, s := range seen {
		e := tbl.alloc()
		e.from, e.examined, e.seen = fromPos, 0, s
		tbl.put(i, 3, 0, nil, e)
	}
	tbl.epoch = epoch
	tbl.splice(documentEdit{start: 50, end: 50})
	for i, w := range want {
		if got := tbl.get(i, 3, 0, nil) != nil; got != w {
			t.Errorf("entry seen at %d of %d edits: kept %v, want %v", seen[i], epoch, got, w)
		}
	}

	d := &typedDocumentState{owner: &documentState{epoch: epoch}, meta: map[*tmemoEntry]typedDocumentMeta{}}
	d.slots = make([]*tmemoEntry, 4)
	for i, s := range seen {
		e := &tmemoEntry{rule: int32(i), seen: s}
		e.next, d.slots[3] = d.slots[3], e
		d.meta[e] = typedDocumentMeta{recorded: true}
	}
	d.splice(documentEdit{start: 50, end: 50})
	kept := map[int32]bool{}
	for e := d.slots[3]; e != nil; e = e.next {
		kept[e.rule] = true
	}
	for i, w := range want {
		if kept[int32(i)] != w {
			t.Errorf("typed entry seen at %d of %d edits: kept %v, want %v", seen[i], epoch, kept[int32(i)], w)
		}
	}
	if len(d.meta) != 2 {
		t.Errorf("%d metadata entries kept, want 2", len(d.meta))
	}
}
