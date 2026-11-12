package core

import "zeroparse.dev/zp/internal/value"

// The engine's implementations of the operations of the call protocol that its printed
// compositions call (call_exec.go, printed from internal/ir/exec/execcore): the call state, the context
// state of a memoized call, the bit of an expectation in the filter of the current record (expBit) and
// the results that a context holds.

// expBit returns the bit of id in the filter of the expectations of the current record (ExpBits),
// which exp.maybe reads.
func expBit(id ExpID) uint64 {
	return uint64(1) << (uint32(id) * 0x9e3779b1 >> 26)
}

// KeyVals are the values of a memo key beyond its rule, position and level: the values of the
// variables the rule reads, and whether the call is in a lookahead that inserts no missing token,
// which has its own results.
type KeyVals struct {
	Env     []any
	Missing bool
}

// keyVals returns the values of the memo key of a call of rule r at the position.
func (p *Parser) keyVals(r *Rule) KeyVals {
	kv := KeyVals{Missing: p.Missing != nil && r.Missing && p.Silent > 0}
	if len(r.Vars) > 0 {
		kv.Env = p.memoEnv(r)
	}
	return kv
}

// GrowKey returns the key of the growth of leader r at binding level min at the position. A growth
// is told apart by its rule, level and missing-token state: the values of variables do not take
// part.
func (p *Parser) GrowKey(r *Rule, min int) MemoKey {
	return MemoKey{Rule: r.ID, Pos: p.Pos, Min: min, MissingSilent: p.Missing != nil && r.Missing && p.Silent > 0}
}

// memoized reports whether the results of rule r, which leads no left-recursive cycle, are memoized
// in the parse (Memoizes).
func (p *Parser) memoized(r *Rule) bool {
	return r.Memo && (!r.Transient || p.MemoAll) && !(p.Missing != nil && r.Missing)
}

// docOnlyMemo reports whether the parse memoizes rule r and a parse from scratch does not: a transient
// rule in a Document parse. The memo protocol of such a rule is Document work.
func (p *Parser) docOnlyMemo(r *Rule) bool {
	return p.MemoAll && r.Memo && r.Transient && !r.Leader && !(p.Missing != nil && r.Missing)
}

// The iterative machine writes the call protocol out on its hot path. In a Document parse (MemoAll)
// it calls the forms below, which run the memo protocol as Document work; a parse from scratch runs
// the plain steps without a call.

// DocCallSpend charges the unit of a call (the charge of Call): Document work for a rule whose
// plain body a parse from scratch calls directly.
func (p *Parser) DocCallSpend(r *Rule) {
	m := p.FreeBegin(r.Plain)
	p.Spend()
	p.FreeEnd(m)
}

// DirectCallDoc reports whether the call of r runs its body unmemoized, as the test of Call does:
// the rule is not memoized, or its memoization is deferred. The test is Document work for a rule that
// only a Document parse memoizes.
func (p *Parser) DirectCallDoc(r *Rule) bool {
	m := p.FreeBegin(p.docOnlyMemo(r))
	direct := !p.Memoizes(r) || p.FirstCall(r)
	p.FreeEnd(m)
	return direct
}

// CallBeginDoc is CallBegin, as work of the Document for a rule that only a Document parse memoizes.
func (p *Parser) CallBeginDoc(r *Rule, min int, st *CallState) (*value.Node, bool, bool) {
	m := p.FreeBegin(p.docOnlyMemo(r))
	v, ok, hit := p.CallBegin(r, min, st)
	p.FreeEnd(m)
	return v, ok, hit
}

// CallEndDoc is CallEnd, as work of the Document for a rule that only a Document parse memoizes.
func (p *Parser) CallEndDoc(r *Rule, st *CallState, v *value.Node, ok bool) (*value.Node, bool) {
	m := p.FreeBegin(p.docOnlyMemo(r))
	v, ok = p.CallEnd(r, st, v, ok)
	p.FreeEnd(m)
	return v, ok
}

// tracing reports whether a call takes the traced path: in a traced parse, except for the call
// that tracedCall makes back, whose mark it clears.
func (p *Parser) tracing() bool {
	if !p.Tr.direct {
		return true
	}
	p.Tr.direct = false
	return false
}

// CallState is the state of a memoized call between CallBegin and CallEnd, which its caller holds.
type CallState struct {
	start, min int
	key        KeyVals
	ctx        CtxState
	rec        int
	exp        ExpMark
	// high and low are the examined range of the enclosing call; missing is the mark of the
	// missing-token record.
	high, low int
	missing   int
}

// CtxState is the context state of a memoized call (see context_memo.go).
type CtxState struct {
	// gp is the index in Gamma of the first growth of the rule's cycle at the position. ctx reports
	// a call in a context, with the present set present (known); framed reports that the call
	// records its dependencies.
	gp         int
	present    uint64
	ctx, known bool
	framed     bool
	// ds are the dependencies that the call's evaluation recorded, and sd those that a result kept
	// in its context stores (ctxPop).
	ds []dep
	sd []sdep
}

// Where ctxPop sends the result of a call.
const (
	storeNowhere = iota
	storeMemo
	storeCtx
)

// ctxHit is a result that the context of a call holds for it: the seed of the call's rule growing
// at the position, or a result kept in the table of the context, or the last result of a growth. e
// is nil for none.
type ctxHit struct {
	e    *MemoEntry
	seed bool
}

// ctxEnter begins the memoized call of rule r, a rule of a left-recursive cycle, at binding level
// min at the position, in a parse where leaders grow: it fills c and returns the seed of r if r is
// a leader that grows here, recording the read of the seed.
func (p *Parser) ctxEnter(c *CtxState, r *Rule, min int) ctxHit {
	gp := p.gammaAt(p.Pos, r.Cycle)
	c.gp = gp
	if gp == len(p.Gamma) {
		return ctxHit{}
	}
	if r.Leader {
		// A leader that grows here reads its seed, whatever its variable values.
		if i := p.gammaFind(gp, r.ID, min, p.Missing != nil && r.Missing && p.Silent > 0); i >= 0 {
			p.noteSeedRead(i)
			return ctxHit{e: p.Gamma[i].seed, seed: true}
		}
	}
	c.ctx = true
	c.present, c.known = p.present(gp)
	c.framed = c.known && !p.strictCtx()
	return ctxHit{}
}

// ctxAllows reports whether the call in a context with the state c may reuse the memo entry e:
// when every leader that grows here grew when e was evaluated, and did not matter to it.
func (p *Parser) ctxAllows(c *CtxState, e *MemoEntry) bool {
	return ContextMemoMutation == CtxIgnorePresent || c.known && !p.strictCtx() && c.present&^e.Present == 0
}

// ctxLookup returns the result kept in the context of the call of rule r at binding level min with
// the key values kv at the position, which the call may reuse, and records what it depends on for
// the enclosing call; otherwise none.
func (p *Parser) ctxLookup(c *CtxState, r *Rule, min int, kv KeyVals) ctxHit {
	if !c.framed {
		return ctxHit{}
	}
	h := ctxHeadKey{kind: ctxCall, id: int32(r.ID), min: int32(min), ms: kv.Missing, present: c.present}
	s := p.ctxFind(p.Pos, c.gp, h, kv.Env)
	if s == nil || !p.reusable(s.e.silent) {
		return ctxHit{}
	}
	p.seedRead(c.gp)
	p.replayDeps(p.Pos, c.gp, c.present, s.deps)
	return ctxHit{e: s.e}
}

// ctxReplay applies the result h as the call that it records would have; a seed adds no
// expectations and is never moved.
func (p *Parser) ctxReplay(h ctxHit, seed bool) {
	e := h.e
	if !seed {
		p.replayEntry(e)
		return
	}
	p.touch(e.Examined)
	p.lw = Min2(p.lw, e.From)
	p.Stats.Reused++
	if !e.Ok {
		return
	}
	p.Pos = e.End
	if p.Missing == nil {
		p.recAdd(e.Errs)
	} else {
		p.ReplayMissing(p.MissingSeedEvents(e), e.Errs)
	}
}

// ctxPop ends recording the dependencies of the evaluation of a call in a context and returns
// where its result goes: storeMemo for a result that read no seed, storeCtx for one that did, and
// storeNowhere for a call whose context records no dependencies. A result that goes nowhere is
// dropped: the last result of a leader's growth is not retained.
func (p *Parser) ctxPop(c *CtxState) int {
	if !c.framed {
		p.grown = nil
		return storeNowhere
	}
	_, c.ds = p.popFrame()
	sd, read := p.stored(c.ds)
	if !read {
		return storeMemo
	}
	c.sd = sd
	return storeCtx
}

// ctxKeep keeps the entry e of the call of rule r at pos, binding level min and key values kv in
// the table of its context, with the dependencies that ctxPop ended.
func (p *Parser) ctxKeep(c *CtxState, r *Rule, pos, min int, kv KeyVals, e *MemoEntry) {
	p.ctxStore(pos, c.gp, ctxHeadKey{kind: ctxCall, id: int32(r.ID), min: int32(min), ms: kv.Missing, present: c.present},
		c.sd, &ctxSlot{e: e, env: kv.Env})
	if ContextMemoMutation == CtxKeepSeedResults {
		p.keptSeedResults = append(p.keptSeedResults, keptSeedResult{MemoKey{Rule: r.ID, Pos: pos, Min: min, Env: kv.Env, MissingSilent: kv.Missing}, e})
	}
}

// growEnd ends the growth g of leader r and returns its last result, which CallEnd stores.
func (p *Parser) growEnd(r *Rule, g *GrowState) ctxHit {
	p.GrowEnd(r, g)
	return ctxHit{e: p.grown}
}
