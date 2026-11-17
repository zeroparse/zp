package libgo

import (
	"unicode/utf8"
	"unsafe"

	"zeroparse.dev/zp/core"
)

// Scan sites: the records of the walks of the single-character repetitions without a maximum, a mirror
// of the engine's (internal/engine/state/scan_sites.go; see "Scan Sites" in spec/runtime/parsing.md and
// the proof in spec/proofs/run-sites.md).
//
// An iteration of a scan site tests one character, so the walk from a position ends where the stretch
// of characters of the class ends, whatever position of the stretch it began at, and the rest of a walk
// from a position is a function of that position and the input. A site keeps runs: a run is a range
// [first, end) of positions that walks passed, all of which end at end. A run holds where its walks
// end, the number of characters, in a Document parse the end of the input that its walks examined, and,
// for the walks that build a value, the elements in one sequence that grows toward its front as walks
// that begin further back extend the run.
//
// A walk tests its first scanHotSpan characters as an ordinary repetition. At a hot site it then looks
// at its position after each further character (ScanProbe): at an iteration start of a run, it takes
// the rest of the walk from the run, and its next iteration fails at the run's end as it did for the
// walks the run recorded, with the same expectation; otherwise it goes on, and when it ends it records
// itself as a new run (ScanEnd, ScanKids, ScanFail), unless it passed a position of a run. A site turns
// hot, separately inside and outside lookaheads, once a walk of scanHotSpan characters or more starts
// below the furthest end of its walks and walks scanHotSpan characters below it (scanWalked), as a run
// site of the engine does. The records of a parse last for that parse; a stream drops those before what
// it committed.
//
// Walks of scan sites do not nest (an iteration matches one character and calls nothing), so the state
// of a walk is the parser's (scanWalk).

// scanHotSpan is the number of characters that a walk of a scan site matches before it looks for a
// run, and the length of the walks that turn a site hot (flow.ScanHotSpan).
const scanHotSpan = 16

// scanReach bounds how far past the start of a character its test examines, in bytes; a walk over bytes
// takes a run only if it began at least this far before the run's end, so that the run's examined end
// is the walk's (the engine's scanReach).
const scanReach = 2 * utf8.UTFMax

// scanTable holds the runs of a site in one value mode, in the order they were made: a run's number is
// its index plus one, and a position belongs to at most one run, as the runs of a table lie in
// different stretches of characters of the class. bits holds a bit for each position from lo, set for
// the positions that a run covers, so that a probe of a position that no run covers reads one word.
// head holds, for each word of bits, the number of the first run in the order of positions that covers
// a position of the word, and each run links the next run in that order that covers a position of the
// word of its end (scanRun.next), so a probe of a covered position follows the runs of its word. A run
// covers at least ScanHotSpan positions and its end, so a word meets a bounded number of runs; making
// or extending a run changes the links of its own words alone and moves no other run, whatever the
// order in which the runs of a table are made.
type scanTable struct {
	lo   int
	bits []uint64
	head []int32
	runs []scanRun
	// bufBytes counts the sequences of elements that extending runs allocated; the first sequence of a
	// run is the list of the walk that made it, counted where it was allocated.
	bufBytes uint64
}

// scanRun is a run of a scan site: every walk from an iteration start in [first, end) ends at end.
type scanRun struct {
	first, end int
	// examined is the end of the input that the walks recorded in the run examined, each from its start,
	// in a Document parse; count is the number of characters in [first, end).
	examined, count int
	// rem is nil when every character of the run takes one unit; otherwise rem[end-b-1] holds, for an
	// iteration start b, the number of characters from b to end, and 0 for the other positions.
	rem []int32
	// buf (a Node parse) or tbuf (a typed parse) holds, for the walks that build a value, the elements of
	// the run at its end; the part before them is room that no list holds, which extending fills.
	buf  []*core.Node
	tbuf []any
	// next is the number of the next run in the order of positions if it covers a position of the word of
	// end, and 0 otherwise.
	next int32
}

// remAt returns the number of characters from b to the end of the run, and whether b is an iteration
// start of the run or its end.
func (r *scanRun) remAt(b int) (int, bool) {
	if b < r.first || b > r.end {
		return 0, false
	}
	if r.rem == nil || b == r.end {
		return r.end - b, true
	}
	n := int(r.rem[r.end-b-1])
	return n, n != 0
}

// at returns the number of the run that covers pos, or 0. A run covers its positions and its end,
// where the character is not of the class, so no other run of the site covers it. It is safe on a nil
// table.
func (t *scanTable) at(pos int) int32 {
	if t == nil {
		return 0
	}
	if i := uint(pos - t.lo); i < uint(len(t.bits))*64 && t.bits[i/64]&(1<<(i%64)) != 0 {
		return t.find(pos)
	}
	return 0
}

// find returns the number of the run that covers pos, following the runs of the word of pos, or 0 if
// none does: a bit can stay set after a stream drops the run that set it (pruneScanRecords).
func (t *scanTable) find(pos int) int32 {
	j := t.head[(pos-t.lo)/64]
	for j != 0 && t.runs[j-1].end < pos {
		j = t.runs[j-1].next
	}
	if j == 0 || t.runs[j-1].first > pos {
		return 0
	}
	return j
}

// grow grows the window to cover the positions of [from, to), doubling on the side that grows.
func (t *scanTable) grow(from, to int) {
	if len(t.bits) == 0 {
		t.lo = from &^ 63
		n := max((to-t.lo+63)/64, 2)
		t.bits, t.head = make([]uint64, n), make([]int32, n)
	} else if from < t.lo || to > t.lo+64*len(t.bits) {
		lo, hi := t.lo, t.lo+64*len(t.bits)
		if from < lo {
			lo = max(min(from, hi-2*64*len(t.bits)), 0) &^ 63
		}
		if to > hi {
			hi = max(to, lo+2*64*len(t.bits))
		}
		n := (hi - lo + 63) / 64
		bits, head := make([]uint64, n), make([]int32, n)
		copy(bits[(t.lo-lo)/64:], t.bits)
		copy(head[(t.lo-lo)/64:], t.head)
		t.lo, t.bits, t.head = lo, bits, head
	}
}

// cover marks the positions of [from, to) as covered by the run id, which covers them: it sets their
// bits and makes the run the head of each of their words that no run before it meets.
func (t *scanTable) cover(id int32, from, to int) {
	for i := uint(from - t.lo); i < uint(to-t.lo); i++ {
		t.bits[i/64] |= 1 << (i % 64)
	}
	t.heads(id, from, to)
}

// heads makes the run id the head of each word that the positions of [from, to) meet, unless a run
// before it in the order of positions is.
func (t *scanTable) heads(id int32, from, to int) {
	first := t.runs[id-1].first
	for w := (from - t.lo) / 64; w <= (to-1-t.lo)/64; w++ {
		if h := t.head[w]; h == 0 || t.runs[h-1].first > first {
			t.head[w] = id
		}
	}
}

// link makes the run before position first that ends in the word of first link the run id.
func (t *scanTable) link(id int32, first int) {
	var pred int32
	for j := t.head[(first-t.lo)/64]; j != 0 && t.runs[j-1].first < first; j = t.runs[j-1].next {
		pred = j
	}
	if pred != 0 {
		t.runs[pred-1].next = id
	}
}

// insert adds the run r, whose positions no run covers, to the table and returns its number.
func (t *scanTable) insert(r scanRun) int32 {
	t.grow(r.first, r.end+1)
	// The next run in the order of positions that meets the word of r's end.
	r.next = 0
	for j := t.head[(r.end-t.lo)/64]; j != 0; j = t.runs[j-1].next {
		if t.runs[j-1].first > r.end {
			r.next = j
			break
		}
	}
	id := int32(len(t.runs) + 1)
	t.link(id, r.first)
	t.runs = append(t.runs, r)
	t.cover(id, r.first, r.end+1)
	return id
}

// extend covers the positions of [start, first) by the run id, whose first position moved back from
// first to start over positions that no run covered.
func (t *scanTable) extend(id int32, start, first int) {
	t.grow(start, first)
	t.link(id, start)
	t.cover(id, start, first)
}

// scanCold is the state of a site that decides whether it is hot: the furthest end of the walks of
// scanHotSpan characters or more.
type scanCold struct {
	end int
	hot bool
}

// scanWalk is the state of the walk of a scan site: from its scanHotSpan-th character, the table it
// probes (nil at a cold site, at a site without runs and after it took a run), whether the site is hot,
// whether the walk took a run (then kids or tkids are its elements for a walk that builds a value) and
// whether it probed a position of a run that it could not take; in a Document parse, the examined end
// of the enclosing call before the walk began (ScanBegin).
type scanWalk struct {
	tab            *scanTable
	kids           []*core.Node
	tkids          []any
	start          int
	hot, hit, seen bool
	savedExamined  int
}

// scanHot reports whether the site keeps records for the walk at the position.
func (p *Parser) scanHot(site int) bool {
	i := 2 * site
	if p.Silent > 0 {
		i++
	}
	return i < len(p.scanCold) && p.scanCold[i].hot
}

// ScanBegin begins a walk of a scan site at the position: a Document parse measures the end of the
// input that the walk examines from its start.
func (p *Parser) ScanBegin() {
	if p.track {
		p.scanWalk.savedExamined, p.docExamined = p.docExamined, p.Pos
	}
}

// ScanProbe is called by a walk of the value-free scan site that began at start after each character
// from its scanHotSpan-th on, count of them. At the scanHotSpan-th it charges one step for the lookup at
// a hot site. At an iteration start of a run it takes the rest of the walk from the run, charging one
// step: it moves to the run's end and returns the count of the whole walk; otherwise it returns count.
// It is small enough to inline: past the scanHotSpan-th character, a walk that has no table to probe
// pays one test.
func (p *Parser) ScanProbe(site, start, count int) int {
	if count == scanHotSpan || p.scanWalk.tab != nil {
		return p.scanProbe(site, start, count)
	}
	return count
}

func (p *Parser) scanProbe(site, start, count int) int {
	if count == scanHotSpan {
		p.scanReach(site, start)
	}
	if id := p.scanWalk.tab.at(p.Pos); id != 0 {
		return p.scanTake(id, count, nil, nil)
	}
	return count
}

// ScanProbeKids is ScanProbe for a walk of a scan site that builds a value, whose elements are on the
// stack from base: a walk that takes a run drops them for the run's (ScanKids).
func (p *Parser) ScanProbeKids(site, start, base, count int) int {
	if count == scanHotSpan || p.scanWalk.tab != nil {
		return p.scanProbeKids(site, start, base, count)
	}
	return count
}

func (p *Parser) scanProbeKids(site, start, base, count int) int {
	if count == scanHotSpan {
		p.scanReach(site, start)
	}
	if id := p.scanWalk.tab.at(p.Pos); id != 0 {
		count = p.scanTake(id, count, p.KidStack[base:], nil)
		if p.scanWalk.hit {
			p.DropKids(base)
		}
	}
	return count
}

// ScanProbeKids is Parser.ScanProbeKids for a typed parse.
func (p *TParser) ScanProbeKids(site, start, base, count int) int {
	if count == scanHotSpan || p.scanWalk.tab != nil {
		return p.scanProbeKids(site, start, base, count)
	}
	return count
}

func (p *TParser) scanProbeKids(site, start, base, count int) int {
	if count == scanHotSpan {
		p.scanReach(site, start)
	}
	if id := p.scanWalk.tab.at(p.Pos); id != 0 {
		count = p.scanTake(id, count, nil, p.KidStack[base:])
		if p.scanWalk.hit {
			p.DropKids(base)
		}
	}
	return count
}

// scanReach starts the probes of the walk of the site that began at start, at its scanHotSpan-th
// character.
func (p *Parser) scanReach(site, start int) {
	w := &p.scanWalk
	w.tab, w.kids, w.tkids, w.start = nil, nil, nil, start
	w.hit, w.seen = false, false
	if w.hot = p.scanHot(site); w.hot {
		p.Spend()
		if site < len(p.scanTabs) {
			w.tab = p.scanTabs[site]
		}
	}
}

// scanTake takes the rest of the walk, which is at an iteration boundary at the position covered by the
// run id, from the run, if the position is an iteration start of the run. count is the number of
// characters the walk has matched, and own or town, for a walk that builds a value, their elements. A
// walk that began before the run extends the run back to its start.
func (p *Parser) scanTake(id int32, count int, own []*core.Node, town []any) int {
	w := &p.scanWalk
	t := w.tab
	r := &t.runs[id-1]
	rb, ok := r.remAt(p.Pos)
	if !ok || p.unit == core.Bytes && w.start > r.end-scanReach {
		// Not an iteration start of the run, or, in bytes, too close to its end for the run's examined
		// end to be the walk's: walk on. The walk lies in the stretch of the run, so it makes no run.
		w.seen = true
		return count
	}
	p.Spend()
	w.hit, w.tab = true, nil
	if w.start < r.first {
		p.scanExtend(t, id, count-(r.count-rb), own, town)
	}
	if own != nil {
		if rs, ok := r.remAt(w.start); ok {
			// The elements of the walk's own characters are those of the run at the same positions.
			w.kids = scanSuffix(r.buf, rs)
		} else {
			// The walk began at a position that is no iteration start of the run, which a parse that
			// decodes the input from its start never reaches: its own elements and a copy of the rest.
			ks := p.Nodes(len(own) + rb)
			copy(ks, own)
			copy(ks[len(own):], scanSuffix(r.buf, rb))
			w.kids = ks
		}
	} else if town != nil {
		if rs, ok := r.remAt(w.start); ok {
			w.tkids = scanSuffix(r.tbuf, rs)
		} else {
			ks := make([]any, len(town)+rb)
			copy(ks, town)
			copy(ks[len(town):], scanSuffix(r.tbuf, rb))
			w.tkids = ks
		}
	}
	p.documentTouch(r.examined)
	p.Pos = r.end
	return count + rb
}

// scanSuffix returns the last n elements of a run's sequence, as a list that no one may append to.
func scanSuffix[E any](buf []E, n int) []E {
	l := len(buf)
	return buf[l-n : l : l]
}

// scanPrepend puts the elements own before the count elements at the end of buf, growing buf if its
// room is short, and returns it with the bytes it allocated.
func scanPrepend[E any](buf []E, count int, own []E, size uint64) ([]E, uint64) {
	var grew uint64
	room := len(buf) - count
	if room < len(own) {
		grown := make([]E, 2*(count+len(own)))
		grew = uint64(len(grown)) * size
		copy(grown[len(grown)-count:], buf[len(buf)-count:])
		buf = grown
		room = len(grown) - count
	}
	copy(buf[room-len(own):room], own)
	return buf, grew
}

// scanExtend extends the run back to the start of the walk that takes it: before is the number of
// characters that the walk matched before the run's first position. It extends nothing if that position
// is no iteration start of the walk, which a parse that decodes from the start never meets. Extending
// charges nothing, as marking positions does not.
func (p *Parser) scanExtend(t *scanTable, id int32, before int, own []*core.Node, town []any) {
	r := &t.runs[id-1]
	start, first := p.scanWalk.start, r.first
	if before != first-start || r.rem != nil {
		// Characters of several units: note the iteration starts of the walk before first.
		starts := make([]int, 0, before)
		for b := start; b < first; b += p.unitsAt(b) {
			starts = append(starts, b)
		}
		if len(starts) != before {
			return
		}
		if r.rem == nil {
			r.rem = make([]int32, r.end-first, 2*(r.end-start))
			for b := first; b < r.end; b++ {
				r.rem[r.end-b-1] = int32(r.end - b)
			}
		}
		r.rem = append(r.rem, make([]int32, first-start)...)
		for j, b := range starts {
			r.rem[r.end-b-1] = int32(r.count + before - j)
		}
	}
	var grew uint64
	if own != nil {
		own = own[:before]
		for _, k := range own {
			// The run shares its elements with the lists of the walks that take it, so they are final.
			if k != nil {
				D(k).Fresh = false
			}
		}
		r.buf, grew = scanPrepend(r.buf, r.count, own, ptrBytes)
	} else if town != nil {
		town = town[:before]
		for _, k := range town {
			if v := AsTval(k); v != nil {
				v.TSetFresh(false)
			}
		}
		r.tbuf, grew = scanPrepend(r.tbuf, r.count, town, uint64(unsafe.Sizeof(any(nil))))
	}
	t.bufBytes += grew
	r.first = start
	r.count += before
	if p.track {
		r.examined = max(r.examined, p.docExamined)
	}
	t.extend(id, start, first)
}

// unitsAt returns the number of units of the character at i, which the parse has read, without charging
// the read: the walk that reads it again here charged its tests already.
func (p *Parser) unitsAt(i int) int {
	if p.unit == core.CodePoints {
		return 1
	}
	var size int
	if s := p.Stream; s != nil {
		_, size = utf8.DecodeRune(s.bs[i-s.base:])
	} else {
		_, size = utf8.DecodeRuneInString(p.bs[i:])
	}
	return max(size, 1)
}

// ScanEnd ends a value-free walk of the scan site that began at start and matched count characters:
// for a walk of scanHotSpan characters or more, unless it took the rest from a run, it records itself
// as a run if the site is hot and it probed no position of a run (a walk probes its end, which a run of
// its stretch covers), or notes that it walked if the site is not hot.
func (p *Parser) ScanEnd(site, start, count int) {
	if count >= scanHotSpan || p.track {
		p.scanEnd(site, start, count, nil, nil)
	}
}

// scanEnd is ScanEnd for a walk whose elements, if it builds a value and took no run, are kids or tkids.
func (p *Parser) scanEnd(site, start, count int, kids []*core.Node, tkids []any) {
	if count >= scanHotSpan {
		w := &p.scanWalk
		switch {
		case w.hit:
		case !w.hot:
			p.scanWalked(site, start)
		case !w.seen:
			p.scanMake(site, start, count, kids, tkids)
		}
	}
	if p.track {
		p.docExamined = max(p.scanWalk.savedExamined, p.docExamined)
	}
}

// ScanKids is Kids for a walk of the scan site that builds a value, which began at start, gathered its
// elements on the stack from base and matched count characters: it ends the walk as ScanEnd does, and
// returns the elements of the run the walk took, if it took one, or the gathered elements, which a run
// that the walk makes shares.
func (p *Parser) ScanKids(site, start, base, count int) []*core.Node {
	kids := p.scanWalk.kids
	if count < scanHotSpan || !p.scanWalk.hit {
		kids = p.Kids(base)
	}
	p.scanEnd(site, start, count, kids, nil)
	return kids
}

// ScanKids is Parser.ScanKids for a typed parse.
func (p *TParser) ScanKids(site, start, base, count int) []any {
	kids := p.scanWalk.tkids
	if count < scanHotSpan || !p.scanWalk.hit {
		kids = p.Kids(base)
	}
	p.scanEnd(site, start, count, nil, kids)
	return kids
}

// ScanFail ends a walk of the scan site that builds a value, which began at start, gathered its elements
// on the stack from base and matched count characters, fewer than the minimum of the repetition. A walk
// that records itself moves its elements into the run without the charge of taking them off as a list,
// as extending a run moves them, since it makes no list (the engine's ScanFailList); any other walk drops
// them.
func (p *Parser) ScanFail(site, start, base, count int) {
	var kids []*core.Node
	if w := &p.scanWalk; count >= scanHotSpan && !w.hit && w.hot && !w.seen {
		kids = p.Nodes(len(p.KidStack) - base)
		copy(kids, p.KidStack[base:])
	}
	p.DropKids(base)
	p.scanEnd(site, start, count, kids, nil)
}

// ScanFail is Parser.ScanFail for a typed parse.
func (p *TParser) ScanFail(site, start, base, count int) {
	var kids []any
	if w := &p.scanWalk; count >= scanHotSpan && !w.hit && w.hot && !w.seen {
		kids = p.NewVals(len(p.KidStack) - base)
		copy(kids, p.KidStack[base:])
	}
	p.DropKids(base)
	p.scanEnd(site, start, count, nil, kids)
}

// scanMake records the walk of the site that began at start, ended at the position and matched count
// characters as a run, charging one step.
func (p *Parser) scanMake(site, start, count int, kids []*core.Node, tkids []any) {
	p.Spend()
	r := scanRun{first: start, end: p.Pos, count: count}
	if p.track {
		r.examined = p.docExamined
	}
	if count != r.end-r.first {
		r.rem = make([]int32, r.end-r.first)
		for b, j := r.first, 0; b < r.end; b, j = b+p.unitsAt(b), j+1 {
			r.rem[r.end-b-1] = int32(count - j)
		}
	}
	// The lists of the walks that take the run share the elements, so they are final from here.
	if kids != nil {
		for _, k := range kids {
			if k != nil {
				D(k).Fresh = false
			}
		}
		r.buf = kids[:len(kids):len(kids)]
	}
	if tkids != nil {
		for _, k := range tkids {
			if v := AsTval(k); v != nil {
				v.TSetFresh(false)
			}
		}
		r.tbuf = tkids[:len(tkids):len(tkids)]
	}
	if site >= len(p.scanTabs) {
		p.scanTabs = append(p.scanTabs, make([]*scanTable, site+1-len(p.scanTabs))...)
	}
	t := p.scanTabs[site]
	if t == nil {
		t = &scanTable{}
		p.scanTabs[site] = t
	}
	t.insert(r)
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

// pruneScanRecords drops the runs that end at or before pos and the positions before pos, where a
// stream that committed at pos never parses again.
func (p *Parser) pruneScanRecords(pos int) {
	for _, t := range p.scanTabs {
		if t == nil || len(t.bits) == 0 || pos <= t.lo {
			continue
		}
		cut := min((pos-t.lo)/64, len(t.bits))
		if 2*cut < len(t.bits) {
			continue
		}
		// Keep the runs that reach past pos, in the order they were made, and the window from the word of
		// pos. A kept run links only runs after it, which are kept too; the heads are those of the kept runs.
		num := make([]int32, len(t.runs)+1)
		runs := t.runs[:0:0]
		for i, r := range t.runs {
			if r.end > pos {
				runs = append(runs, r)
				num[i+1] = int32(len(runs))
			}
		}
		for i := range runs {
			runs[i].next = num[runs[i].next]
		}
		t.lo, t.bits, t.runs = t.lo+64*cut, append([]uint64(nil), t.bits[cut:]...), runs
		t.head = make([]int32, len(t.bits))
		for i, r := range runs {
			t.heads(int32(i+1), max(r.first, t.lo), r.end+1)
		}
	}
}

// scanBytes returns the bytes that the runs of scan sites hold, for the memory limit.
func (p *Parser) scanBytes() uint64 {
	n := uint64(cap(p.scanTabs))*ptrBytes + uint64(cap(p.scanCold))*uint64(unsafe.Sizeof(scanCold{}))
	for _, t := range p.scanTabs {
		if t == nil {
			continue
		}
		n += uint64(unsafe.Sizeof(scanTable{})) + uint64(cap(t.bits))*8 + uint64(cap(t.head))*4 + uint64(cap(t.runs))*uint64(unsafe.Sizeof(scanRun{})) + t.bufBytes
		for i := range t.runs {
			n += uint64(cap(t.runs[i].rem)) * 4
		}
	}
	return n
}
