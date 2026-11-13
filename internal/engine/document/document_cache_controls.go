package document

import (
	"unsafe"
	"weak"

	"zeroparse.dev/zp/internal/engine/core"
	"zeroparse.dev/zp/internal/value"
	"zeroparse.dev/zp/runtime/parse"
)

// ResetHistory releases incremental reuse history without changing the text,
// construction options or last parse statistics. Previously returned trees are
// detached from future node movement; this does not undo earlier movement.
func (d *Document) ResetHistory() {
	d.memo = core.NewMemoTable()
	d.edits, d.runs, d.kids, d.machine = nil, nil, nil, nil
	d.runGen, d.resumed = 0, 0
	d.in.Offs, d.in.Lines = nil, nil
	d.previousSnapshot, d.previousRoot = weak.Pointer[snapshotGraph]{}, 0
}

// historyBytes returns the bytes of the repetition records and edits the Document retains, as a
// parse under a memory limit counts them. The memo table, the child stack and the backend's stacks
// are counted by the parse itself (core.BudgetControl.MemoryUsed).
func (d *Document) historyBytes() uint64 {
	var n uint64
	for _, run := range d.runs {
		n += uint64(unsafe.Sizeof(core.RunRecord{})) + uint64(cap(run.Elems))*uint64(unsafe.Sizeof(core.RunElem{}))
	}
	return n + uint64(cap(d.edits))*uint64(unsafe.Sizeof(core.DocEdit{}))
}

// retainedBytes returns the bytes a parse under a memory limit starts with: the history, the memo
// table's count, and the kept child and backend stacks.
func (d *Document) retainedBytes() uint64 {
	n := d.historyBytes() + uint64(cap(d.kids))*uint64(unsafe.Sizeof((*value.Node)(nil)))
	if d.memo != nil {
		n += d.memo.ChargedBytes()
	}
	if d.machine != nil {
		n += d.machine.StackBytes()
	}
	return n
}

// CacheStats returns a point-in-time estimate of cache-owned retained buffers.
// It does not measure process heap usage or bytes returned to the OS.
func (d *Document) CacheStats() parse.CacheStats {
	var s parse.CacheStats
	t := d.memo
	if t != nil {
		for _, head := range t.Slots {
			for e := head; e != nil; e = e.Next {
				s.LiveMemoEntries++
			}
		}
		for e := t.Free; e != nil; e = e.Next {
			s.RetiredMemoEntries++
		}
		s.EstimatedBytes += uint64(cap(t.Slots)) * uint64(unsafe.Sizeof((*core.MemoEntry)(nil)))
		s.EstimatedBytes += uint64(cap(t.Chunks)) * uint64(unsafe.Sizeof([]core.MemoEntry(nil)))
		for _, chunk := range t.Chunks {
			s.EstimatedBytes += uint64(cap(chunk)) * uint64(unsafe.Sizeof(core.MemoEntry{}))
		}
		s.EstimatedBytes += uint64(cap(t.Seen)) * uint64(unsafe.Sizeof(uint64(0)))
		s.EstimatedBytes += uint64(cap(t.Calls)) * uint64(unsafe.Sizeof(core.SeenCalls{}))
		s.EstimatedBytes += uint64(cap(t.Pages)) * uint64(unsafe.Sizeof(core.SeenRulePages{}))
		for _, dir := range t.Pages {
			s.EstimatedBytes += uint64(cap(dir.Pages)) * uint64(unsafe.Sizeof((*[16]uint64)(nil)))
		}
		s.EstimatedBytes += uint64(cap(t.BitPages.Chunks)) * uint64(unsafe.Sizeof((*[64][16]uint64)(nil)))
		s.EstimatedBytes += uint64(len(t.BitPages.Chunks)) * uint64(unsafe.Sizeof([64][16]uint64{}))
	}
	for _, run := range d.runs {
		s.RepetitionElements += len(run.Elems)
		s.EstimatedBytes += uint64(unsafe.Sizeof(core.RunRecord{}))
		s.EstimatedBytes += uint64(cap(run.Elems)) * uint64(unsafe.Sizeof(core.RunElem{}))
	}
	s.EstimatedBytes += uint64(cap(d.edits)) * uint64(unsafe.Sizeof(core.DocEdit{}))
	s.EstimatedBytes += uint64(cap(d.kids)) * uint64(unsafe.Sizeof((*value.Node)(nil)))
	if d.machine != nil {
		s.EstimatedBytes += d.machine.RetainedBytes()
	}
	return s
}
