package libgo

import (
	"zeroparse.dev/zp/core"
)

// Skip runs and run sites: the records of the walks of skip-run sites and run sites, a mirror of
// the engine's (internal/engine/state/skip_runs.go; see "Skip Runs" and "Run Sites" in
// spec/runtime/parsing.md and the proofs in spec/proofs/skip-runs.md and spec/proofs/run-sites.md).
//
// A walk of a site repetition visits positions r0 < r1 < ... with one iteration each. The slot of
// a position in the table of the site (and lookahead state) is fresh, visited or recorded. A walk
// starts plain: it marks each position it leaves after a matching iteration as visited and
// records nothing, so a walk that fails at once touches no slot. At its first
// iteration at a position that is not fresh, which an earlier walk of the site visited, it starts
// recording: from there on, for every position it visits, it records what the rest of the walk
// from there did (where it ended and in which environment, its farthest failure and the
// expectations there, the elements it collected, the errors it recovered and, in a Document
// parse, the input it examined). A recording walk that reaches a recorded position (once it has
// matched its minimum) takes the rest from the record: it moves to the end, records the
// expectations again and extends the examined range, as a memo hit does. The record of a position
// combines the iteration there with the record of the next position, so a walk fills the records
// of its recording visits backward when it ends: at a failed or empty iteration, whose record ends
// at its own start, or at a recorded position.
//
// A run site walks as an ordinary repetition until its walks overlap (RunHot, RunWalked); the
// walks of a hot site are the walks above. The records of a parse last for that parse; a stream
// drops those before what it committed.

// The states of a slot.
const (
	skipFresh    = iota // no walk visited the position
	skipVisited         // a walk visited it without recording
	skipRecorded        // the record holds the rest of a walk from it
)

// skipRecord is the state of a position and, once recorded, what the rest of a walk from it did:
// where it ended and the values it left in the variables that the element binds (bind, in the
// order of the site's run key; none for a site not keyed by the variables), its farthest failure
// and the expectations there, the input it examined in a Document parse ([from, examined)), the
// elements it collected (a run site that builds values; nodes in a Node parse, tvals in a typed
// one) and the errors it recovered. list reports that a walk that builds values made the record:
// the value-free twin of a rule walks the same site without elements, and a walk that builds
// values takes no record of such a walk, but records the position again.
type skipRecord struct {
	end, far       int
	exp            []ExpID
	from, examined int
	state          uint8
	list           bool
	// key holds the values of the variables of the key that the walk from the position started with.
	key, bind []any
	nodes     valSuffix[*core.Node]
	tvals     valSuffix[any]
	errs      *recEntry // nil for none, so that a record of no errors stays small
}

// skipKey indexes the records: the position and the hash of the values of the variables of the
// key that the walk from the position started with, zero for a site not keyed by them
// (skipWalk.key). Records whose keys share a hash are told apart by their values (skipSlots.more).
type skipKey struct {
	pos int
	h   uint64
}

// walkKey is the key of a visit: the values of the variables of the key, and their hash.
type walkKey struct {
	vals []any
	h    uint64
}

// RunKey holds the slots of the variables that key the records of a run site that is not free of
// the environment: Vars, those that its element reads or binds and those that the rules it calls
// read, and Bind, those that it binds, which a walk that takes a record sets to the values the walk
// of the record left in them.
type RunKey struct {
	Vars, Bind []int
}

// valBuf holds the elements that walks collected. A record holds a suffix of a buffer, which is
// never written once a record refers to it, except below lo: a walk that reaches a record whose
// suffix starts at lo puts its own elements before it, and shares the rest.
type valBuf[T any] struct {
	a  []T
	lo int
}

// valSuffix is the elements buf.a[i:], none if buf is nil.
type valSuffix[T any] struct {
	buf *valBuf[T]
	i   int
}

// skipSlots holds the slots of a site: a bit for each position from base on, set when the slot is
// not fresh, and the records of the recorded slots by position. The bits take an eighth of a byte
// per position between the first and the last position walked, and a mark costs no allocation once
// they cover the position; the records take memory only for the positions recording walks visited.
// The slots of the positions before floor are dropped: a stream parses there no more.
type skipSlots struct {
	base  int
	marks []uint64
	recs  map[skipKey]skipRecord
	// more holds the records whose key shares the hash of the one in recs but not its values.
	more  map[skipKey][]skipRecord
	floor int
	// kept is the number of records after dropped ones were last removed.
	kept int
	// expBytes counts the expectations of the records stored, for the memory limit.
	expBytes uint64
}

// marked reports whether the slot of pos is not fresh. s may be nil, with every slot fresh.
func (s *skipSlots) marked(pos int) bool {
	if s == nil || pos < s.floor {
		return false
	}
	i := pos - s.base
	return i >= 0 && i>>6 < len(s.marks) && s.marks[i>>6]&(1<<(i&63)) != 0
}

// getSkip returns the slot of pos for walks that start with the key k; a fresh slot is the zero
// record. Comparing the values of the key charges as comparing those of a memo key does.
func (p *Parser) getSkip(s *skipSlots, pos int, k walkKey) skipRecord {
	if !s.marked(pos) {
		return skipRecord{}
	}
	sk := skipKey{pos, k.h}
	if r, ok := s.recs[sk]; ok {
		if len(k.vals) == 0 || p.memoKeyEqual(r.key, k.vals) {
			return r
		}
		for _, r := range s.more[sk] {
			if p.memoKeyEqual(r.key, k.vals) {
				return r
			}
		}
	}
	return skipRecord{state: skipVisited}
}

// putSkip sets the slot of pos for walks that start with the key k to r, which is visited or
// recorded; a record without values leaves one with values in place. Room made for bits before
// base is as large as the bits, so walks that start farther and farther back copy them a
// logarithmic number of times.
func (p *Parser) putSkip(s *skipSlots, pos int, k walkKey, r skipRecord) {
	switch {
	case len(s.marks) == 0:
		s.base = pos &^ 63
	case pos < s.base:
		base := max(0, min(pos, s.base-64*len(s.marks))) &^ 63
		marks := make([]uint64, (s.base-base)>>6+len(s.marks))
		copy(marks[(s.base-base)>>6:], s.marks)
		s.marks, s.base = marks, base
	}
	i := pos - s.base
	if i>>6 >= len(s.marks) {
		s.marks = append(s.marks, make([]uint64, i>>6+1-len(s.marks))...)
	}
	s.marks[i>>6] |= 1 << (i & 63)
	if r.state == skipRecorded {
		if s.recs == nil {
			s.recs = make(map[skipKey]skipRecord)
		}
		sk := skipKey{pos, k.h}
		r.key = k.vals
		old, ok := s.recs[sk]
		if ok && len(k.vals) > 0 && !p.memoKeyEqual(old.key, k.vals) {
			// Another key with the same hash holds the slot: the record goes with the others.
			p.putMore(s, sk, r)
			return
		}
		if ok && old.list && !r.list {
			// A record with values serves the walks without values too, and holds what this
			// one holds besides them, so it stays: a key holds a record without values only
			// until a walk with values records it, and never after.
			return
		}
		s.recs[sk] = r
		s.expBytes += uint64(len(r.exp)) * expIDBytes
	}
}

// putMore stores r in the records of sk whose key shares the hash of the one in recs, keeping a
// record with values over one without, as putSkip does.
func (p *Parser) putMore(s *skipSlots, sk skipKey, r skipRecord) {
	if s.more == nil {
		s.more = map[skipKey][]skipRecord{}
	}
	rs := s.more[sk]
	for i, old := range rs {
		if p.memoKeyEqual(old.key, r.key) {
			if !old.list || r.list {
				rs[i] = r
			}
			return
		}
	}
	s.more[sk] = append(rs, r)
	s.expBytes += uint64(len(r.exp)) * expIDBytes
}

// drop drops the slots of the positions before pos: the words of bits wholly before it at once, and
// the records once they can make up half of them, so its cost is proportional to the records stored.
func (s *skipSlots) drop(pos int) {
	if pos <= s.floor {
		return
	}
	s.floor = pos
	if k := min((pos-s.base)>>6, len(s.marks)); k > 0 {
		s.marks, s.base = s.marks[k:], s.base+64*k
	}
	if len(s.recs) < 2*s.kept+64 {
		return
	}
	for k := range s.recs {
		if k.pos < pos {
			delete(s.recs, k)
		}
	}
	for k := range s.more {
		if k.pos < pos {
			delete(s.more, k)
		}
	}
	s.kept = len(s.recs)
}

// skipVisit is a recording visit of the walk in progress.
type skipVisit struct {
	pos, far       int
	exp            []ExpID
	from, examined int
	// key is the key the iteration started with (none for a site not keyed by the variables), and
	// vi the number of elements collected before it. e0 is the number of entries of the record of
	// recovered errors before it, and errs the errors it recovered.
	key    walkKey
	vi, e0 int
	errs   recEntry
}

// walkVals are the elements that a walk that builds values collected (vals), whose list the end of
// the walk makes. placed reports that the list is placed in buf from lo on, so that the records
// share it.
type walkVals[T any] struct {
	vals   []T
	placed bool
	buf    *valBuf[T]
	lo     int
}

// skipWalk is a walk in progress. Walks nest: the element of a run site can call rules that walk
// sites of their own, so the walks in progress form a stack (Parser.skips), and the innermost
// one is the one in use.
type skipWalk struct {
	// key holds the slots of the variables that key the records of the site (RunKey), nil for a
	// site whose element reads or binds no variable, directly or in the rules it calls: its records
	// hold for every environment, which a walk keeps. A walk that reuses the record of a keyed site
	// binds the variables of the key's Bind to the values the walk of the record left in them.
	key *RunKey
	// recording reports that the walk records its visits (it met a position that is not fresh).
	recording bool
	// table is the index of the slots in skipTables, and slots the slots, nil until a walk of the
	// site first writes one.
	table int
	slots *skipSlots
	// prev is the position of the last iteration of a plain walk, if prevOK; the walk marks it
	// when the iteration has matched.
	prev   int
	prevOK bool
	visits []skipVisit
	// open reports that the last visit's iteration is running: its expectations are isolated
	// from mk, and the examined range from before it is savedLow and savedExamined.
	open                    bool
	mk                      ExpMark
	savedLow, savedExamined int
	hit                     skipRecord
	hitOK                   bool
	// build reports that the walk collects the values of its iterations: nodes in a Node parse,
	// tvals in a typed one (typed).
	build, typed bool
	nodes        walkVals[*core.Node]
	tvals        walkVals[any]
}

// walk returns the walk in use, the innermost one in progress.
func (p *Parser) walk() *skipWalk { return &p.skips[p.skipN-1] }

// RunHotSpan is the length of input that a walk of a run site may walk again, below the furthest
// point that the walks before it reached, before the site turns hot: the engine's. A walk shorter
// than that neither turns a site hot nor can walk that far again, so generated code passes only the
// longer ones to RunWalked. It is a variable only as a hook for tests, which lower it to run the hot
// path on short inputs; nothing else writes it.
var RunHotSpan = 64

// runCold is the state of a run site (and lookahead state) before it keeps records: the furthest
// position that its walks passed, among the walks that RunWalked was told of. A site turns hot when
// a walk begins below that position and walks RunHotSpan or more of the input below it. A walk of a
// site that is not hot costs its length, and the part of it below the furthest position is under
// RunHotSpan or the site turns hot, so the walks of a cold site cost in total at most the input
// length plus RunHotSpan for each walk. A run site whose walks never go back allocates nothing.
type runCold struct {
	end int
	hot bool
}

// RunHot reports whether the walks of the run site keep records. A walk of a site that is not hot
// is an ordinary repetition, whose end the generated code passes to RunWalked.
func (p *Parser) RunHot(site int) bool {
	i := 2 * site
	if p.Silent > 0 {
		i++
	}
	return i < len(p.runCold) && p.runCold[i].hot
}

// RunWalked ends the walk of a run site that was not hot, which began at start and ended at the
// current position. A walk that began below the furthest position that the walks of the site passed
// walks input again, and the site turns hot when it walks RunHotSpan of it below that position.
func (p *Parser) RunWalked(site, start int) {
	i := 2 * site
	if p.Silent > 0 {
		i++
	}
	if i >= len(p.runCold) {
		// One table for all the sites: grown to twice the index, it is made once for most programs.
		grown := make([]runCold, max(16, 2*(i+1)))
		copy(grown, p.runCold)
		p.runCold = grown
	}
	c := &p.runCold[i]
	if start < c.end && min(p.Pos, c.end)-start >= RunHotSpan {
		c.hot = true
	}
	c.end = max(c.end, p.Pos)
}

// SkipBegin starts a walk of the skip-run site at the current position.
func (p *Parser) SkipBegin(site int) { p.skipBegin(site, false, false) }

// RunBegin starts a walk of the hot run site at the current position. build reports that its
// iterations have values (SkipValue), which SkipKids gathers in a list, and keyEnv that its records
// are keyed by the values of the variables of the site's run key (Grammar.SetRunKey).
func (p *Parser) RunBegin(site int, build, keyEnv bool) { p.skipBegin(site, build, keyEnv) }

// RunBegin is the Parser's RunBegin in a typed parse, whose values SkipValue and SkipKids take.
func (p *TParser) RunBegin(site int, build, keyEnv bool) {
	p.skipBegin(site, build, keyEnv)
	p.walk().typed = true
}

func (p *Parser) skipBegin(site int, build, keyEnv bool) {
	if p.skipN == len(p.skips) {
		p.skips = append(p.skips, skipWalk{})
	}
	p.skipN++
	w := p.walk()
	i := 2 * site
	if p.Silent > 0 {
		i++
	}
	w.table, w.slots = i, nil
	if i < len(p.skipTables) {
		w.slots = p.skipTables[i]
	}
	w.key, w.build, w.typed = nil, build, false
	if keyEnv {
		w.key = p.g.runKey(site)
	}
	w.nodes.placed, w.tvals.placed = false, false
	w.visits, w.recording, w.prevOK, w.open, w.hitOK = w.visits[:0], false, false, false, false
}

// SkipValue adds the value of the iteration that matched. Nodes in records are shared by the lists
// that reuse them, so a node that the rule that made it may still change is final from here.
func (p *Parser) SkipValue(v *core.Node) {
	w := p.walk()
	if v != nil {
		D(v).Fresh = false
	}
	w.nodes.vals = append(w.nodes.vals, v)
}

// SkipValue is the Parser's SkipValue for a typed value.
func (p *TParser) SkipValue(v any) {
	w := p.walk()
	if n := AsTval(v); n != nil {
		n.TSetFresh(false)
	}
	w.tvals.vals = append(w.tvals.vals, v)
}

// skipTable returns the slots of the walk, making them.
func (p *Parser) skipTable() *skipSlots {
	w := p.walk()
	if w.slots == nil {
		if w.table >= len(p.skipTables) {
			p.skipTables = append(p.skipTables, make([]*skipSlots, w.table+1-len(p.skipTables))...)
		}
		w.slots = &skipSlots{}
		p.skipTables[w.table] = w.slots
	}
	return w.slots
}

// walkKey returns the key of the walk at the current position: the values of the variables of the
// key and their hash for a site keyed by them, none otherwise. Reading the values and hashing them
// charge as for a memo key (hashMemoKey).
func (p *Parser) walkKey() walkKey {
	k := p.walk().key
	if k == nil {
		return walkKey{}
	}
	p.chargeValueBytes(len(k.Vars) * int(anyBytes))
	vals := p.envValues(k.Vars)
	return walkKey{vals, p.hashMemoKey(0, 0, vals)}
}

// walkBind returns the values of the variables that the element of the walk's site binds, in the
// order of RunKey.Bind, none for a site not keyed by the variables.
func (p *Parser) walkBind() []any {
	k := p.walk().key
	if k == nil || len(k.Bind) == 0 {
		return nil
	}
	p.chargeValueBytes(len(k.Bind) * int(anyBytes))
	return p.envValues(k.Bind)
}

// skipMarkPrev marks the position of the last iteration of a plain walk, which matched.
func (p *Parser) skipMarkPrev() {
	w := p.walk()
	if w.prevOK {
		p.putSkip(p.skipTable(), w.prev, walkKey{}, skipRecord{state: skipVisited})
		w.prevOK = false
	}
}

// skipRecording reports whether the walk records from the current position on, which it starts
// to at the first position that is not fresh.
func (p *Parser) skipRecording() bool {
	w := p.walk()
	if !w.recording && w.slots.marked(p.Pos) {
		p.skipMarkPrev()
		w.recording = true
	}
	return w.recording
}

// SkipHit is called before an iteration after the minimum. It ends the iteration before and, in a
// recording walk, reports whether the current position has a record, which it then reuses. The
// walk then stops.
func (p *Parser) SkipHit() bool {
	if !p.skipRecording() {
		return false
	}
	w := p.walk()
	p.skipClose()
	p.Spend()
	r := p.getSkip(w.slots, p.Pos, p.walkKey())
	if r.state != skipRecorded || w.build && !r.list {
		return false
	}
	p.Spend()
	p.documentTouch(r.examined)
	p.documentTouchFrom(r.from)
	p.MergeExpected(r.far, r.exp)
	p.Pos = r.end
	if w.key != nil {
		for i, slot := range w.key.Bind {
			p.bind(slot, r.bind[i])
		}
	}
	if r.errs != nil {
		p.recAdd(*r.errs)
	}
	w.hit, w.hitOK = r, true
	return true
}

// SkipVisit begins an iteration at the current position: in a plain walk it marks the position of
// the iteration before, which matched, visited; in a recording walk it ends the iteration before
// and isolates this one.
func (p *Parser) SkipVisit() {
	w := p.walk()
	if !p.skipRecording() {
		p.skipMarkPrev()
		w.prev, w.prevOK = p.Pos, true
		return
	}
	p.skipClose()
	w.visits = append(w.visits, skipVisit{pos: p.Pos, key: p.walkKey(), vi: w.count(), e0: len(p.Recovered)})
	w.open, w.savedLow, w.savedExamined = true, p.docLow, p.docExamined
	if p.Document != nil {
		p.docLow = p.Pos
	}
	if p.track {
		p.docExamined = p.Pos
	}
	w.mk = p.Isolate(p.Pos)
}

// count returns the number of elements the walk collected.
func (w *skipWalk) count() int {
	if w.typed {
		return len(w.tvals.vals)
	}
	return len(w.nodes.vals)
}

// skipClose ends the running iteration, if any: it keeps its expectations and examined range
// and records them as usual.
func (p *Parser) skipClose() {
	w := p.walk()
	if !w.open {
		return
	}
	w.open = false
	far, inner := p.Unisolate(w.mk)
	v := &w.visits[len(w.visits)-1]
	v.far, v.exp = far, p.Keep(inner)
	p.MergeExpected(far, inner)
	v.from, v.examined = p.docLow, p.docExamined
	v.errs = p.recSince(v.e0)
	p.docLow, p.docExamined = min(w.savedLow, p.docLow), max(w.savedExamined, p.docExamined)
}

// SkipEnd ends the walk and records the rest of it from each position it visited recording. A walk
// that builds values drops them.
func (p *Parser) SkipEnd() {
	p.skipEnd(false)
	w := p.walk()
	clearSpend(p, w.nodes.vals)
	clearSpend(p, w.tvals.vals)
	w.nodes.vals, w.nodes.buf, w.tvals.vals, w.tvals.buf = w.nodes.vals[:0], nil, w.tvals.vals[:0], nil
	p.skipN--
}

// SkipKids ends a walk begun with build as SkipEnd does, and returns the values of its iterations,
// then those of the record it took, as a child list. The list may share its elements with the
// lists of other walks. Taking the n values that the walk gathered off into the list is one
// reservation of 2n units, made before any of them moves, as Kids makes for a repetition that keeps
// no records; the elements that a reused record adds after them are charged as copies where they
// are copied.
func (p *Parser) SkipKids() []*core.Node {
	if n := len(p.walk().nodes.vals); n > 0 {
		p.spendValue(2 * n)
	}
	p.skipEnd(true)
	w := p.walk()
	kids := endVals(p, &w.nodes, p.Nodes)
	p.skipN--
	return kids
}

// SkipKids is the Parser's SkipKids in a typed parse.
func (p *TParser) SkipKids() []any {
	if n := len(p.walk().tvals.vals); n > 0 {
		p.spendValue(2 * n)
	}
	p.skipEnd(true)
	w := p.walk()
	kids := endVals(p.Parser, &w.tvals, p.NewVals)
	p.skipN--
	return kids
}

// endVals returns the list of the values of a walk that ended: the placed list, or a copy of vals
// in storage that alloc makes; it then drops vals. The caller has charged the move.
func endVals[T any](p *Parser, wv *walkVals[T], alloc func(int) []T) []T {
	var kids []T
	if wv.placed {
		kids = wv.buf.a[wv.lo:len(wv.buf.a):len(wv.buf.a)]
	} else {
		kids = alloc(len(wv.vals))
		copy(kids, wv.vals)
	}
	clear(wv.vals)
	wv.vals, wv.buf = wv.vals[:0], nil
	return kids
}

// skipEnd ends the walk and records the rest of it from each position it visited recording. paid
// says that the move of the gathered values into the list has been charged (SkipKids).
func (p *Parser) skipEnd(paid bool) {
	w := p.walk()
	p.skipClose()
	vs := w.visits
	endBind := p.walkBind()
	var next skipRecord
	if w.hitOK {
		endBind = w.hit.bind
	}
	if w.build && (len(vs) > 0 || w.hitOK) {
		if w.typed {
			w.tvals.buf, w.tvals.lo = placeVals(p, w.tvals.vals, w.hit.tvals, w.hitOK, paid)
			w.tvals.placed = true
		} else {
			w.nodes.buf, w.nodes.lo = placeVals(p, w.nodes.vals, w.hit.nodes, w.hitOK, paid)
			w.nodes.placed = true
		}
	}
	switch {
	case w.hitOK:
		next = w.hit
	case len(vs) > 0:
		// The last iteration failed or matched nothing: the walk ended at its start.
		v := vs[len(vs)-1]
		vs = vs[:len(vs)-1]
		next = skipRecord{end: v.pos, far: v.far, exp: v.exp, from: v.from, examined: v.examined, state: skipRecorded, list: w.build, bind: endBind}
		p.skipLink(w, &next, v, nil)
		p.skipPut(v.pos, v.key, next)
	}
	for i := len(vs) - 1; i >= 0; i-- {
		v := vs[i]
		r := skipRecord{end: next.end, far: next.far, exp: next.exp, from: min(v.from, next.from), examined: max(v.examined, next.examined), state: skipRecorded, list: w.build, bind: endBind}
		switch {
		case v.far > next.far:
			r.far, r.exp = v.far, v.exp
		case v.far == next.far:
			r.exp = mergeSkipExpected(v.exp, next.exp)
		}
		p.skipLink(w, &r, v, next.errs)
		p.skipPut(v.pos, v.key, r)
		next = r
	}
	w.visits = vs[:0]
}

// skipLink sets the elements and errors of the record r of the visit v: the elements of the walk
// from the iteration on, and the errors of the iteration before those of the rest, rest.
func (p *Parser) skipLink(w *skipWalk, r *skipRecord, v skipVisit, rest *recEntry) {
	if i := w.nodes.lo + v.vi; w.nodes.placed && i < len(w.nodes.buf.a) {
		r.nodes = valSuffix[*core.Node]{w.nodes.buf, i} // (an empty suffix keeps nothing alive)
	}
	if i := w.tvals.lo + v.vi; w.tvals.placed && i < len(w.tvals.buf.a) {
		r.tvals = valSuffix[any]{w.tvals.buf, i}
	}
	switch {
	case v.errs.none():
		r.errs = rest
	case rest == nil:
		e := v.errs
		r.errs = &e
		p.skipBytes += recEntryBytes
	default:
		// The list of an iteration's errors and the rest's: a hit records the rest of the walk as one
		// entry, and each list holds an error of its own iteration, so listing k errors still visits
		// at most 2k lists.
		l := &errList{entries: []recEntry{v.errs, *rest}, n: v.errs.len() + rest.len()}
		r.errs = &recEntry{list: l}
		p.skipBytes += errListBytes + 3*recEntryBytes
	}
}

// placeVals returns a buffer and an index in it from which vals, then the elements of tail (if the
// walk took a record, hit), are laid out. It writes vals in place before the tail where the buffer
// has room there that no record refers to, and otherwise copies the tail with room to spare, so
// that a walk that goes on to reach a record again finds it. The copy of vals is charged unless paid
// says that it has been.
func placeVals[T any](p *Parser, vals []T, tail valSuffix[T], hit, paid bool) (*valBuf[T], int) {
	place := func(dst []T) {
		if paid {
			copy(dst, vals)
		} else {
			copySpend(p, dst, vals)
		}
	}
	if !hit {
		tail = valSuffix[T]{}
	}
	n := len(vals)
	if n == 0 && tail.buf != nil {
		return tail.buf, tail.i
	}
	tl := 0
	if tail.buf != nil {
		tl = len(tail.buf.a) - tail.i
		if tail.i == tail.buf.lo && tail.i >= n {
			b := tail.buf
			place(b.a[tail.i-n : tail.i])
			b.lo = tail.i - n
			return b, b.lo
		}
	}
	slack := 0
	if tl > 0 {
		slack = n + tl
	}
	a := make([]T, slack+n+tl)
	p.skipBytes += uint64(len(a)) * ptrBytes
	place(a[slack:])
	if tl > 0 {
		copySpend(p, a[slack+n:], tail.buf.a[tail.i:])
	}
	return &valBuf[T]{a: a, lo: slack}, slack
}

// mergeSkipExpected returns the expectations a, then those of b not in a.
func mergeSkipExpected(a, b []ExpID) []ExpID {
	if len(a) == 0 {
		return b
	}
	out := a
	for _, x := range b {
		found := false
		for _, y := range a {
			if x == y {
				found = true
				break
			}
		}
		if !found {
			if len(out) == len(a) {
				out = append(make([]ExpID, 0, len(a)+len(b)), a...)
			}
			out = append(out, x)
		}
	}
	return out
}

// skipPut stores the record r of the position pos in the slots of the walk.
func (p *Parser) skipPut(pos int, k walkKey, r skipRecord) {
	p.Spend()
	p.putSkip(p.skipTable(), pos, k, r)
}

// pruneSkipRecords drops the records of the positions before pos, where a stream that committed
// at pos never parses again.
func (p *Parser) pruneSkipRecords(pos int) {
	for _, s := range p.skipTables {
		if s != nil {
			s.drop(pos)
		}
	}
}
