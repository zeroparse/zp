package state

import (
	"sync/atomic"
	"unsafe"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/core/impl"
	"zeroparse.dev/zp/internal/value"
)

// Skip runs: the records of the walks of skip-run sites (compile/skip_sites.go; see "Skip Runs" in
// spec/runtime/parsing.md and the proofs in spec/proofs/skip-runs.md).
//
// A walk of a site repetition visits positions r0 < r1 < ... with one iteration each. The slot of
// a position in the table of the site (and lookahead state) is fresh, visited or recorded. A walk
// starts plain: it marks each position it leaves after a matching iteration as visited and
// records nothing, so a walk that fails at once touches no slot. At its first
// iteration at a position that is not fresh, which an earlier walk of the site visited, it starts
// recording: from there on, for every position it visits, it records what the rest of the walk
// from there did (where it ended, its farthest failure and the expectations there, and the input
// range it examined). A recording walk that reaches a recorded position (once it has matched its
// minimum) takes the rest from the record: it moves to the end, records the expectations again and
// extends the examined range, as a memo hit does. The record of a position combines the iteration
// there with the record of the next position, so a walk fills the records of its recording visits
// backward when it ends: at a failed or empty iteration, whose record ends at its own start, or at
// a recorded position. A plain walk thus pays for nothing but its marks, and a position is walked
// at most once before it is marked and once more before it has a record, besides the last iteration
// of a plain walk and the minimum number of iterations of each walk.
//
// The records of a parse last for that parse; a stream drops those before a commit. A parse with
// missing-token insertion records nothing.

// SkipRecordsOff disables skip-run records, for tests that compare parses with and without them.
var SkipRecordsOff = false

// SkipRecordMutation, when not 0, breaks the records for the negative controls of the tests: a
// reused record leaves out one part of what it records (SkipKeepPos, SkipNoExpected,
// SkipNoExamined, SkipNoFrom), walks never record (SkipNeverRecord), or a recording visit stores
// a record of its own iteration when the iteration ends, which the end of the walk leaves as it is
// (SkipEarlyStore).
var SkipRecordMutation = 0

// The parts that SkipRecordMutation leaves out.
const (
	SkipKeepPos      = 1 + iota // the end: the position stays at the record
	SkipNoExpected              // the farthest failure and its expectations
	SkipNoExamined              // the end of the examined range
	SkipNoFrom                  // the start of the examined range
	SkipNeverRecord             // walks stay plain
	SkipEarlyStore              // records are stored when their iteration ends and never completed
	SkipIgnoreEnv               // the key leaves out the variables
	SkipKeepEnv                 // a hit leaves the variables as they are, not as the walk of the record bound them
	SkipAnyList                 // a walk that builds values takes the records of value-free walks
	SkipKeyEnvObject            // the key holds the environment as an object besides the values of its variables
	SkipScanKeys                // a lookup steps through every record of the site before it takes the one of its key
)

// The states of a slot.
const (
	skipFresh    = iota // no walk visited the position
	skipVisited         // a walk visited it without recording
	skipRecorded        // the record holds the rest of a walk from it
)

// skipRecord is the state of a position and, once recorded, what the rest of a walk from it did:
// where it ended and the values it left in the variables that the element binds (bind, in the order
// of RunKey.Bind; none for a site not keyed by the variables), its farthest failure, the input it examined, the
// elements it collected (a run site that builds a value) and the errors it recovered. list reports
// that a walk that builds values made the record, so that it holds the elements: the value-free
// twin of a rule walks the same site without them, and a walk that builds values does not take a
// record of such a walk, but records the position again.
type skipRecord struct {
	end, far       int
	exp            []ExpID
	from, examined int
	state          uint8
	list           bool
	// key holds the values of the variables of the key that the walk from the position started with.
	key, bind []any
	vals      valSuffix
	errs      *RecEntry // nil for none, so that a record of no errors stays small
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

// valBuf holds the elements that walks collected. A record holds a suffix of a buffer, which is
// never written once a record refers to it, except below lo: a walk that reaches a record whose
// suffix starts at lo puts its own elements before it, and shares the rest.
type valBuf struct {
	a  []*core.Node
	lo int
}

// valSuffix is the elements buf.a[i:], none if buf is nil.
type valSuffix struct {
	buf *valBuf
	i   int
}

func (s valSuffix) slice() []*core.Node {
	if s.buf == nil {
		return nil
	}
	return s.buf.a[s.i:len(s.buf.a):len(s.buf.a)]
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

// get returns the slot of pos for walks that start with the key k; a fresh slot is the zero record.
// Comparing the values of the key charges as comparing those of a memo key does.
func (p *Parser) getSkip(s *skipSlots, pos int, k walkKey) skipRecord {
	if !s.marked(pos) {
		return skipRecord{}
	}
	sk := skipKey{pos, k.h}
	if SkipRecordMutation == SkipScanKeys && p != nil {
		for range s.recs {
			p.Spend()
		}
	}
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

// put sets the slot of pos to r, which is visited or recorded; a record without values leaves one
// with values in place. Room made for bits before base is as
// large as the bits, so walks that start farther and farther back copy them a logarithmic number of
// times.
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
			// A record with values serves the walks without values too, and holds what this one
			// holds besides them (they rest on the same evaluations), so it stays: a key holds a
			// record without values only until a walk with values records it, and never after.
			return
		}
		s.recs[sk] = r
		s.expBytes += uint64(len(r.exp)) * expIDBytes
	}
}

// get and put are getSkip and putSkip for the slots of a site whose walks are not keyed by the
// variables, such as a scan site; env is always nil.
func (s *skipSlots) get(pos int, env *Env) skipRecord {
	if env != nil {
		panic("skipSlots.get: a keyed walk goes through Parser.getSkip")
	}
	return (*Parser)(nil).getSkip(s, pos, walkKey{})
}

func (s *skipSlots) put(pos int, env *Env, r skipRecord) {
	if env != nil {
		panic("skipSlots.put: a keyed walk goes through Parser.putSkip")
	}
	(*Parser)(nil).putSkip(s, pos, walkKey{}, r)
}

// putMore stores r in the records of sk whose key shares the hash of the one in recs, keeping a
// record with values over one without, as put does.
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
	// key is the key the iteration started with (none for a site not keyed by the variables), and vi the
	// number of elements collected before it. e0 is the number of entries of the record of recovered
	// errors before it, and errs the errors it recovered.
	key    walkKey
	vi, e0 int
	errs   RecEntry
}

// skipWalk is a walk in progress. Walks nest: the element of a run site can call rules that walk
// sites of their own, so the walks in progress form a stack (Parser.skips), and the innermost
// one is the one in use.
type skipWalk struct {
	on bool
	// key holds the slots of the variables that key the records of the site (compile.RunKey), nil
	// for a site whose element reads or binds no variable, directly or in the rules it calls
	// (compile.Program.EnvFree): its records hold for every environment, which a walk keeps. A
	// walk that reuses the record of a keyed site binds the variables of the key's Bind to the
	// values the walk of the record left in them. The program says which sites are free (the
	// closure backend asks compile.Program, a module marks them in its instructions), and the
	// program's RunKey gives the variables of the others.
	key *runKeySlots
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
	// from mk, and the examined range from before it is savedHW and savedLW.
	open             bool
	mk               ExpMark
	savedHW, savedLW int
	hit              skipRecord
	hitOK            bool
	// build reports that the walk collects the values of its iterations (vals), whose list the end
	// of the walk makes. placed reports that the list is placed in buf from lo on, so that the
	// records share it.
	build  bool
	vals   []*core.Node
	placed bool
	buf    *valBuf
	lo     int
}

// walk returns the walk in use, the innermost one in progress.
func (p *Parser) walk() *skipWalk { return &p.skips[p.skipN-1] }

// RunHotSpan is the length of input that a walk of a run site may walk again, below the furthest
// point that the walks before it reached, before the site turns hot. A walk shorter than that
// neither turns a site hot nor can walk that far again, so a caller need not pass it to RunWalked.
// It is a variable and exported only as a hook for tests in other packages, which lower it to run the
// hot path on short inputs and restore it; nothing else writes it, and it is not an option.
var RunHotSpan = 64

// RunHotTurns counts the run sites that turned hot. It is a hook for tests in other packages that
// check a parse reaches the hot path; nothing reads it in a parse. It is atomic because parses run
// concurrently, and a per-parse counter would need the tests to reach the parser of each parse. A turn happens once for each site
// and lookahead state of a parse, so counting it costs nothing on the hot path.
var RunHotTurns atomic.Int64

// runCold is the state of a run site (and lookahead state) before it keeps records: the furthest
// position that its walks passed, among the walks that RunWalked was told of (the engine tells it of
// walks of RunHotSpan or more). A site turns hot when a walk begins below that position and walks
// RunHotSpan or more of the input below it. A walk of a site that is not hot costs its length, and
// the part of it below the furthest position is under RunHotSpan or the site turns hot, so the walks
// of a cold site cost in total at most the input length plus RunHotSpan for each walk: linear in the
// input and the number of walks, however the walks alternate. A site that turns hot pays for records
// instead, and a run site whose walks never go back, as in a list parsed once, allocates nothing.
type runCold struct {
	end int
	hot bool
}

func (p *Parser) runIndex(site int) int {
	if p.Silent > 0 {
		return 2*site + 1
	}
	return 2 * site
}

// RunHot reports whether the walks of the run site keep records. A site turns hot only through
// RunWalked, which does nothing when records are off or the parse inserts missing tokens. A walk of
// a site that is not hot is an ordinary repetition, whose end the caller passes to RunWalked.
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
func (p *Parser) RunWalked(site, start int) { p.walked(site, start, RunHotSpan) }

// walked is RunWalked for a site whose walks turn it hot at span characters below the furthest position.
func (p *Parser) walked(site, start, span int) {
	if SkipRecordsOff || p.Missing != nil {
		return
	}
	i := p.runIndex(site)
	if i >= len(p.runCold) {
		// One table for all the sites: grown to twice the index, it is made once for most programs.
		grown := make([]runCold, max(16, 2*(i+1)))
		copy(grown, p.runCold)
		p.runCold = grown
	}
	c := &p.runCold[i]
	if start < c.end && min(p.Pos, c.end)-start >= span {
		c.hot = true
		RunHotTurns.Add(1)
	}
	c.end = max(c.end, p.Pos)
}

// SkipBegin starts a walk of the site at the current position. keyEnv reports that the records of
// the site are keyed by the variables (skipWalk.key).
func (p *Parser) SkipBegin(site int, keyEnv bool) { p.skipBegin(site, false, keyEnv) }

// SkipBeginList starts a walk of the site, whose iterations have values (SkipValue) that SkipEndList
// gathers in a list.
func (p *Parser) SkipBeginList(site int, keyEnv bool) { p.skipBegin(site, true, keyEnv) }

func (p *Parser) skipBegin(site int, build, keyEnv bool) {
	if p.skipN == len(p.skips) {
		p.skips = append(p.skips, skipWalk{})
	}
	p.skipN++
	w := p.walk()
	w.build, w.placed, w.vals = build, false, w.vals[:0]
	w.on = !SkipRecordsOff && p.Missing == nil
	if !w.on {
		return
	}
	w.key = nil
	if keyEnv {
		w.key = p.Prog.runKeySlots(site)
	}
	i := 2 * site
	if p.Silent > 0 {
		i++
	}
	w.table, w.slots = i, nil
	if i < len(p.skipTables) {
		w.slots = p.skipTables[i]
	}
	w.visits, w.recording, w.prevOK, w.open, w.hitOK = w.visits[:0], false, false, false, false
}

// SkipValue adds the value of the iteration that matched. Nodes in records are shared by the lists
// that reuse them, so a node that the rule that made it may still change is final from here.
func (p *Parser) SkipValue(v *core.Node) {
	w := p.walk()
	if v != nil {
		value.D(v).Fresh = false
	}
	w.vals = append(w.vals, v)
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
// key and their hash for a site keyed by them, none otherwise. Reading the values charges as a memo
// key does, and hashing them one unit and the bytes of their strings, as the hash of a memo key.
func (p *Parser) walkKey() walkKey {
	k := p.walk().key
	if k == nil || SkipRecordMutation == SkipIgnoreEnv {
		return walkKey{}
	}
	p.ChargeValueBytes(len(k.vars) * int(anyBytes))
	vals := p.EnvValues(k.vars)
	h, bytes := memoKeyHash(0, 0, false, vals)
	if SkipRecordMutation == SkipKeyEnvObject {
		// The object is compared by identity (rawValueEqual) and hashed by its address.
		vals = append(vals, p.Env)
		h ^= uint64(uintptr(unsafe.Pointer(p.Env))) * 0x9e3779b97f4a7c15
	}
	if BudgetControlsEnabled && p.Control != nil {
		p.Spend()
		if bytes > 0 {
			p.SpendValue(bytes)
		}
	}
	return walkKey{vals, h}
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
	if !w.recording && w.slots.marked(p.Pos) && SkipRecordMutation != SkipNeverRecord {
		p.skipMarkPrev()
		w.recording = true
	}
	return w.recording
}

// SkipHit is called before an iteration after the minimum. It ends the iteration before and, in a
// recording walk, reports whether the current position has a record, which it then reuses. The
// walk then stops.
func (p *Parser) SkipHit() bool {
	w := p.walk()
	if !w.on || !p.skipRecording() {
		return false
	}
	p.skipClose()
	p.Spend()
	r := p.getSkip(w.slots, p.Pos, p.walkKey())
	if r.state != skipRecorded || w.build && !r.list && SkipRecordMutation != SkipAnyList {
		return false
	}
	p.Spend()
	if SkipRecordMutation != 0 {
		r = mutateSkipRecord(r, p.Pos)
	}
	p.touch(r.examined)
	p.lw = Min2(p.lw, r.from)
	p.MergeExpected(r.far, r.exp)
	p.Pos = r.end
	if w.key != nil && SkipRecordMutation != SkipKeepEnv {
		for i, slot := range w.key.bind {
			p.Bind(slot, r.bind[i])
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
	if !w.on {
		return
	}
	if !p.skipRecording() {
		p.skipMarkPrev()
		w.prev, w.prevOK = p.Pos, true
		return
	}
	p.skipClose()
	w.visits = append(w.visits, skipVisit{pos: p.Pos, key: p.walkKey(), vi: len(w.vals), e0: len(p.Recovered)})
	w.open, w.savedHW, w.savedLW = true, p.Hw, p.lw
	p.Hw, p.lw = p.Pos, p.Pos
	w.mk = p.Isolate(p.Pos)
}

// skipClose ends the running iteration, if any: it keeps its expectations and examined range and
// records them as usual.
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
	v.from, v.examined = p.lw, p.Hw
	v.errs = p.recSince(v.e0)
	p.Hw, p.lw = max(w.savedHW, p.Hw), Min2(w.savedLW, p.lw)
	if SkipRecordMutation == SkipEarlyStore {
		p.putSkip(p.skipTable(), v.pos, v.key, skipRecord{end: p.Pos, far: v.far, exp: v.exp, from: v.from, examined: v.examined, state: skipRecorded, bind: p.walkBind()})
	}
}

// SkipEnd ends the walk and records the rest of it from each position it visited recording.
func (p *Parser) SkipEnd() {
	p.skipEnd(false)
	p.walk().vals = p.walk().vals[:0]
	p.skipN--
}

// SkipEndList ends a walk begun with SkipBeginList as SkipEnd does, and returns the list node of
// the values of its iterations for the range from start to the current position, or nil if build
// is false. The list may share its elements with the lists of other walks. Taking the n values that
// the walk gathered off into the list is one reservation of 2n units, made before any of them moves,
// as Parser.Kids makes for a repetition that keeps no records; the elements that a reused record
// adds after them are charged as copies where they are copied. Without a list, dropping the gathered
// values is charged if chargeClear is set, as the closure backend charges dropping its element
// stack; the bytecode machine pops its elements uncharged.
func (p *Parser) SkipEndList(start int, build, chargeClear bool) *core.Node {
	if n := len(p.walk().vals); build && n > 0 {
		p.SpendValue(2 * n)
	}
	p.skipEnd(build)
	w := p.walk()
	var kids []*core.Node
	if build {
		if w.placed {
			kids = w.buf.a[w.lo:len(w.buf.a):len(w.buf.a)]
		} else {
			kids = p.Nodes(len(w.vals))
			copy(kids, w.vals)
		}
	}
	if chargeClear && !build {
		ClearSpend(p, w.vals)
	} else {
		clear(w.vals)
	}
	w.vals, w.buf = w.vals[:0], nil
	p.skipN--
	if !build {
		return nil
	}
	return p.NewNode(value.Data{Kind: coreimpl.ListKind, Start: int32(start), End: int32(p.Pos), Children: kids, Fresh: true})
}

// skipEnd ends the walk and records the rest of it from each position it visited recording. paid
// says that the move of the gathered values into the list has been charged (SkipEndList).
func (p *Parser) skipEnd(paid bool) {
	w := p.walk()
	if !w.on {
		return
	}
	p.skipClose()
	w.on = false
	if SkipRecordMutation == SkipEarlyStore {
		w.visits = w.visits[:0]
		return
	}
	vs := w.visits
	endBind := p.walkBind()
	var next skipRecord
	var tail valSuffix
	if w.hitOK {
		tail = w.hit.vals
		if SkipRecordMutation != SkipKeepEnv {
			endBind = w.hit.bind
		}
	}
	if w.build && (len(vs) > 0 || w.hitOK) {
		w.buf, w.lo = p.placeVals(w.vals, tail, paid)
		w.placed = true
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
		r := skipRecord{end: next.end, far: next.far, exp: next.exp, from: Min2(v.from, next.from), examined: max(v.examined, next.examined), state: skipRecorded, list: w.build, bind: endBind}
		switch {
		case v.far > next.far:
			r.far, r.exp = v.far, v.exp
		case v.far == next.far:
			r.exp = mergeExp(v.exp, next.exp)
		}
		p.skipLink(w, &r, v, next.errs)
		p.skipPut(v.pos, v.key, r)
		next = r
	}
	w.visits = vs[:0]
}

// skipLink sets the elements and errors of the record r of the visit v: the elements of the walk
// from the iteration on, and the errors of the iteration before those of the rest, rest.
func (p *Parser) skipLink(w *skipWalk, r *skipRecord, v skipVisit, rest *RecEntry) {
	if i := w.lo + v.vi; w.placed && i < len(w.buf.a) {
		r.vals = valSuffix{w.buf, i} // (an empty suffix keeps nothing alive)
	}
	switch {
	case v.errs.None():
		r.errs = rest
	case rest == nil:
		e := v.errs
		r.errs = &e
		p.skipBytes += recEntryBytes
	default:
		// The list of an iteration's errors and the rest's: a hit records the rest of the walk as one
		// entry, and each list holds an error of its own iteration, so listing k errors still visits
		// at most 2k lists.
		l := &ErrList{entries: []RecEntry{v.errs, *rest}, n: v.errs.Len() + rest.Len()}
		r.errs = &RecEntry{list: l}
		p.skipBytes += errListBytes + 3*recEntryBytes
	}
}

// placeVals returns a buffer and an index in it from which vals, then the elements of tail, are
// laid out. It writes vals in place before the tail where the buffer has room there that no record
// refers to, and otherwise copies the tail with room to spare, so that a walk that goes on to
// reach a record again finds it. The copy of vals is charged unless paid says that it has been.
func (p *Parser) placeVals(vals []*core.Node, tail valSuffix, paid bool) (*valBuf, int) {
	n := len(vals)
	place := func(dst []*core.Node) {
		if paid {
			copy(dst, vals)
		} else {
			CopySpend(p, dst, vals)
		}
	}
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
	a := make([]*core.Node, slack+n+tl)
	p.skipBytes += uint64(len(a)) * ptrBytes
	place(a[slack:])
	if tl > 0 {
		CopySpend(p, a[slack+n:], tail.buf.a[tail.i:])
	}
	return &valBuf{a: a, lo: slack}, slack
}

// mergeExp returns the expectations a, then those of b not in a.
func mergeExp(a, b []ExpID) []ExpID {
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

// walkBind returns the values of the variables that the element of the walk's site binds, in the
// order of RunKey.Bind, none for a site not keyed by the variables.
func (p *Parser) walkBind() []any {
	k := p.walk().key
	if k == nil || len(k.bind) == 0 {
		return nil
	}
	p.ChargeValueBytes(len(k.bind) * int(anyBytes))
	return p.EnvValues(k.bind)
}

// PruneSkipRecords drops the records of the positions before pos, where a stream that committed
// at pos never parses again.
func (p *Parser) PruneSkipRecords(pos int) {
	for _, s := range p.skipTables {
		if s != nil {
			s.drop(pos)
		}
	}
}

// mutateSkipRecord returns r reused at pos without the part SkipRecordMutation names.
func mutateSkipRecord(r skipRecord, pos int) skipRecord {
	switch SkipRecordMutation {
	case SkipKeepPos:
		r.end = pos
	case SkipNoExpected:
		r.exp = nil
	case SkipNoExamined:
		r.examined = pos
	case SkipNoFrom:
		r.from = pos
	}
	return r
}
