package ref

import (
	"fmt"
	"slices"

	"zeroparse.dev/zp/internal/ir/exec"
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

// maxCtxDeps is the number of dependencies that a dependency set indexes: a result with more is not
// kept.
const maxCtxDeps = 6

// maxLeaderBits is the number of leaders of a cycle that have a bit in a present set: a context with
// another leader cannot be told from others.
const maxLeaderBits = 32

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

// CtxState is the model's Ctx handle: the context state of a memoized call.
type CtxState struct {
	gp                 int
	present            uint64
	ctx, known, framed bool
	// ds are the dependencies that ctx.pop ended, and sd those that ctx.keep stores.
	ds []ctxDep
	sd []ctxSDep
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
	site int64
	high int64
}

// ctxModel is the state of the component growth.
type ctxModel struct {
	growths []growthRec
	frames  []ctxFrame
	tables  []*ctxTable
	looks   []lookRec
	guard   []int64
	// units counts the table and dependency work of the operation in progress.
	tableUnits, depUnits int64
	// merge is the units that the merge of expectations of the last ctx.replay charged.
	merge int64
}

func (m *Model) ctxRule(r RuleRef) CtxRule {
	if int(r.Index) < len(m.cfg.CtxRules) {
		return m.cfg.CtxRules[r.Index]
	}
	return CtxRule{}
}

// leaderBit returns the bit of the leader r in the present sets of its cycle: leaders are numbered in
// the order of Config.CtxRules within each cycle.
func (m *Model) leaderBit(r RuleRef) uint64 {
	c := m.ctxRule(r)
	if !c.Leader || c.Cycle == 0 {
		return 0
	}
	k := 0
	for i, x := range m.cfg.CtxRules {
		if int64(i) == r.Index {
			break
		}
		if x.Leader && x.Cycle == c.Cycle {
			k++
		}
	}
	if k >= maxLeaderBits {
		return 0
	}
	return 1 << k
}

func (m *Model) pos() int64 { return m.regs["pos"].(int64) }

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

func (s *ctxModel) present(gp int) (uint64, bool) {
	var p uint64
	for _, g := range s.growths[gp:] {
		if g.bit == 0 {
			return 0, false
		}
		p |= g.bit
	}
	return p, true
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
	if len(seeds)+len(obs) > maxCtxDeps {
		return
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

// callCtx runs the operations of the component growth and the conformance operations of its cases.
func (m *Model) callCtx(o *exec.Op, args []any) ([]any, bool) {
	s := &m.ctx
	s.tableUnits, s.depUnits = 0, 0
	switch o.Name {
	case CtxGrowOp:
		r := args[0].(RuleRef)
		pos := m.pos()
		s.growths = append(s.growths, growthRec{pos: pos, cycle: m.ctxRule(r).Cycle, key: growKey{r.Index, args[1].(int64)},
			bit: m.leaderBit(r), id: uint64(args[4].(int64)), seed: &CtxResult{ok: args[2].(bool), end: args[3].(int64), examined: args[3].(int64), from: pos}})
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
		return nil, true
	case CtxLookBeginOp:
		pos := m.pos()
		if n := len(s.frames); n > 0 {
			f := s.frames[n-1]
			if g := len(s.growths); f.pos == pos && f.gp < g && s.growths[g-1].pos == pos && s.growths[g-1].cycle == s.growths[f.gp].cycle {
				s.looks = append(s.looks, lookRec{site: args[0].(int64), high: m.doc.high})
				if m.cfg.Document {
					m.doc.high = pos
				}
				s.frames = append(s.frames, ctxFrame{pos: pos, gp: f.gp, depth: len(s.growths)})
				return nil, true
			}
		}
		s.looks = append(s.looks, lookRec{site: -1})
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
			examined = m.doc.high
			m.doc.high = max(l.high, m.doc.high)
		}
		if !slices.ContainsFunc(ds, func(d ctxDep) bool { return d.kind == depSeed }) {
			s.mergeDeps(ds)
			return nil, true
		}
		val := obsOutcome(args[0].(bool), f.pos, examined)
		present, known := s.present(f.gp)
		sd, _ := s.stored(ds)
		if known {
			s.store(f.pos, f.gp, ctxHead{look: true, id: l.site, present: present}, sd, &ctxSlot{outcome: val})
		}
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
		c := &CtxState{}
		cr := m.ctxRule(r)
		if cr.Cycle == 0 || len(s.growths) == 0 {
			return []any{c, (*CtxResult)(nil)}, true
		}
		gp := s.gammaAt(m.pos(), cr.Cycle)
		c.gp = gp
		if gp == len(s.growths) {
			return []any{c, (*CtxResult)(nil)}, true
		}
		if cr.Leader {
			if i := s.gammaFind(gp, growKey{r.Index, args[1].(int64)}); i >= 0 {
				s.noteSeedRead(i)
				return []any{c, s.growths[i].seed}, true
			}
		}
		c.ctx = true
		c.present, c.known = s.present(gp)
		c.framed = c.known
		return []any{c, (*CtxResult)(nil)}, true
	case "ctx.found":
		return []any{args[0].(*CtxResult) != nil}, true
	case "ctx.ok":
		return []any{args[0].(*CtxResult).ok}, true
	case "ctx.value":
		return []any{args[0].(*CtxResult).value}, true
	case "ctx.replay":
		m.ctxReplay(args[0].(*CtxResult), args[1].(bool))
		return nil, true
	case "ctx.allows":
		c, e := args[0].(*CtxState), args[1].(*Entry)
		return []any{!c.ctx || c.known && c.present&^e.present == 0}, true
	case "ctx.active":
		return []any{args[0].(*CtxState).ctx}, true
	case "ctx.lookup":
		c := args[0].(*CtxState)
		if !c.framed {
			return []any{(*CtxResult)(nil)}, true
		}
		r := args[1].(RuleRef)
		pos := m.pos()
		slot, _ := s.find(pos, c.gp, ctxHead{id: r.Index, min: args[2].(int64), present: c.present}, args[3].(int64))
		if slot == nil || slot.res.silent && !m.regs["silent"].(bool) {
			return []any{(*CtxResult)(nil)}, true
		}
		s.replayDeps(pos, c.gp, c.present, slot.deps)
		return []any{slot.res}, true
	case "ctx.push":
		if c := args[0].(*CtxState); c.framed {
			s.frames = append(s.frames, ctxFrame{pos: m.pos(), gp: c.gp, depth: len(s.growths)})
		}
		return nil, true
	case "ctx.pop":
		c := args[0].(*CtxState)
		if !c.ctx {
			return []any{CtxStoreMemo}, true
		}
		if !c.framed {
			return []any{CtxStoreNowhere}, true
		}
		_, c.ds = s.popFrame()
		sd, read := s.stored(c.ds)
		if !read {
			return []any{CtxStoreMemo}, true
		}
		c.sd = sd
		return []any{CtxStoreCtx}, true
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
			far: args[10].(int64), kept: args[11].(*ExpKept), examined: m.doc.high, from: m.doc.low}
		s.store(args[2].(int64), c.gp, ctxHead{id: r.Index, min: args[3].(int64), present: c.present}, c.sd,
			&ctxSlot{res: res, key: args[4].(int64)})
		return nil, true
	case "ctx.done":
		if c := args[0].(*CtxState); c.framed {
			s.mergeDeps(c.ds)
		}
		return nil, true
	}
	return nil, false
}

// ctxReplay applies the result r as the call that it records would have: a reuse in the call
// bookkeeping, its expectations unless it is a seed, and on success the position and the recovered
// errors.
func (m *Model) ctxReplay(r *CtxResult, seed bool) {
	if m.cfg.Document {
		m.doc.high = max(m.doc.high, r.examined)
		m.doc.low = min(m.doc.low, r.from)
		m.doc.reused++
	}
	if !seed {
		m.ctx.merge = m.replayExpected(r.far, r.kept)
	}
	if !r.ok {
		return
	}
	m.regs["pos"] = r.end
	if l, ok := r.errs.(*ErrList); ok && l.n > 0 {
		m.rec = append(slices.Clip(m.rec), l)
	}
}

// replayExpected records the expectations of k at far as the composition that merges a kept set does, and returns the
// units it charges.
func (m *Model) replayExpected(far int64, k *ExpKept) int64 {
	if k == nil {
		return 0
	}
	units := int64(0)
	s := &m.exp
	for _, id := range k.ids {
		units++
		if m.cfg.Document {
			m.doc.high = max(m.doc.high, far+1)
		}
		if m.regs["silent"].(bool) || far < s.cur.far {
			continue
		}
		if far > s.cur.far {
			s.cur = expRecord{far: far}
		}
		if m.cfg.NoExpectations {
			if len(s.cur.ids) == 0 {
				s.write(0)
				s.cur.ids = append(slices.Clip(s.cur.ids), Placeholder)
			}
			continue
		}
		if slices.Contains(s.cur.ids, id) {
			units += int64(slices.Index(s.cur.ids, id)) + 1
			continue
		}
		units += int64(len(s.cur.ids))
		s.write(len(s.cur.ids))
		s.cur.ids = append(slices.Clip(s.cur.ids), id)
	}
	return units
}
