package ref

import (
	"fmt"
	"maps"
	"slices"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
)

// The reference model of the dependency memo of left recursion (the ctx operations): the growths in
// progress, the dependency frames of the evaluations in a context and the tables of the contexts
// (design 065, "Left-recursion contexts and growth"). Conformance cases install growths and begin and
// end lookaheads with operations of their own (CtxGrow, CtxReseed, CtxUngrow, CtxLookBegin and
// CtxLookEnd) instead of running the growth loop, so the seed identities are the cases'.

// CtxRule is the left-recursion shape of the rule of a RuleRef index: its cycle (0 for none) and
// whether it is a leader of the cycle.
type CtxRule struct {
	Cycle  int64
	Leader bool
}

// CtxLeaderBits is the number of leaders of a cycle that have a bit in a word of bits of present
// sets, those of the generated parsers. A context with another leader has as its present set an
// identity that names the set of its leaders (presentWide): a memo entry holds a word of bits, so
// no entry answers a call in such a context and every result there goes to the table of the
// context. A binding gives the same leaders a bit.
const CtxLeaderBits = 31

// presentWide tags the present set of a context with a leader that has no bit: no word of bits has
// it.
const presentWide uint64 = 1 << 63

// The values of ctx.pop.
const (
	CtxStoreNowhere int64 = iota
	CtxStoreMemo
	CtxStoreCtx
)

// obsNone is the outcome of a lookahead with none memoized in the current context.
const obsNone uint64 = 1 << 63

// CtxResult is the model's CtxResult handle: a seed, a result kept in a context table or the result of
// a growth. The null result is nil.
type CtxResult struct {
	ok             bool
	end            int64
	value, errs    any
	examined, from int64
	silent         bool
	far            int64
	kept           *ExpKept
}

// ctxRes returns the CtxResult handle a, nil for the null result, which the model returns as nil, as
// every other null handle.
func ctxRes(a any) *CtxResult {
	r, _ := a.(*CtxResult)
	return r
}

// CtxState is the model's Ctx handle: the context state of a memoized call.
type CtxState struct {
	gp          int
	present     uint64
	ctx, framed bool
	// ds are the dependencies that ctx.pop ended, and sd those that ctx.keep stores.
	ds []ctxDep
	sd []ctxSDep
	// pushed, popped and done record the calls of ctx.push, ctx.pop (its result) and ctx.done
	// with the state, for the preconditions; frame is the number of frames after ctx.push.
	pushed, done bool
	popped       int64
	frame        int
}

type depKind uint8

const (
	depSeed depKind = iota
	depObs
	depShadow
)

// ctxDep is a dependency in a frame: a seed of growths[idx] read with identity val, or the outcome val
// of the lookahead site idx observed in the context of depth d, or a shadow: a seed that an observation
// at depth d read.
type ctxDep struct {
	kind   depKind
	idx, d int
	val    uint64
}

// ctxSDep is a stored dependency: a seed or shadow by the leader's key, or an observation by site.
type ctxSDep struct {
	kind            depKind
	rule, min, site int64
	val             uint64
}

type growKey struct{ rule, min int64 }

type growthRec struct {
	pos, cycle int64
	key        growKey
	bit        uint64
	idx        int64
	seed       *CtxResult
	id         uint64
}

type ctxFrame struct {
	pos       int64
	gp, depth int
	deps      []ctxDep
}

type ctxHead struct {
	look    bool
	id, min int64
	present uint64
}

// ctxSlot is a result of a head under the values of the dependencies of its set: the result of a call
// with its key, or the outcome of a lookahead.
type ctxSlot struct {
	res     *CtxResult
	key     int64
	outcome uint64
	deps    []ctxSDep
}

type ctxSet struct {
	seeds []growKey
	obs   []int64
	vals  map[string][]*ctxSlot
}

type ctxTable struct {
	pos   int64
	depth int
	heads map[ctxHead][]*ctxSet
}

type lookRec struct {
	site     int64
	examined int64
	// frame is the number of frames after the lookahead began.
	frame int
}

// ctxModel is the state of the component growth.
type ctxModel struct {
	growths []growthRec
	frames  []ctxFrame
	tables  []*ctxTable
	looks   []lookRec
	guard   []int64
	// wide holds the identities of the present sets with a leader that has no bit, by their leaders,
	// while a growth is in progress.
	wide map[string]uint64
	// units counts the table, dependency and present-set work of the operation in progress.
	tableUnits, depUnits, presentUnits int64
	// keyCost is Model.keyCost: what comparing the key of a result inspects.
	keyCost func(int64) int64
}

func (m *Model) ctxRule(r RuleRef) CtxRule {
	if int(r.Index) < len(m.cfg.CtxRules) {
		return m.cfg.CtxRules[r.Index]
	}
	return CtxRule{}
}

// leaderIndex returns the number of the leader r in its cycle: leaders are numbered from 0 in the
// order of Config.CtxRules within each cycle.
func (m *Model) leaderIndex(r RuleRef) int64 {
	c := m.ctxRule(r)
	k := int64(0)
	for i, x := range m.cfg.CtxRules {
		if int64(i) == r.Index {
			break
		}
		if x.Leader && x.Cycle == c.Cycle {
			k++
		}
	}
	return k
}

// leaderBit returns the bit of the leader r in the words of bits of present sets, and 0 if it has
// none.
func (m *Model) leaderBit(r RuleRef) uint64 {
	c := m.ctxRule(r)
	if !c.Leader || c.Cycle == 0 {
		return 0
	}
	if k := m.leaderIndex(r); k < CtxLeaderBits {
		return 1 << k
	}
	return 0
}

func (m *Model) pos() int64 { return m.regs["pos"].(int64) }

// silent reports whether the parse is inside a lookahead (the register silent).
func (m *Model) silent() bool {
	n, _ := m.regs["silent"].(int64)
	return n > 0
}

func (s *ctxModel) gammaAt(pos, cycle int64) int {
	i := len(s.growths)
	for i > 0 && s.growths[i-1].pos == pos && s.growths[i-1].cycle == cycle {
		i--
	}
	return i
}

func (s *ctxModel) gammaFind(gp int, k growKey) int {
	for i := len(s.growths) - 1; i >= gp; i-- {
		if s.growths[i].key == k {
			return i
		}
	}
	return -1
}

// present returns the present set of the context growths[gp:]: the bits of its leaders, or, if a
// leader has no bit, the identity of the set of their numbers, which naming counts in presentUnits:
// one for each growth, and one when the set gets its identity.
func (s *ctxModel) present(gp int) uint64 {
	var p uint64
	for _, g := range s.growths[gp:] {
		if g.bit == 0 {
			return s.presentWide(gp)
		}
		p |= g.bit
	}
	return p
}

func (s *ctxModel) presentWide(gp int) uint64 {
	var ks []int64
	for _, g := range s.growths[gp:] {
		s.presentUnits++
		if !slices.Contains(ks, g.idx) {
			ks = append(ks, g.idx)
		}
	}
	slices.Sort(ks)
	key := fmt.Sprint(ks)
	if v, ok := s.wide[key]; ok {
		return v
	}
	s.presentUnits++
	if s.wide == nil {
		s.wide = map[string]uint64{}
	}
	v := presentWide | uint64(len(s.wide)+1)
	s.wide[key] = v
	return v
}

func (s *ctxModel) addDep(d ctxDep) {
	f := &s.frames[len(s.frames)-1]
	for _, x := range f.deps {
		if x.kind == d.kind && x.idx == d.idx && (d.kind != depObs || x.d == d.d) {
			return
		}
	}
	s.depUnits++
	f.deps = append(f.deps, d)
}

func (s *ctxModel) mergeDeps(ds []ctxDep) {
	if len(s.frames) == 0 {
		return
	}
	depth := s.frames[len(s.frames)-1].depth
	for _, d := range ds {
		switch {
		case d.kind == depSeed && d.idx < depth, d.kind != depSeed && d.d == depth:
			s.addDep(d)
		case d.kind == depShadow && d.idx < depth:
			s.addDep(ctxDep{kind: depSeed, idx: d.idx, val: d.val})
		}
	}
}

func (s *ctxModel) noteSeedRead(i int) {
	if len(s.frames) == 0 || i >= s.frames[len(s.frames)-1].depth {
		return
	}
	s.addDep(ctxDep{kind: depSeed, idx: i, val: s.growths[i].id})
}

func (s *ctxModel) popFrame() (ctxFrame, []ctxDep) {
	f := s.frames[len(s.frames)-1]
	s.frames = s.frames[:len(s.frames)-1]
	return f, f.deps
}

// stored converts frame dependencies to stored ones and reports whether they read a seed or observed
// a lookahead.
func (s *ctxModel) stored(ds []ctxDep) ([]ctxSDep, bool) {
	var out []ctxSDep
	read := false
	for _, d := range ds {
		if d.kind == depObs {
			out = append(out, ctxSDep{kind: depObs, site: int64(d.idx), val: d.val})
			read = true
			continue
		}
		k := s.growths[d.idx].key
		out = append(out, ctxSDep{kind: d.kind, rule: k.rule, min: k.min, val: d.val})
		read = read || d.kind == depSeed
	}
	return out, read
}

func (s *ctxModel) table(pos int64, gp int, create bool) *ctxTable {
	if n := len(s.tables); n > 0 && s.tables[n-1].pos == pos && s.tables[n-1].depth == gp {
		return s.tables[n-1]
	}
	if !create {
		return nil
	}
	t := &ctxTable{pos: pos, depth: gp, heads: map[ctxHead][]*ctxSet{}}
	s.tables = append(s.tables, t)
	return t
}

func valsKey(v []uint64) string { return fmt.Sprint(v) }

// find returns the first result of head h for the key in the table of the growths from gp at pos whose
// dependencies have the values they were stored with in the current context, and its set.
func (s *ctxModel) find(pos int64, gp int, h ctxHead, key int64) (*ctxSlot, *ctxSet) {
	t := s.table(pos, gp, false)
	if t == nil {
		return nil, nil
	}
	s.tableUnits++
next:
	for _, set := range t.heads[h] {
		s.tableUnits++
		var v []uint64
		for _, g := range set.seeds {
			i := s.gammaFind(gp, g)
			if i < 0 {
				continue next
			}
			v = append(v, s.growths[i].id)
		}
		for _, o := range set.obs {
			x := s.obsValue(pos, gp, h.present, o)
			if x == obsNone {
				continue next
			}
			v = append(v, x)
		}
		for _, slot := range set.vals[valsKey(v)] {
			s.tableUnits++
			if !h.look {
				s.tableUnits += s.keyCost(slot.key)
			}
			if slot.key == key {
				return slot, set
			}
		}
	}
	return nil, nil
}

func (s *ctxModel) obsValue(pos int64, gp int, present uint64, site int64) uint64 {
	if slices.Contains(s.guard, site) {
		return obsNone
	}
	s.guard = append(s.guard, site)
	defer func() { s.guard = s.guard[:len(s.guard)-1] }()
	slot, _ := s.find(pos, gp, ctxHead{look: true, id: site, present: present}, 0)
	if slot == nil {
		return obsNone
	}
	return slot.outcome
}

func (s *ctxModel) store(pos int64, gp int, h ctxHead, ds []ctxSDep, slot *ctxSlot) {
	var seeds []growKey
	var obs []int64
	var v, ov []uint64
	for _, d := range ds {
		switch d.kind {
		case depSeed:
			seeds, v = append(seeds, growKey{d.rule, d.min}), append(v, d.val)
		case depObs:
			obs, ov = append(obs, d.site), append(ov, d.val)
		}
	}
	v = append(v, ov...)
	t := s.table(pos, gp, true)
	s.tableUnits++
	var set *ctxSet
	for _, x := range t.heads[h] {
		s.tableUnits++
		if slices.Equal(x.seeds, seeds) && slices.Equal(x.obs, obs) {
			set = x
			break
		}
	}
	if set == nil {
		set = &ctxSet{seeds: seeds, obs: obs, vals: map[string][]*ctxSlot{}}
		t.heads[h] = append(t.heads[h], set)
	}
	slot.deps = ds
	k := valsKey(v)
	for i, x := range set.vals[k] {
		s.tableUnits++
		if !h.look {
			s.tableUnits += s.keyCost(x.key)
		}
		if x.key == slot.key {
			set.vals[k][i] = slot
			return
		}
	}
	set.vals[k] = append([]*ctxSlot{slot}, set.vals[k]...)
}

// replayDeps passes the stored dependencies ds of a result reused at pos to the innermost frame, as if
// the evaluation that produced them had run in the current context.
func (s *ctxModel) replayDeps(pos int64, gp int, present uint64, ds []ctxSDep) {
	if len(s.frames) == 0 || len(ds) == 0 {
		return
	}
	depth := len(s.growths)
	var buf []ctxDep
	for _, d := range ds {
		if d.kind == depObs {
			buf = append(buf, ctxDep{kind: depObs, idx: int(d.site), d: depth, val: d.val})
			buf = s.replayObs(buf, pos, gp, present, d.site, depth)
			continue
		}
		if i := s.gammaFind(gp, growKey{d.rule, d.min}); i >= 0 {
			buf = append(buf, ctxDep{kind: d.kind, idx: i, d: depth, val: s.growths[i].id})
		}
	}
	s.mergeDeps(buf)
}

func (s *ctxModel) replayObs(buf []ctxDep, pos int64, gp int, present uint64, site int64, depth int) []ctxDep {
	if slices.Contains(s.guard, site) {
		return buf
	}
	s.guard = append(s.guard, site)
	defer func() { s.guard = s.guard[:len(s.guard)-1] }()
	slot, set := s.find(pos, gp, ctxHead{look: true, id: site, present: present}, 0)
	if slot == nil {
		return buf
	}
	for _, k := range set.seeds {
		if i := s.gammaFind(gp, k); i >= 0 {
			buf = append(buf, ctxDep{kind: depShadow, idx: i, d: depth, val: s.growths[i].id})
		}
	}
	for _, o := range set.obs {
		if v := s.obsValue(pos, gp, present, o); v != obsNone {
			buf = append(buf, ctxDep{kind: depObs, idx: int(o), d: depth, val: v})
			buf = s.replayObs(buf, pos, gp, present, o, depth)
		}
	}
	return buf
}

// obsOutcome encodes the outcome of a lookahead at pos that matched if ok and examined the input up to
// examined.
func obsOutcome(ok bool, pos, examined int64) uint64 {
	v := uint64(examined-pos) << 1
	if ok {
		v |= 1
	}
	return v
}

// The conformance operations that install growths and lookaheads (design 065).
const (
	CtxGrowOp      = "test.grow"
	CtxReseedOp    = "test.reseed"
	CtxUngrowOp    = "test.ungrow"
	CtxLookBeginOp = "test.lookBegin"
	CtxLookEndOp   = "test.lookEnd"
)

// allowsCtx reports whether a call of a ctx operation or of a conformance operation of the growths
// meets its precondition. Calls nest as the call protocol nests them: a frame or a lookahead ends
// before the one that encloses it, and a growth ends after every frame that began inside it.
func (m *Model) allowsCtx(o *exec.Op, args []any) bool {
	s := &m.ctx
	switch o.Name {
	case "ctx.ok", "ctx.value", "ctx.replay":
		return ctxRes(args[0]) != nil
	case "ctx.lookup":
		c := args[0].(*CtxState)
		return c.ctx && !c.pushed
	case "ctx.push":
		return !args[0].(*CtxState).pushed
	case "ctx.pop":
		c := args[0].(*CtxState)
		return c.pushed && c.popped < 0 && len(s.frames) == c.frame
	case "ctx.keep":
		c := args[0].(*CtxState)
		return c.popped == CtxStoreCtx && !c.done
	case "ctx.done":
		c := args[0].(*CtxState)
		return c.popped >= 0 && !c.done
	case CtxGrowOp:
		r := args[0].(RuleRef)
		cr, pos := m.ctxRule(r), m.pos()
		if !cr.Leader || cr.Cycle == 0 {
			return false
		}
		if n := len(s.growths); n > 0 && s.growths[n-1].pos == pos && s.growths[n-1].cycle != cr.Cycle {
			return false
		}
		return s.gammaFind(s.gammaAt(pos, cr.Cycle), growKey{r.Index, args[1].(int64)}) < 0
	case CtxReseedOp:
		return len(s.growths) > 0
	case CtxUngrowOp:
		n := len(s.growths)
		return n > 0 && !slices.ContainsFunc(s.frames, func(f ctxFrame) bool { return f.depth >= n })
	case CtxLookBeginOp:
		return args[0].(int64) >= 0
	case CtxLookEndOp:
		return len(s.looks) > 0 && s.looks[len(s.looks)-1].frame == len(s.frames)
	}
	return true
}

// callCtx runs the operations of the component growth and the conformance operations of its cases.
func (m *Model) callCtx(o *exec.Op, args []any) ([]any, bool) {
	s := &m.ctx
	s.tableUnits, s.depUnits, s.presentUnits, s.keyCost = 0, 0, 0, m.keyCost
	switch o.Name {
	case CtxGrowOp:
		r := args[0].(RuleRef)
		pos := m.pos()
		s.growths = append(s.growths, growthRec{pos: pos, cycle: m.ctxRule(r).Cycle, key: growKey{r.Index, args[1].(int64)},
			bit: m.leaderBit(r), idx: m.leaderIndex(r), id: uint64(args[4].(int64)), seed: &CtxResult{ok: args[2].(bool), end: args[3].(int64), examined: args[3].(int64), from: pos}})
		return nil, true
	case CtxReseedOp:
		g := &s.growths[len(s.growths)-1]
		g.seed = &CtxResult{ok: args[0].(bool), end: args[1].(int64), examined: args[1].(int64), from: g.pos}
		g.id = uint64(args[2].(int64))
		return nil, true
	case CtxUngrowOp:
		i := len(s.growths) - 1
		if n := len(s.tables); n > 0 && s.tables[n-1].pos == s.growths[i].pos && s.tables[n-1].depth == i {
			s.tables = s.tables[:n-1]
		}
		s.growths = s.growths[:i]
		if i == 0 {
			// Nothing outside the growths in progress holds the identities of present sets.
			clear(s.wide)
		}
		return nil, true
	case CtxLookBeginOp:
		pos := m.pos()
		if n := len(s.frames); n > 0 {
			f := s.frames[n-1]
			if g := len(s.growths); f.pos == pos && f.gp < g && s.growths[g-1].pos == pos && s.growths[g-1].cycle == s.growths[f.gp].cycle {
				examined := m.doc.examined
				if m.cfg.Document {
					m.doc.examined = pos
				}
				s.frames = append(s.frames, ctxFrame{pos: pos, gp: f.gp, depth: len(s.growths)})
				s.looks = append(s.looks, lookRec{site: args[0].(int64), examined: examined, frame: len(s.frames)})
				return nil, true
			}
		}
		s.looks = append(s.looks, lookRec{site: -1, frame: len(s.frames)})
		return nil, true
	case CtxLookEndOp:
		l := s.looks[len(s.looks)-1]
		s.looks = s.looks[:len(s.looks)-1]
		if l.site < 0 {
			return nil, true
		}
		f, ds := s.popFrame()
		examined := f.pos
		if m.cfg.Document {
			examined = m.doc.examined
			m.doc.examined = max(l.examined, m.doc.examined)
		}
		if !slices.ContainsFunc(ds, func(d ctxDep) bool { return d.kind == depSeed }) {
			s.mergeDeps(ds)
			return nil, true
		}
		val := obsOutcome(args[0].(bool), f.pos, examined)
		present := s.present(f.gp)
		sd, _ := s.stored(ds)
		s.store(f.pos, f.gp, ctxHead{look: true, id: l.site, present: present}, sd, &ctxSlot{outcome: val})
		out := []ctxDep{{kind: depObs, idx: int(l.site), d: f.depth, val: val}}
		for _, d := range ds {
			if d.kind == depSeed {
				d = ctxDep{kind: depShadow, idx: d.idx, d: f.depth, val: d.val}
			}
			out = append(out, d)
		}
		s.mergeDeps(out)
		return nil, true
	case "ctx.enter":
		r := args[0].(RuleRef)
		c := &CtxState{popped: -1}
		cr := m.ctxRule(r)
		if cr.Cycle == 0 || len(s.growths) == 0 {
			return []any{c, nil}, true
		}
		gp := s.gammaAt(m.pos(), cr.Cycle)
		c.gp = gp
		if gp == len(s.growths) {
			return []any{c, nil}, true
		}
		if cr.Leader {
			if i := s.gammaFind(gp, growKey{r.Index, args[1].(int64)}); i >= 0 {
				s.noteSeedRead(i)
				return []any{c, s.growths[i].seed}, true
			}
		}
		c.ctx = true
		c.present = s.present(gp)
		c.framed = true
		return []any{c, nil}, true
	case "ctx.found":
		return []any{ctxRes(args[0]) != nil}, true
	case "ctx.ok":
		return []any{ctxRes(args[0]).ok}, true
	case "ctx.value":
		return []any{ctxRes(args[0]).value}, true
	case "ctx.replay":
		m.ctxReplay(ctxRes(args[0]), args[1].(bool))
		return nil, true
	case "ctx.allows":
		c, e := args[0].(*CtxState), args[1].(*Entry)
		return []any{!c.ctx || c.present&presentWide == 0 && c.present&^e.present == 0}, true
	case "ctx.active":
		return []any{args[0].(*CtxState).ctx}, true
	case "ctx.lookup":
		c := args[0].(*CtxState)
		if !c.framed {
			return []any{nil}, true
		}
		r := args[1].(RuleRef)
		pos := m.pos()
		slot, _ := s.find(pos, c.gp, ctxHead{id: r.Index, min: args[2].(int64), present: c.present}, args[3].(int64))
		if slot == nil || slot.res.silent && !m.silent() {
			return []any{nil}, true
		}
		s.replayDeps(pos, c.gp, c.present, slot.deps)
		return []any{slot.res}, true
	case "ctx.push":
		c := args[0].(*CtxState)
		c.pushed = true
		if c.framed {
			s.frames = append(s.frames, ctxFrame{pos: m.pos(), gp: c.gp, depth: len(s.growths)})
		}
		c.frame = len(s.frames)
		return nil, true
	case "ctx.pop":
		c := args[0].(*CtxState)
		c.popped = m.ctxPop(c)
		return []any{c.popped}, true
	case "ctx.storeMemo":
		return []any{CtxStoreMemo}, true
	case "ctx.storeCtx":
		return []any{CtxStoreCtx}, true
	case "memo.setPresent":
		args[0].(*Entry).present = args[1].(*CtxState).present
		return nil, true
	case "ctx.keep":
		c, r := args[0].(*CtxState), args[1].(RuleRef)
		res := &CtxResult{ok: args[5].(bool), end: args[6].(int64), silent: args[7].(bool), value: args[8], errs: args[9],
			far: args[10].(int64), kept: args[11].(*ExpKept), examined: m.doc.examined, from: m.doc.low}
		s.store(args[2].(int64), c.gp, ctxHead{id: r.Index, min: args[3].(int64), present: c.present}, c.sd,
			&ctxSlot{res: res, key: args[4].(int64)})
		return nil, true
	case "ctx.done":
		c := args[0].(*CtxState)
		c.done = true
		if c.framed {
			s.mergeDeps(c.ds)
		}
		return nil, true
	}
	return nil, false
}

func (m *Model) ctxPop(c *CtxState) int64 {
	if !c.ctx {
		return CtxStoreMemo
	}
	if !c.framed {
		return CtxStoreNowhere
	}
	_, c.ds = m.ctx.popFrame()
	sd, read := m.ctx.stored(c.ds)
	if !read && c.present&presentWide == 0 {
		return CtxStoreMemo
	}
	c.sd = sd
	return CtxStoreCtx
}

// ctxReplay applies the result r as the call that it records would have: a reuse in the call
// bookkeeping, its expectations unless it is a seed, and on success the position and the recovered
// errors.
func (m *Model) ctxReplay(r *CtxResult, seed bool) {
	if m.cfg.Document {
		m.doc.examined = max(m.doc.examined, r.examined)
		m.doc.low = min(m.doc.low, r.from)
		m.doc.reused++
	}
	if !seed {
		m.replayExpected(r.far, r.kept, true)
	}
	if !r.ok {
		return
	}
	m.regs["pos"] = r.end
	if l, ok := r.errs.(*ErrList); ok && l.n > 0 {
		m.rec = append(slices.Clip(m.rec), l)
	}
}

// replayExpected records the expectations of k at far as the composition that merges a kept set does,
// and returns the units it charges. Unless apply is set it changes nothing and only counts them.
func (m *Model) replayExpected(far int64, k *ExpKept, apply bool) int64 {
	if k == nil {
		return 0
	}
	units := int64(0)
	s := &m.exp
	cur := expRecord{far: s.cur.far, ids: slices.Clone(s.cur.ids)}
	for _, id := range k.ids {
		units++
		if m.silent() || far < cur.far {
			continue
		}
		if far > cur.far {
			cur = expRecord{far: far}
		}
		if m.cfg.NoExpectations {
			if len(cur.ids) == 0 {
				if apply {
					s.write(0)
				}
				cur.ids = append(cur.ids, Placeholder)
			}
			continue
		}
		if slices.Contains(cur.ids, id) {
			units += int64(slices.Index(cur.ids, id)) + 1
			continue
		}
		units += int64(len(cur.ids))
		if apply {
			s.write(len(cur.ids))
		}
		cur.ids = append(cur.ids, id)
	}
	if apply && (cur.far != s.cur.far || len(cur.ids) != len(s.cur.ids)) {
		s.cur = cur
	}
	return units
}

// clone returns a copy of s that operations change apart from s.
func (s *ctxModel) clone() ctxModel {
	c := ctxModel{growths: slices.Clone(s.growths), looks: slices.Clone(s.looks), guard: slices.Clone(s.guard), keyCost: s.keyCost,
		wide: maps.Clone(s.wide)}
	for _, f := range s.frames {
		f.deps = slices.Clone(f.deps)
		c.frames = append(c.frames, f)
	}
	for _, t := range s.tables {
		u := &ctxTable{pos: t.pos, depth: t.depth, heads: map[ctxHead][]*ctxSet{}}
		for h, sets := range t.heads {
			for _, set := range sets {
				x := &ctxSet{seeds: set.seeds, obs: set.obs, vals: map[string][]*ctxSlot{}}
				for k, slots := range set.vals {
					for _, slot := range slots {
						y := *slot
						x.vals[k] = append(x.vals[k], &y)
					}
				}
				u.heads[h] = append(u.heads[h], x)
			}
		}
		c.tables = append(c.tables, u)
	}
	return c
}

// ctxQuantity returns the cost quantity name of a call of the ctx operation o with args, in the state
// before the call, and false for another operation: it runs the call on a copy of the state.
func (m *Model) ctxQuantity(name string, o *exec.Op, args []any) (int64, bool) {
	if o == nil {
		return 0, false
	}
	switch o.Name {
	case "ctx.enter":
		if name != ops.QPresent {
			return 0, false
		}
		saved := m.ctx
		m.ctx = saved.clone()
		defer func() { m.ctx = saved }()
		m.callCtx(o, args)
		return m.ctx.presentUnits, true
	case "ctx.lookup", "ctx.keep", "ctx.pop", "ctx.done":
	case "ctx.replay":
		r := ctxRes(args[0])
		switch name {
		case ops.QMerge:
			if args[1].(bool) {
				return 0, true
			}
			return m.replayExpected(r.far, r.kept, false), true
		case ops.QErrs:
			if l, ok := r.errs.(*ErrList); ok && r.ok {
				return int64(l.n), true
			}
			return 0, true
		}
		return 0, false
	default:
		return 0, false
	}
	saved := m.ctx
	m.ctx = saved.clone()
	defer func() { m.ctx = saved }()
	cs := *args[0].(*CtxState)
	a := slices.Clone(args)
	a[0] = &cs
	if o.Name == "ctx.pop" && cs.framed {
		// The dependencies that the frame recorded are converted for the store.
		return int64(len(m.ctx.frames[len(m.ctx.frames)-1].deps)), name == ops.QDeps
	}
	m.callCtx(o, a)
	switch name {
	case ops.QPresent:
		return m.ctx.presentUnits, true
	case ops.QTable:
		return m.ctx.tableUnits, true
	case ops.QDeps:
		if o.Name == "ctx.keep" {
			// The dependencies that ctx.pop ended are converted for the store.
			return int64(len(cs.ds)), true
		}
		return m.ctx.depUnits, true
	}
	return 0, false
}
