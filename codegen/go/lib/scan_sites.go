package libgo

import (
	"unsafe"

	"zeroparse.dev/zp/core"
)

// Scan sites: the records of the walks of the repetitions of one character without a maximum in
// the rules that call no rule, a mirror of the engine's (internal/engine/state/scan_sites.go; see
// "Scan Sites" in spec/runtime/parsing.md and the proof in spec/proofs/run-sites.md).
//
// An iteration of such a repetition reads one character and nothing else, so the walk from a
// start ends at a position, matches a number of characters and builds elements that depend on the
// start and the input alone. A site keeps at most one record for each start, made by a walk that
// matched scanHotSpan characters or more: the end, the count and, for a walk that builds a value
// (the site after the value-free one), the elements. A walk that has matched scanHotSpan characters
// at a hot site looks for the record of its start (ScanReach): it moves to the recorded end and
// takes the count, and its next iteration fails there as it did when the walk was recorded, with
// the same expectation; otherwise it goes on and records itself when it ends (ScanEnd, ScanKids). In
// a Document parse a record also holds the end of the input that the rest of its walk examined, which
// a walk that takes the record examines too: decoding the last characters it skips may examine input
// past the character where it fails. A site turns hot, separately inside and outside lookaheads, once a walk
// of scanHotSpan characters or more starts below the furthest end of its walks and walks
// scanHotSpan characters below it (scanWalked), as a run site of the engine does: until then the
// walks of a site that never go back record nothing. The records of a parse last for that parse; a
// stream drops those before what it committed.
//
// Walks of scan sites do not nest (an iteration matches one character and calls nothing), so the
// state of the walk between ScanReach and its end is the parser's (scanWalk).

// scanHotSpan is the number of characters that a walk of a scan site matches before it looks for
// the record of its start, and the length of the walks that turn a site hot (flow.scanHotSpan).
const scanHotSpan = 16

// scanRecord is the record of a walk of a scan site: where it ended, how many characters it
// matched, in a Document parse the end of the input that the walk examined after ScanReach and, for a
// walk that builds a value, its elements (kids for a Node parse, tkids for a typed one).
type scanRecord struct {
	end, count, examined int
	kids                 []*core.Node
	tkids                []any
}

// scanCold is the state of a site that decides whether it is hot: the furthest end of the walks of
// scanHotSpan characters or more.
type scanCold struct {
	end int
	hot bool
}

// scanWalk is the state of the walk that ScanReach reached: whether its site was hot, whether it
// took a record, and the record; in a Document parse, for a walk that records itself, the examined
// end of the enclosing call before ScanReach (savedExamined).
type scanWalk struct {
	hot, hit      bool
	rec           scanRecord
	savedExamined int
}

// scanHot reports whether the site keeps records for the walk at the position.
func (p *Parser) scanHot(site int) bool {
	i := 2 * site
	if p.Silent > 0 {
		i++
	}
	return i < len(p.scanCold) && p.scanCold[i].hot
}

// ScanReach is called by a walk of the scan site that began at start when it has matched
// scanHotSpan characters, count of them. At a hot site with a record of start it moves to the
// recorded end and returns the recorded count, charging one step for the lookup and one for the
// record; otherwise it returns count, charging one step for the lookup at a hot site.
func (p *Parser) ScanReach(site, start, count int) int {
	w := &p.scanWalk
	w.hot, w.hit = p.scanHot(site), false
	if !w.hot {
		return count
	}
	p.Spend()
	var rec scanRecord
	ok := false
	if site < len(p.scanTables) {
		rec, ok = p.scanTables[site][start]
	}
	if !ok {
		if p.Document != nil {
			// The walk records itself: measure what the rest of it examines.
			w.savedExamined, p.docExamined = p.docExamined, p.Pos
		}
		return count
	}
	p.Spend()
	w.hit, w.rec = true, rec
	p.Pos = rec.end
	p.documentTouch(rec.examined)
	return rec.count
}

// scanExamined returns the end of the input that the rest of a walk that records itself examined
// (ScanReach), restoring that of the enclosing call, or 0 outside a Document parse.
func (p *Parser) scanExamined() int {
	if p.Document == nil {
		return 0
	}
	x := p.docExamined
	p.docExamined = max(p.scanWalk.savedExamined, x)
	return x
}

// ScanEnd ends a value-free walk of the scan site that began at start, matched count characters
// (at least scanHotSpan) and ended at the position: unless it took the rest from a record, it records
// itself if the site is hot and notes that it walked if not.
func (p *Parser) ScanEnd(site, start, count int) {
	w := &p.scanWalk
	switch {
	case w.hit:
	case w.hot:
		p.scanStore(site, start, scanRecord{end: p.Pos, count: count, examined: p.scanExamined()})
	default:
		p.scanWalked(site, start)
	}
}

// ScanKids returns the elements of a walk of the scan site that builds a value, which began at
// start, gathered its elements on the stack from base and matched count characters: for a walk of
// scanHotSpan characters or more that took the rest from a record, the elements of the record, which
// it shares (the gathered ones are dropped); otherwise the gathered elements, which a walk of that
// length records at a hot site.
func (p *Parser) ScanKids(site, start, base, count int) []*core.Node {
	if count < scanHotSpan {
		return p.Kids(base)
	}
	w := &p.scanWalk
	if w.hit {
		p.DropKids(base)
		return w.rec.kids
	}
	kids := p.Kids(base)
	if w.hot {
		for _, k := range kids {
			if k != nil {
				D(k).Fresh = false
			}
		}
		p.scanStore(site, start, scanRecord{end: p.Pos, count: count, examined: p.scanExamined(), kids: kids})
	} else {
		p.scanWalked(site, start)
	}
	return kids
}

// ScanKids is Parser.ScanKids for a typed parse.
func (p *TParser) ScanKids(site, start, base, count int) []any {
	if count < scanHotSpan {
		return p.Kids(base)
	}
	w := &p.scanWalk
	if w.hit {
		p.DropKids(base)
		return w.rec.tkids
	}
	kids := p.Kids(base)
	if w.hot {
		for _, k := range kids {
			if t := AsTval(k); t != nil {
				t.TSetFresh(false)
			}
		}
		p.scanStore(site, start, scanRecord{end: p.Pos, count: count, examined: p.scanExamined(), tkids: kids})
	} else {
		p.scanWalked(site, start)
	}
	return kids
}

// scanStore records the walk of the site that began at start, charging one step.
func (p *Parser) scanStore(site, start int, rec scanRecord) {
	p.Spend()
	if site >= len(p.scanTables) {
		p.scanTables = append(p.scanTables, make([]map[int]scanRecord, site+1-len(p.scanTables))...)
	}
	t := p.scanTables[site]
	if t == nil {
		t = map[int]scanRecord{}
		p.scanTables[site] = t
	}
	t[start] = rec
}

// scanWalked notes a walk of scanHotSpan characters or more of a site that is not hot, which began at
// start and ended at the position: the site turns hot when the walk began below the furthest end of
// the walks before it and walked scanHotSpan characters below that end.
func (p *Parser) scanWalked(site, start int) {
	i := 2 * site
	if p.Silent > 0 {
		i++
	}
	if i >= len(p.scanCold) {
		p.scanCold = append(p.scanCold, make([]scanCold, max(16, 2*(i+1))-len(p.scanCold))...)
	}
	c := &p.scanCold[i]
	if start < c.end && min(p.Pos, c.end)-start >= scanHotSpan {
		c.hot = true
	}
	c.end = max(c.end, p.Pos)
}

// pruneScanRecords drops the records of the starts before pos, where a stream that committed at
// pos never parses again.
func (p *Parser) pruneScanRecords(pos int) {
	for _, t := range p.scanTables {
		for start := range t {
			if start < pos {
				delete(t, start)
			}
		}
	}
}

// scanBytes returns the bytes that the records of scan sites take, for the memory limit.
func (p *Parser) scanBytes() uint64 {
	n := uint64(cap(p.scanTables))*ptrBytes + uint64(cap(p.scanCold))*uint64(unsafe.Sizeof(scanCold{}))
	for _, t := range p.scanTables {
		n += uint64(len(t)) * (intBytes + uint64(unsafe.Sizeof(scanRecord{})))
	}
	return n
}
