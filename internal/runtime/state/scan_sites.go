package state

import (
	"sync/atomic"

	"zeroparse.dev/zp/internal/runtime/nodes"
	"zeroparse.dev/zp/pkg/runtime/value"
	"zeroparse.dev/zp/pkg/runtime/value/impl"
)

// Scan sites: the records of the walks of the single-character repetitions in pure rules
// (middleend/corepass/sites.go; see "Scan Sites" in spec/runtime/parsing.md).
//
// The walk of a scan site from a position ends at a position and examines input up to a position that
// depend on the position and the input alone. A scan site keeps at most one record for each walk, at
// the start of the walk: where it ended, the input range it examined and, for a walk that builds a
// value, the elements. It records nothing for the positions inside the walk. The site is cold, as a
// run site is (RunHot, RunWalked), until its walks have walked the same input again. A walk that
// reaches its ScanHotSpan-th character at a hot site looks for the record of its start: it takes the
// rest from the record, or walks on and records itself.

// ScanRecord is the record of a walk of a scan site.
type ScanRecord = skipRecord

// A record of a scan site holds the number of characters of the walk in far, which no scan site
// uses otherwise, and the elements of a walk that builds a value in vals.

// Count returns the number of elements of the walk.
func (r skipRecord) Count() int { return r.far }

// Kids returns the elements of a walk that builds a value. They are shared by the lists of the walks.
func (r skipRecord) Kids() []*value.Node { return r.vals.slice() }

// ScanHotSpan is the number of characters that a walk of a scan site matches before it looks for the
// record of its start, and the length of a walk that can turn the site hot (RunHotSpan for run
// sites). A smaller value lowers the cost of each call that a record answers and raises the cost of
// the walks that find none: the steps of a parse with records exceed those of a parse without by
// two for each walk of at least ScanHotSpan characters. It is a variable and exported only as a
// hook for tests in other packages, as RunHotSpan is.
var ScanHotSpan = 16

// ScanHits counts the walks answered from a record. It is a hook for tests that check a parse
// reaches the records; nothing reads it in a parse.
var ScanHits atomic.Int64

// scanSlots returns the slots of the site, or nil if it has none. A site has one table for the
// walks inside and outside lookaheads, unlike a skip-run site: the end, the examined range and the
// elements of a walk depend on the input and its start alone, and a lookahead only silences the
// expectations that the caller records after the walk (the walk reports none of its own into the
// record), so a walk in a lookahead and one outside it make the same record. Whether the site is hot
// is kept apart (RunHot), as for run sites.
func (p *Parser) scanSlots(site int) *skipSlots {
	if i := 2 * site; i < len(p.skipTables) {
		return p.skipTables[i]
	}
	return nil
}

// ScanHot reports whether the site keeps records in this parse.
func (p *Parser) ScanHot(site int) bool {
	return !SkipRecordsOff && p.Missing == nil && p.RunHot(site)
}

// ScanLookup returns the record of the walk of the hot site that began at start, if there is one.
func (p *Parser) ScanLookup(site, start int) (skipRecord, bool) {
	p.Spend()
	rec := p.scanSlots(site).get(start, nil)
	if rec.state != skipRecorded {
		return rec, false
	}
	p.Spend()
	if SkipRecordMutation != 0 {
		rec = mutateSkipRecord(rec, start)
	}
	return rec, true
}

// ScanJump moves the walk to the end of the record: the position of the end and the input range
// the rest of the walk examined. The caller runs the iteration that fails there.
func (p *Parser) ScanJump(rec skipRecord) {
	p.touch(rec.examined)
	p.Pos = rec.end
	ScanHits.Add(1)
}

// ScanStore records the walk of the hot site that began at start and ended at the current position;
// examined is the end of the input range it examined.
//
// The guard on the range is defensive and not what makes a record exact: Decode fills the input up to
// the character it examines (Fill) or reaches the end of the input (Eof) before it reports the
// range, so the range of a walk never reaches past the input loaded unless the input has ended. The
// guard stores nothing if that were not so, because more input would then change the walk. With the
// guard made to panic, the tests of the engine package, which include streams, pass; other packages
// were not run that way.
func (p *Parser) ScanStore(site, start, examined, count int, kids []*value.Node) {
	if SkipRecordMutation == SkipNeverRecord || !(p.Eof || examined <= p.Loaded()) {
		return
	}
	p.Spend()
	i := 2 * site
	s := p.scanSlots(site)
	if s == nil {
		if i >= len(p.skipTables) {
			p.skipTables = append(p.skipTables, make([]*skipSlots, i+1-len(p.skipTables))...)
		}
		s = &skipSlots{}
		p.skipTables[i] = s
	}
	rec := skipRecord{end: p.Pos, far: count, from: start, examined: examined, state: skipRecorded}
	for _, k := range kids {
		// The lists of the walks that take the record share the elements, so they are final from here, as
		// those of run sites are (SkipValue).
		if k != nil {
			nodes.D(k).Fresh = false
		}
	}
	if kids != nil {
		rec.vals = valSuffix{buf: &valBuf{a: kids}}
	}
	s.put(start, nil, rec)
}

// EndScanWalk ends a walk of a scan site that reached its ScanHotSpan-th iteration: unless it took
// the rest from a record, it records itself (with the elements, if it built a value) if the site is
// hot and notes that it walked if not. hw0 is the high-water mark before the walk reached the span.
func (p *Parser) EndScanWalk(site, start, count int, kids []*value.Node, hot, hit bool, hw0 int) {
	if !hit {
		if hot {
			p.ScanStore(site, start, p.Hw, count, kids)
		} else {
			p.walked(site, start, ScanHotSpan)
		}
	}
	p.Hw = max(hw0, p.Hw)
}

// FinishScan ends a walk of a scan site that builds no value and reached its ScanHotSpan-th iteration
// (see FinishScanList).
func (p *Parser) FinishScan(site, start, count int, hot, hit bool, hw0 int) {
	p.EndScanWalk(site, start, count, nil, hot, hit, hw0)
}

// FinishScanList ends a walk of a scan site that builds a value and reached its ScanHotSpan-th
// character: it returns the list of the elements, those of the record if the walk took the rest from
// it, and records the walk if the site is hot and the walk did not.
func (p *Parser) FinishScanList(site, start, base, count int, hot, hit bool, rec skipRecord, hw0 int) *value.Node {
	var kids []*value.Node
	if hit {
		clear(p.KidStack[base:])
		p.KidStack = p.KidStack[:base]
		kids = rec.Kids()
	} else {
		kids = p.Kids(base)
	}
	p.EndScanWalk(site, start, count, kids, hot, hit, hw0)
	return p.NewNode(nodes.Data{Kind: valueimpl.ListKind, Start: int32(start), End: int32(p.Pos), Children: kids, Fresh: true})
}

// flowScan is the walk of a scan site of a Flow body that has reached ScanHotSpan characters, from
// ScanReach until ScanEnd or ScanKids ends it: whether the site is hot, whether the walk took the
// rest from a record, the record, and the high-water mark before the walk reached the span. The body
// of a scan site is one character and calls no rule, so one walk is in progress at a time, as in the
// Go library.
type flowScan struct {
	hot, hit bool
	rec      skipRecord
	hw0      int
}

// ScanReach is flow.scanReach: for a walk of the scan site that began at start and has matched
// ScanHotSpan characters, count of them, at a hot site with a record of start it moves to the
// recorded end and returns the recorded count; otherwise it returns count. It charges what the match
// code charges at the same point (ScanLookup).
func (p *Parser) ScanReach(site, start, count int) int {
	w := &p.scan
	*w = flowScan{hot: p.ScanHot(site), hw0: p.Hw}
	p.Hw = 0
	if w.hot {
		if w.rec, w.hit = p.ScanLookup(site, start); w.hit {
			p.ScanJump(w.rec)
			return w.rec.Count()
		}
	}
	return count
}

// ScanEnd is flow.scanEnd: it ends a value-free walk of the scan site that began at start and
// matched count characters, at least ScanHotSpan, as FinishScan does.
func (p *Parser) ScanEnd(site, start, count int) {
	w := p.scan
	p.scan = flowScan{}
	p.FinishScan(site, start, count, w.hot, w.hit, w.hw0)
}

// ScanKids is flow.scanKids: it returns the elements of a walk of the scan site that began at start,
// gathered on the stack from base, which matched count characters. A walk of ScanHotSpan characters
// or more ends as FinishScanList ends it: the elements are those of the record it took, if it took
// one, which drops the gathered ones; otherwise the gathered ones, which it records at a hot site.
// Taking the gathered elements off charges as Kids does.
func (p *Parser) ScanKids(site, start, base, count int) []*value.Node {
	if count < ScanHotSpan {
		return p.Kids(base)
	}
	w := p.scan
	p.scan = flowScan{}
	var kids []*value.Node
	if w.hit {
		p.TruncateKids(base)
		kids = w.rec.Kids()
	} else {
		kids = p.Kids(base)
	}
	p.EndScanWalk(site, start, count, kids, w.hot, w.hit, w.hw0)
	return kids
}
