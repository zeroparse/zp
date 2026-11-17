package state

import (
	"slices"
	"sync/atomic"

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
	SkipKeepPos       = 1 + iota // the end: the position stays at the record
	SkipNoExpected               // the farthest failure and its expectations
	SkipNoExamined               // the end of the examined range
	SkipNoFrom                   // the start of the examined range
	SkipNeverRecord              // walks stay plain
	SkipEarlyStore               // records are stored when their iteration ends and never completed
	SkipIgnoreEnv                // the key leaves out the environment
	SkipKeepEnv                  // a hit leaves the environment as it is, not the bindings of the record
	SkipKeyByIdentity            // the key is the environment as an object, not the values the element reads
	SkipNoHide                   // a walk leaves the variables that the element only binds as they are
	SkipLinearKeys               // a key is looked up by comparing it with every key made before
)

// The states of a slot.
const (
	skipFresh    = iota // no walk visited the position
	skipVisited         // a walk visited it without recording
	skipRecorded        // the record holds the rest of a walk from it
)

// skipRecord is the state of a position and, once recorded, what the rest of a walk from it did:
// where it ended, the values it left in the variables that the element binds (SiteKey.Bind; nil
// for a site free of the environment), its farthest failure, the input it examined, the elements it
// collected (a run site that builds a value) and the errors it recovered.
type skipRecord struct {
	end, far       int
	exp            []ExpID
	from, examined int
	state          uint8
	binds          []any
	vals           valSuffix
	errs           *RecEntry // nil for none, so that a record of no errors stays small
}

// skipKey is the key of a record: the position and the number of the values of the variables that
// the walk from the position read (Parser.siteKey), 0 for a site free of the environment.
type skipKey struct {
	pos int
	env int32
}

// SiteKey says which variables key the records of a run site and which a walk that takes a record
// rebinds (compile.SiteVars, by slot). A nil *SiteKey is a site free of the environment, whose
// records are kept by position alone.
//
// Read are the variables that the rest of a walk reads: its records are kept for their values, which
// a walk numbers (Parser.siteKey). Bind are those that the element binds: a record holds, for each,
// the value that the rest of the walk left in it, and a walk that takes the record binds it to that
// value. A variable of Bind that the rest of the walk did not bind keeps its value, which for one in
// Read is the value of the key, but for one in Write, which the element binds and does not read, is
// whatever the walk that takes the record has. To tell the two apart, a walk hides the variables of
// Write while it runs: it binds them to unbound at each iteration, after keeping the value that the
// iteration before left in them, so that a value other than unbound when the iteration ends is one
// the iteration bound. No part of the element reads them, so hiding them changes nothing that the
// walk does, and the walk binds them to their values when it ends.
type SiteKey struct {
	Read, Bind []int
	// write holds, for each variable of Write, its index in Bind.
	Write []int
	write []int
}

// NewSiteKey returns the key of a site whose element reads the variables in the slots read and binds
// those in bind. A slot below zero (an undeclared variable) is left out.
func NewSiteKey(read, bind []int) *SiteKey {
	k := &SiteKey{}
	in := map[int]bool{}
	for _, s := range read {
		if s >= 0 && !in[s] {
			in[s] = true
			k.Read = append(k.Read, s)
		}
	}
	for _, s := range bind {
		if s < 0 || slices.Contains(k.Bind, s) {
			continue
		}
		k.Bind = append(k.Bind, s)
		if !in[s] {
			k.Write = append(k.Write, s)
			k.write = append(k.write, len(k.Bind)-1)
		}
	}
	return k
}

// AllVars returns the key of a site of which nothing is known but that it reads and binds no more
// than the n variables of the program: every variable keys its records and is rebound by a hit.
func AllVars(n int) *SiteKey {
	all := make([]int, n)
	for i := range all {
		all[i] = i
	}
	return NewSiteKey(all, all)
}

// unbound is the value of a variable of SiteKey.Write while a walk hides it.
var unbound any = new(struct{ unbound byte })

// keyLink is an entry of Parser.siteKeys: the number of a key made of the values of a key and one
// more value.
type keyLink struct {
	prev int32
	v    any
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

// get returns the slot of pos for walks that start in the environment env; a fresh slot is the
// zero record.
func (s *skipSlots) get(pos int, env int32) skipRecord {
	if !s.marked(pos) {
		return skipRecord{}
	}
	if r, ok := s.recs[skipKey{pos, env}]; ok {
		return r
	}
	return skipRecord{state: skipVisited}
}

// put sets the slot of pos to r, which is visited or recorded. Room made for bits before base is as
// large as the bits, so walks that start farther and farther back copy them a logarithmic number of
// times.
func (s *skipSlots) put(pos int, env int32, r skipRecord) {
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
		s.recs[skipKey{pos, env}] = r
		s.expBytes += uint64(len(r.exp)) * expIDBytes
	}
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
	s.kept = len(s.recs)
}

// skipVisit is a recording visit of the walk in progress.
type skipVisit struct {
	pos, far       int
	exp            []ExpID
	from, examined int
	// env is the number of the values of the variables the iteration started with (Parser.siteKey),
	// and vi the number of elements collected before it. e0 is the number of entries of the record
	// of recovered errors before it, and errs the errors it recovered. bound holds the values that
	// the iteration bound the variables of SiteKey.Write to, unbound for those it did not bind.
	env    int32
	vi, e0 int
	errs   RecEntry
	bound  []any
}

// skipWalk is a walk in progress. Walks nest: the element of a run site can call rules that walk
// sites of their own, so the walks in progress form a stack (Parser.skips), and the innermost
// one is the one in use.
type skipWalk struct {
	on bool
	// key says which variables key the records of the site and which a walk that reuses one
	// rebinds (SiteKey). The walks of a site whose element reads or binds no variable, directly or
	// in the rules it calls (compile.Program.EnvFree), have none: their records hold for every
	// environment, which a walk keeps. The program says which sites those are (the closure backend
	// asks compile.Program, a module lists them), and a site it says nothing of is keyed by every
	// variable. keyEnv and keyID are the last environment the walk numbered and its number.
	key    *SiteKey
	keyEnv *Env
	keyID  int32
	// held holds, while the walk hides the variables of key.Write, the value each would have
	// without the hiding: the value it had when the walk began or that an iteration bound it to.
	held []any
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

// SkipBegin starts a walk of the site at the current position. key says which variables key the
// records of the site, nil for a site free of the environment (skipWalk.key).
func (p *Parser) SkipBegin(site int, key *SiteKey) { p.skipBegin(site, false, key) }

// SkipBeginList starts a walk of the site, whose iterations have values (SkipValue) that SkipEndList
// gathers in a list.
func (p *Parser) SkipBeginList(site int, key *SiteKey) { p.skipBegin(site, true, key) }

func (p *Parser) skipBegin(site int, build bool, key *SiteKey) {
	if p.skipN == len(p.skips) {
		p.skips = append(p.skips, skipWalk{})
	}
	p.skipN++
	w := p.walk()
	w.build, w.placed, w.vals = build, false, w.vals[:0]
	w.on = !SkipRecordsOff && p.Missing == nil
	w.key, w.keyEnv = key, nil
	if !w.on {
		return
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
	if key != nil && len(key.Write) > 0 {
		w.held = w.held[:0]
		for _, s := range key.Write {
			w.held = append(w.held, p.Env.Get(s, p.envLevels))
		}
		p.hide()
	}
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

// walkEnv returns the number of the key of the records of the walk at the current position: that
// of the values of the variables the site reads, 0 for a site free of the environment.
func (p *Parser) walkEnv() int32 {
	w := p.walk()
	switch {
	case w.key == nil || SkipRecordMutation == SkipIgnoreEnv:
		return 0
	case w.keyEnv == p.Env && w.keyID != 0:
		// The same environment has the same values.
		return w.keyID
	}
	w.keyEnv, w.keyID = p.Env, p.siteKey(w.key)
	return w.keyID
}

// siteKey numbers the values of the variables of key.Read in the current environment: equal values
// have equal numbers, from 1, for the whole parse. A number is made one value at a time from the
// number of the values before it, so a lookup costs a step and a hash of the value for each variable,
// and a key of string values hashes the strings, as a memo lookup of a rule that reads them compares
// them.
func (p *Parser) siteKey(key *SiteKey) int32 {
	if SkipRecordMutation == SkipKeyByIdentity {
		return p.envIdentity()
	}
	var id int32
	for _, s := range key.Read {
		v := p.VarValue(s)
		p.Spend()
		if t, ok := v.(string); ok {
			p.SpendValue(len(t) / 8)
		}
		if SkipRecordMutation == SkipLinearKeys {
			id = p.linearKey(id, v)
			continue
		}
		l := keyLink{id, v}
		next, ok := p.siteKeys[l]
		if !ok {
			if p.siteKeys == nil {
				p.siteKeys = map[keyLink]int32{}
			}
			next = int32(len(p.siteKeys) + 1)
			p.siteKeys[l] = next
			p.skipBytes += keyLinkBytes
		}
		id = next
	}
	return id
}

// keyLinkBytes is what an entry of Parser.siteKeys holds, for the memory limit.
const keyLinkBytes = 4 + 16 + 4 + 16

// envIdentity numbers the environment as an object, for the mutation SkipKeyByIdentity.
func (p *Parser) envIdentity() int32 {
	l := keyLink{-1, p.Env}
	id, ok := p.siteKeys[l]
	if !ok {
		if p.siteKeys == nil {
			p.siteKeys = map[keyLink]int32{}
		}
		id = int32(len(p.siteKeys) + 1)
		p.siteKeys[l] = id
	}
	return id
}

// linearKey is a lookup in Parser.siteKeys that compares the link with every link made before, for
// the mutation SkipLinearKeys.
func (p *Parser) linearKey(prev int32, v any) int32 {
	for l, id := range p.siteKeys {
		p.Spend()
		if l.prev == prev && l.v == v {
			return id
		}
	}
	if p.siteKeys == nil {
		p.siteKeys = map[keyLink]int32{}
	}
	id := int32(len(p.siteKeys) + 1)
	p.siteKeys[keyLink{prev, v}] = id
	return id
}

// hide keeps the values that the iteration that ended left in the variables of key.Write, in the
// walk and in its last visit, and binds them to unbound (SiteKey).
func (p *Parser) hide() {
	w := p.walk()
	if SkipRecordMutation == SkipNoHide {
		return
	}
	var bound []any
	if n := len(w.visits); n > 0 && w.recording {
		bound = w.visits[n-1].bound
	}
	for i, s := range w.key.Write {
		v := p.Env.Get(s, p.envLevels)
		if v == unbound {
			continue
		}
		w.held[i] = v
		if bound != nil {
			bound[i] = v
		}
		p.Env = p.Env.Set(s, p.envLevels, unbound)
	}
}

// unhide ends the hiding of the variables of key.Write: it binds each to the value it would have
// without it.
func (p *Parser) unhide() {
	w := p.walk()
	if SkipRecordMutation == SkipNoHide {
		return
	}
	for i, s := range w.key.Write {
		if p.Env.Get(s, p.envLevels) == unbound {
			p.Env = p.Env.Set(s, p.envLevels, w.held[i])
		} else {
			w.held[i] = p.Env.Get(s, p.envLevels)
		}
	}
}

// hides reports whether the walk hides variables.
func (w *skipWalk) hides() bool { return w.key != nil && len(w.key.Write) > 0 }

// skipMarkPrev marks the position of the last iteration of a plain walk, which matched.
func (p *Parser) skipMarkPrev() {
	w := p.walk()
	if w.prevOK {
		p.skipTable().put(w.prev, 0, skipRecord{state: skipVisited})
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
	if w.hides() {
		p.hide()
	}
	p.Spend()
	r := w.slots.get(p.Pos, p.walkEnv())
	if r.state != skipRecorded {
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
		p.takeBinds(r.binds)
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
		if w.hides() {
			p.hide()
		}
		p.skipMarkPrev()
		w.prev, w.prevOK = p.Pos, true
		return
	}
	p.skipClose()
	if w.hides() {
		p.hide()
	}
	v := skipVisit{pos: p.Pos, env: p.walkEnv(), vi: len(w.vals), e0: len(p.Recovered)}
	if w.hides() {
		v.bound = p.unboundVals(len(w.key.Write))
	}
	w.visits = append(w.visits, v)
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
		p.skipTable().put(v.pos, v.env, skipRecord{end: p.Pos, far: v.far, exp: v.exp, from: v.from, examined: v.examined, state: skipRecorded, binds: p.endBinds()})
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
	if w.hides() {
		p.hide()
	}
	w.on = false
	if SkipRecordMutation == SkipEarlyStore {
		w.visits = w.visits[:0]
		if w.hides() {
			p.unhide()
		}
		return
	}
	vs := w.visits
	var next skipRecord
	var tail valSuffix
	if w.hitOK {
		tail = w.hit.vals
	}
	// The variables of the key that the element binds end with the values they have now, whichever
	// record a later walk takes, as the key holds their values at its start.
	var ends []any
	if w.key != nil {
		ends = p.endBinds()
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
		next = skipRecord{end: v.pos, far: v.far, exp: v.exp, from: v.from, examined: v.examined, state: skipRecorded, binds: p.recordBinds(ends, v.bound, nil)}
		p.skipLink(w, &next, v, nil)
		p.skipPut(v.pos, v.env, next)
	}
	for i := len(vs) - 1; i >= 0; i-- {
		v := vs[i]
		r := skipRecord{end: next.end, far: next.far, exp: next.exp, from: Min2(v.from, next.from), examined: max(v.examined, next.examined), state: skipRecorded, binds: p.recordBinds(ends, v.bound, next.binds)}
		switch {
		case v.far > next.far:
			r.far, r.exp = v.far, v.exp
		case v.far == next.far:
			r.exp = mergeExp(v.exp, next.exp)
		}
		p.skipLink(w, &r, v, next.errs)
		p.skipPut(v.pos, v.env, r)
		next = r
	}
	w.visits = vs[:0]
	if w.hides() {
		p.unhide()
	}
}

// endBinds returns the values that the walk leaves in the variables of key.Bind: their values now,
// and, for those of key.Write, which the walk hides, unbound (recordBinds puts what the rest of a walk
// bound them to in their place).
func (p *Parser) endBinds() []any {
	w := p.walk()
	if w.key == nil || len(w.key.Bind) == 0 {
		return nil
	}
	ends := make([]any, len(w.key.Bind))
	for i, s := range w.key.Bind {
		ends[i] = p.Env.Get(s, p.envLevels)
	}
	for _, i := range w.key.write {
		ends[i] = unbound
	}
	p.skipBytes += uint64(len(ends)) * anyBytes
	return ends
}

// recordBinds returns the values that the rest of a walk from a visit leaves in the variables of
// key.Bind: ends for those of the key, and for those of key.Write the value that the rest after the
// visit bound one to (rest), or else the value that the iteration of the visit bound it to (bound),
// or unbound if neither bound it. Without variables of key.Write, every record of the walk shares ends.
func (p *Parser) recordBinds(ends, bound, rest []any) []any {
	w := p.walk()
	if w.key == nil || len(w.key.write) == 0 {
		return ends
	}
	out := make([]any, len(ends))
	copy(out, ends)
	for j, i := range w.key.write {
		switch {
		case rest != nil && rest[i] != unbound:
			out[i] = rest[i]
		case bound != nil:
			out[i] = bound[j]
		}
	}
	p.skipBytes += uint64(len(out)) * anyBytes
	return out
}

// takeBinds binds the variables of key.Bind as the record whose values are binds says that the rest
// of its walk left them: those of key.Write that it bound to the value the walk holds for them, which
// it binds them to when it ends (unhide), and the others to their values.
func (p *Parser) takeBinds(binds []any) {
	w := p.walk()
	if len(binds) == 0 {
		return
	}
	for i, s := range w.key.Bind {
		if binds[i] != unbound {
			if j := slices.Index(w.key.write, i); j >= 0 {
				if SkipRecordMutation != SkipNoHide {
					w.held[j] = binds[i]
					continue
				}
			}
			p.Bind(s, binds[i])
		}
	}
}

// unboundVals returns n values unbound, for the variables of key.Write that an iteration has not bound.
func (p *Parser) unboundVals(n int) []any {
	out := make([]any, n)
	for i := range out {
		out[i] = unbound
	}
	p.skipBytes += uint64(n) * anyBytes
	return out
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
func (p *Parser) skipPut(pos int, env int32, r skipRecord) {
	p.Spend()
	p.skipTable().put(pos, env, r)
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
