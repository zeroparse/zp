package state

import (
	"sync/atomic"
	"unicode/utf8"
	"unsafe"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/core/impl"
	"zeroparse.dev/zp/internal/value"
)

// Scan sites: the records of the walks of the single-character repetitions (compile/scan_sites.go;
// see "Scan Sites" in spec/runtime/parsing.md).
//
// An iteration of a scan site tests one character, so the walk from a position ends where the
// stretch of characters of the class ends, whatever position of the stretch it began at, and the rest
// of a walk from a position is a function of that position and the input. A site keeps runs: a run
// is a range [first, end) of positions that walks passed, all of which end at end. A run holds where
// its walks end, the input range they examined, and, for the walks that build a value, the elements
// in one sequence that grows toward its front as walks that begin further back extend the run.
//
// A walk tests its first ScanHotSpan characters as an ordinary repetition. At a hot site (RunHot) it
// then looks at its position before each further iteration (ScanReach, ScanTable.At, ScanTake): at
// an iteration start of a run, it takes the rest of the walk from the run; otherwise it goes on, and
// when it ends it records itself as a new run (ScanEnd), unless it passed a position of a run. A site
// is cold until its walks have walked the same input again, as a run site is (walked).

// ScanHotSpan is the number of characters that a walk of a scan site matches before it looks for a
// run, and the length of a walk that can turn the site hot (RunHotSpan for run sites). A smaller value
// lowers the cost of each call that a run answers and raises the cost of the walks that find none:
// the steps of a parse with records exceed those of a parse without by two for each walk of at least
// ScanHotSpan characters. It is a variable and exported only as a hook for tests in other packages, as
// RunHotSpan is.
var ScanHotSpan = 16

// ScanHits counts the walks answered from a run. It is a hook for tests that check a parse reaches the
// records; nothing reads it in a parse.
var ScanHits atomic.Int64

// ScanIntervalsOff, for the negative controls of the tests, lets a walk take a run only when it began
// at the first position of the run, and never extends a run: the records answer only the walks that
// begin where a recorded walk began, as records kept by the start of a walk would.
var ScanIntervalsOff = false

// scanReach bounds how far past the start of a character its test examines, in units: a valid
// character examines itself, and an invalid byte of a bytes-unit input the bytes up to the first that
// makes it invalid, at most utf8.UTFMax. Twice that is how far back from the end of a run a walk over
// bytes must have begun for the run's examined range to be its own (ScanTake): every iteration of the
// run before the walk's start, and every one that a walk which began inside a character makes before
// it meets the run's iteration starts, then examines less than the test at the end of the run. A walk
// of ScanHotSpan characters always has; a test of a code point examines that code point alone.
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
	// examined is the end of the input range that the walks recorded in the run examined, each from its
	// first character; count is the number of characters in [first, end).
	examined, count int
	// rem is nil when every character of the run takes one unit; otherwise rem[end-b-1] holds, for an
	// iteration start b, the number of characters from b to end, and 0 for the other positions.
	rem []int32
	// buf holds, for the walks that build a value, the elements of the run in buf[len(buf)-count:]. The
	// part before them is room that no list holds, which a walk that extends the run fills.
	buf []*core.Node
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

// suffix returns the elements of the last n characters of the run, as a list that no one may append to.
func (r *scanRun) suffix(n int) []*core.Node {
	l := len(r.buf)
	return r.buf[l-n : l : l]
}

// At returns the number of the run that covers pos, or 0. A run covers its positions and its end,
// where the character is not of the class, so no other run of the site covers it. It is safe on a nil
// table.
func (t *scanTable) At(pos int) int32 {
	if t == nil {
		return 0
	}
	if i := uint(pos - t.lo); i < uint(len(t.bits))*64 && t.bits[i/64]&(1<<(i%64)) != 0 {
		return t.find(pos)
	}
	return 0
}

// find returns the number of the run that covers pos, following the runs of the word of pos, or 0 if
// none does: a bit can stay set after a stream drops the run that set it (PruneScanRecords).
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

// ScanWalk is the state of a walk of a scan site that has matched ScanHotSpan characters.
type ScanWalk struct {
	// Tab is the table of the site, which the walk probes before each further iteration while it is
	// not nil (ScanTable.At); nil at a cold site, at a site without runs, and after the walk took a run.
	Tab *ScanTable
	// Kids are the elements of a walk that builds a value and took a run, which replace its own.
	Kids             []*core.Node
	site, start, hw0 int
	hot, hit, seen   bool
}

// ScanTable is the table of runs of a scan site in one value mode.
type ScanTable = scanTable

// scanTab returns the table of the site, or nil if it has none.
func (p *Parser) scanTab(site int) *scanTable {
	if site < len(p.scanTabs) {
		return p.scanTabs[site]
	}
	return nil
}

// ScanHot reports whether the site keeps records in this parse.
func (p *Parser) ScanHot(site int) bool {
	return !SkipRecordsOff && p.Missing == nil && p.RunHot(site)
}

// ScanBegin starts a walk of the scan site at the current position: it returns the high-water mark of
// the examined range, which the walk measures from its start, for ScanEnd or ScanShort.
func (p *Parser) ScanBegin() int {
	hw0 := p.Hw
	p.Hw = 0
	return hw0
}

// ScanShort ends a walk that matched fewer than ScanHotSpan characters.
func (p *Parser) ScanShort(hw0 int) { p.Hw = max(hw0, p.Hw) }

// ScanReach is called by a walk of the site that began at start when it has matched ScanHotSpan
// characters. At a hot site it charges one step for the lookup and returns the table to probe.
func (p *Parser) ScanReach(site, start, hw0 int) ScanWalk {
	w := ScanWalk{site: site, start: start, hw0: hw0}
	if w.hot = p.ScanHot(site); w.hot {
		p.Spend()
		w.Tab = p.scanTab(site)
	}
	return w
}

// ScanTakeKids is ScanTake for a walk whose elements, if it builds a value, are on the kid stack from
// base (base is -1 for a walk that builds none): a walk that takes the run drops them.
func (p *Parser) ScanTakeKids(w *ScanWalk, id int32, count, base int) int {
	if base < 0 {
		count, _ = p.ScanTake(w, id, count, nil, false)
		return count
	}
	count, hit := p.ScanTake(w, id, count, p.KidStack[base:], true)
	if hit {
		clear(p.KidStack[base:])
		p.KidStack = p.KidStack[:base]
	}
	return count
}

// ScanTake takes the rest of the walk w, which is at an iteration boundary at the current position
// covered by the run id, from the run, if the position is an iteration start of the run. count is the
// number of characters the walk has matched, and own, for a walk that builds a value (build), their
// elements, which the run may take but does not keep as a slice. It returns the count of the whole walk
// and whether it took the run: then the position is the end of the run, the examined range covers the
// run's, and a walk that builds a value holds its list in w.Kids, which replaces its own elements. A
// walk that began before the run extends the run back to its start.
func (p *Parser) ScanTake(w *ScanWalk, id int32, count int, own []*core.Node, build bool) (int, bool) {
	t := w.Tab
	r := &t.runs[id-1]
	b := p.Pos
	rb, ok := r.remAt(b)
	if !ok || p.Unit == core.Bytes && w.start > r.end-scanReach || ScanIntervalsOff && w.start != r.first {
		// Not an iteration start of the run, or, in bytes, too close to its end for the run's examined
		// range to be the walk's: walk on. The walk lies in the stretch of the run, so it records no run of its own.
		w.seen = true
		return count, false
	}
	p.Spend()
	ScanHits.Add(1)
	w.hit, w.Tab = true, nil
	if w.start < r.first && !ScanIntervalsOff {
		p.scanExtend(t, id, w.start, count-(r.count-rb), own, build)
	}
	if build {
		if rs, ok := r.remAt(w.start); ok {
			// The elements of the walk's own characters are those of the run at the same positions.
			w.Kids = r.suffix(rs)
		} else {
			// The walk began at a position that is no iteration start of the run, which a parse that
			// decodes the input from its start never reaches: its own elements and a copy of the rest.
			ks := p.Nodes(len(own) + rb)
			copy(ks, own)
			copy(ks[len(own):], r.suffix(rb))
			w.Kids = ks
		}
	}
	if SkipRecordMutation != SkipKeepPos {
		p.touch(r.examined)
		p.Pos = r.end
	}
	return count + rb, true
}

// scanExtend extends the run back to start, the start of the walk that takes it: before is the number
// of characters that the walk matched before the run's first position. It extends nothing if that
// position is no iteration start of the walk, which a parse that decodes from the start never meets.
// Extending charges nothing, as marking positions does not.
func (p *Parser) scanExtend(t *scanTable, id int32, start, before int, own []*core.Node, build bool) {
	r := &t.runs[id-1]
	first := r.first
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
	if build {
		own = own[:before]
		room := len(r.buf) - r.count
		if room < before {
			grown := make([]*core.Node, 2*(r.count+before))
			t.bufBytes += uint64(len(grown)) * ptrBytes
			copy(grown[len(grown)-r.count:], r.suffix(r.count))
			r.buf = grown
			room = len(grown) - r.count
		}
		for i, k := range own {
			// The run shares its elements with the lists of the walks that take it, so they are final.
			value.D(k).Fresh = false
			r.buf[room-before+i] = k
		}
	}
	r.first = start
	r.count += before
	r.examined = max(r.examined, p.Hw)
	t.extend(id, start, first)
}

// unitsAt returns the number of units of the character at i, which the parse has loaded, without
// charging the read: the walk that reads it again here charged its tests already.
func (p *Parser) unitsAt(i int) int {
	if p.Unit == core.CodePoints {
		return 1
	}
	var buf [utf8.UTFMax]byte
	var b []byte
	if DocumentPieceInput && p.Pieces != nil {
		n := min(utf8.UTFMax, p.Pieces.N-i)
		for j := 0; j < n; j++ {
			c, _ := p.Pieces.At(i + j)
			buf[j] = byte(c)
		}
		b = buf[:n]
	} else {
		b = p.Bs[i-p.Base:]
	}
	_, size := utf8.DecodeRune(b)
	return max(size, 1)
}

// ScanEnd ends a walk that matched ScanHotSpan characters or more, with count characters in all: unless
// it took the rest from a run, it records itself as a run if the site is hot and it probed no position
// of a run (a walk probes its end, which a run of its stretch covers), or notes that it walked if the site is not hot. kids are the elements of a walk that builds
// a value and did not take a run, or nil.
func (p *Parser) ScanEnd(w *ScanWalk, count int, kids []*core.Node) {
	if !w.hit {
		if !w.hot {
			p.walked(w.site, w.start, ScanHotSpan)
		} else if !w.seen {
			p.scanMake(w, count, kids)
		}
	}
	p.Hw = max(w.hw0, p.Hw)
}

// scanMake records the walk w, which ended at the current position after count characters, as a run,
// charging one step. It records only if the input the walk examined is available.
//
// The guard on the range is defensive and not what makes a run exact: Decode fills the input up to the
// character it examines (Fill) or reaches the end of the input (Eof) before it reports the range, so the
// range of a walk never reaches past the input loaded unless the input has ended. The guard stores
// nothing if that were not so, because more input would then change the walk.
func (p *Parser) scanMake(w *ScanWalk, count int, kids []*core.Node) {
	if SkipRecordMutation == SkipNeverRecord || !(p.Eof || p.Hw <= p.Loaded()) {
		return
	}
	p.Spend()
	r := scanRun{first: w.start, end: p.Pos, examined: p.Hw, count: count}
	if count != r.end-r.first {
		r.rem = make([]int32, r.end-r.first)
		for b, j := r.first, 0; b < r.end; b, j = b+p.unitsAt(b), j+1 {
			r.rem[r.end-b-1] = int32(count - j)
		}
	}
	if kids != nil {
		for _, k := range kids {
			// The lists of the walks that take the run share the elements, so they are final from here, as
			// those of run sites are (SkipValue).
			if k != nil {
				value.D(k).Fresh = false
			}
		}
		r.buf = kids[:len(kids):len(kids)]
	}
	if w.site >= len(p.scanTabs) {
		p.scanTabs = append(p.scanTabs, make([]*scanTable, w.site+1-len(p.scanTabs))...)
	}
	t := p.scanTabs[w.site]
	if t == nil {
		t = &scanTable{}
		p.scanTabs[w.site] = t
	}
	t.insert(r)
}

// ScanList ends a walk of a scan site that builds a value and matched ScanHotSpan characters or more:
// it returns the list of the elements, those of the run if the walk took the rest from one, and records
// the walk as ScanEnd does.
func (p *Parser) ScanList(w *ScanWalk, base, count int) *core.Node {
	kids := w.Kids
	if !w.hit {
		kids = p.Kids(base)
	}
	p.ScanEnd(w, count, kids)
	return p.NewNode(value.Data{Kind: coreimpl.ListKind, Start: int32(w.start), End: int32(p.Pos), Children: kids, Fresh: true})
}

// ScanRecords reports whether the walk w, which matched ScanHotSpan characters or more, records itself
// as a run when it ends: it took no run, the site is hot and it probed no position of a run.
func (p *Parser) ScanRecords(w *ScanWalk) bool { return !w.hit && w.hot && !w.seen }

// ScanFailList ends a walk of a scan site that builds a value, matched ScanHotSpan characters or more
// and falls short of the minimum, with its elements on the kid stack from base unless it took a run. A
// walk that records itself moves its elements into the run without the charge of taking them off as a
// list, as extending a run moves them, since it makes no list; any other walk drops them as a failed
// repetition does.
func (p *Parser) ScanFailList(w *ScanWalk, base, count int) {
	var kids []*core.Node
	if p.ScanRecords(w) {
		kids = p.Nodes(len(p.KidStack) - base)
		copy(kids, p.KidStack[base:])
		clear(p.KidStack[base:])
		p.KidStack = p.KidStack[:base]
	} else {
		p.DropKids(base)
	}
	p.ScanEnd(w, count, kids)
}

// PruneScanRecords drops the runs that end at or before pos and the positions before pos, where a
// stream that committed at pos never parses again.
func (p *Parser) PruneScanRecords(pos int) {
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

// scanBytes returns the bytes that the runs of scan sites hold.
func (p *Parser) scanBytes() uint64 {
	n := uint64(cap(p.scanTabs)) * ptrBytes
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
