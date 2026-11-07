package core

import "zeroparse.dev/zp/internal/ir/exec/ops"

// rawValueEqual preserves the interface equality of memo keys and bindings.
// In particular, a typed nil pointer differs from a nil interface. Action
// ValueEqual has different nil semantics and must not be used for these keys.
func (p *Parser) rawValueEqual(l, r any) bool {
	if s, ok := l.(string); ok {
		t, ok := r.(string)
		return ok && len(s) == len(t) && p.CompareText(s, t) == 0
	}
	return l == r
}

// identEqual compares two key values for a lookup of a rule that only a Document parse memoizes: strings
// are equal if they are the same bytes of the same memory, which costs a constant; a value that is not
// identical is a miss, and a miss is sound, since the rule is then evaluated and that work counts.
func (p *Parser) identEqual(l, r any) bool {
	if s, ok := l.(string); ok {
		t, ok := r.(string)
		return ok && sameData(s, t)
	}
	return l == r
}

// memoKeyIdentical is memoKeyEqual under identEqual.
func (p *Parser) memoKeyIdentical(l, r []any) bool {
	if len(l) != len(r) {
		return false
	}
	for i, v := range l {
		p.Spend()
		if !p.identEqual(v, r[i]) {
			return false
		}
	}
	return true
}

// keyMatches compares a memo key under the lookup that ident selects. A lookup of a rule that a parse
// from scratch memoizes too pays for the comparison with every entry of this parse that the walk compares,
// as a parse from scratch does, so it counts in a region of Document work as well: it is bounded by the
// size of the key and by the entries of the walk. The comparison with an entry that is not current in this
// parse (current reports it; see MemoEntry.Seen) is Document work, since a parse from scratch has no such
// entry, and stays inside the region; countKeyEqual charges it when such an entry answers. A lookup of a
// rule that only a Document memoizes compares by identity, which costs a constant per variable, and counts
// there too only for the entry that answers.
func (p *Parser) keyMatches(l, r []any, ident, current bool) bool {
	if ident {
		return p.memoKeyIdentical(l, r)
	}
	if !current {
		return p.memoKeyEqual(l, r)
	}
	held := p.Control.FreeHold()
	defer p.Control.FreeResume(held)
	return p.memoKeyEqual(l, r)
}

// current reports whether e was stored or answered a lookup in this parse: in a Document, an entry that
// an earlier parse stored and this one has not touched is not.
func (p *Parser) current(e *MemoEntry) bool {
	return e.Seen == p.Gen
}

// lagged reports whether a lookup treats e as a miss because it lags behind more than ops.PendingEditLimit
// edits (AdvanceEntry). A Document drops such an entry where a walk meets it (unlinkLagged): the entries of
// earlier texts that it retains, and so the work of a walk that compares them, stay bounded by that many
// edits.
func (p *Parser) lagged(e *MemoEntry) bool {
	return !e.Growing && p.Gen-e.Vgen > ops.PendingEditLimit
}

// unlinkLagged removes from the chain at pos, starting at the link, the entries that lag behind (lagged)
// among the next limit entries that do not, or among all of them if limit is negative, and reports where the
// walk ends: the link before the first entry it did not visit. Each entry removed costs the step of a walk.
// The index of the position, if there is one, no longer matches the chain and is dropped.
func (p *Parser) unlinkLagged(pos int, link **MemoEntry, limit int) **MemoEntry {
	t := p.Memo
	for kept := 0; *link != nil && kept != limit; {
		e := *link
		if !p.lagged(e) {
			link, kept = &e.Next, kept+1
			continue
		}
		p.Spend()
		*link = e.Next
		if t.Index[pos] != nil {
			t.dropIndex(pos)
		}
		t.Retire(e)
	}
	return link
}

// countKeyEqual charges, in a region of Document work, what comparing the key of an entry that was not
// current in this parse costs once the entry answers a lookup: a parse from scratch compares it too.
func (p *Parser) countKeyEqual(l, r []any) {
	if p.Control.FreeHold() {
		p.memoKeyEqual(l, r)
		p.Control.FreeResume(true)
	}
}

// countKeyMatch charges what comparing the key of the entry that answers a lookup of a rule that only a
// Document memoizes costs, in a region of Document work: a constant per variable.
func (p *Parser) countKeyMatch(l, r []any) {
	if p.Control.FreeHold() {
		p.memoKeyIdentical(l, r)
		p.Control.FreeResume(true)
	}
}

func (p *Parser) memoKeyEqual(l, r []any) bool {
	if len(l) != len(r) {
		return false
	}
	for i, v := range l {
		p.Spend()
		if !p.rawValueEqual(v, r[i]) {
			return false
		}
	}
	return true
}

// bindControlled is Bind under budget controls. It charges one unit for the lookup of the old
// value and one for each node that a rebind copies, which are the levels+1 nodes on the path to
// the slot; comparing the values is charged by rawValueEqual.
func (p *Parser) bindControlled(slot int, val any) {
	p.Spend()
	if p.rawValueEqual(p.Env.Get(slot, p.envLevels), val) {
		return
	}
	for l := 0; l <= p.envLevels; l++ {
		p.Spend()
	}
	// Saved environments are immutable; only publish a completed path.
	p.Env = p.Env.Set(slot, p.envLevels, val)
	p.ChargeValueBytes(int(uint64(p.envLevels+1) * envNodeBytes))
}

// MemoGet looks up the entry for k. The lookup counts one unit, the steps of the walk of the chain at the
// position and the comparison of the keys. In a Document parse the chain holds the entries of rules that a
// parse from scratch does not memoize and those of earlier texts, so the steps are Document work there; the
// comparison of the key with each entry of this parse that the walk compares it with counts, as it does in a
// parse from scratch, and it is bounded by the size of the key and the entries of the walk. The walk drops
// the entries that lag behind too many edits (lagged) before it compares anything with them.
func (p *Parser) MemoGet(k MemoKey) (*MemoEntry, bool) {
	if !BudgetControlsEnabled || p.Control == nil {
		return p.Memo.Get(k)
	}
	p.Spend()
	ident := k.DocOnly // see MemoKey.DocOnly
	if p.MemoAll {
		defer p.FreeEnd(p.FreeBegin(true))
	}
	t := p.Memo
	i := k.Pos - t.Base
	if i >= t.Gap {
		i += t.GapLen
	}
	if i < 0 || i >= len(t.Slots) {
		return nil, false
	}
	scan := memoScan
	if ident {
		scan = docOnlyScan
	}
	link := p.unlinkLagged(k.Pos, &t.Slots[i], scan)
	for e := t.Slots[i]; e != nil && e != *link; e = e.Next {
		p.Spend()
		if e.Rule == int32(k.Rule) && e.Min == int32(k.Min) && e.missingSilent == k.MissingSilent && p.keyMatches(e.Env, k.Env, ident, p.current(e)) {
			if p.MemoAll {
				if ident {
					p.countKeyMatch(e.Env, k.Env)
				} else if !p.current(e) {
					p.countKeyEqual(e.Env, k.Env)
				}
			}
			if p.Control.DefaultCompute {
				e.Seen = p.Gen // an explicit limit counts every comparison, and needs no stamp
			}
			return e, true
		}
	}
	if *link == nil || ident {
		return nil, false
	}
	p.unlinkLagged(k.Pos, link, -1)
	if *link == nil {
		return nil, false
	}
	ix := t.indexAt(k.Pos, t.Slots[i], p.chargeMemoHash)
	h := p.countedHash(int32(k.Rule), int32(k.Min), k.MissingSilent, k.Env)
	_, x := ix.probe(h, func(x *MemoEntry) bool {
		return x.Rule == int32(k.Rule) && x.Min == int32(k.Min) && x.missingSilent == k.MissingSilent && p.keyMatches(x.Env, k.Env, false, p.current(x))
	})
	if x != nil && p.MemoAll && !p.current(x) {
		p.countKeyEqual(x.Env, k.Env)
	}
	if x != nil && p.Control.DefaultCompute {
		x.Seen = p.Gen
	}
	return x, x != nil
}

// hashMemoKey hashes a memo key for the index of a long chain. The hash costs
// one step plus one per variable of the key, and the bytes of string values
// as a value comparison would, so a lookup through the index costs the size of
// the key, not the length of the chain.
func (p *Parser) hashMemoKey(rule, min int32, silent bool, env []any) uint64 {
	h, bytes := memoKeyHash(rule, min, silent, env)
	for range 1 + len(env) {
		p.Spend()
	}
	if bytes > 0 {
		p.SpendValue(bytes)
	}
	return h
}

// countedHash is hashMemoKey for the key of a lookup, which counts in a region of Document work: a
// parse from scratch hashes the key too, once per lookup.
func (p *Parser) countedHash(rule, min int32, silent bool, env []any) uint64 {
	h := p.Control.FreeHold()
	defer p.Control.FreeResume(h)
	return p.hashMemoKey(rule, min, silent, env)
}

// chargeMemoHash is the cost of hashing e while its position's index is built.
func (p *Parser) chargeMemoHash(e *MemoEntry) {
	_, bytes := memoKeyHash(e.Rule, e.Min, e.missingSilent, e.Env)
	for range 1 + len(e.Env) {
		p.Spend()
	}
	if bytes > 0 {
		p.SpendValue(bytes)
	}
}

// MemoPut stores e for k. The store counts one unit; the walk of the chain at the position and the
// growth of the table count as MemoGet's walk does, and are Document work in a Document parse.
func (p *Parser) MemoPut(k MemoKey, e *MemoEntry) {
	if !BudgetControlsEnabled || p.Control == nil {
		p.Memo.Put(k, e)
		return
	}
	p.Spend()
	ident := k.DocOnly
	if p.MemoAll {
		defer p.FreeEnd(p.FreeBegin(true))
	}
	t := p.Memo
	i := k.Pos - t.Base
	if i < 0 {
		return
	}
	if i >= t.Gap {
		i += t.GapLen
	}
	slots := t.Slots
	if i >= len(slots) {
		p.growthSpend(len(slots), i+1)
		// The slot array is charged by its length, not its capacity, so the count does not
		// depend on the capacity an earlier parse left.
		t.MemBytes += uint64(i+1-len(slots)) * memoSlotBytes
		if i < cap(slots) {
			slots = slots[:i+1]
		} else {
			grown := make([]*MemoEntry, i+1, max(2*cap(slots), i+1, 64))
			copy(grown, slots)
			slots = grown
		}
	}
	link := &slots[i]
	var replaced *MemoEntry
	same := func(x *MemoEntry) bool {
		return x.Rule == int32(k.Rule) && x.Min == int32(k.Min) && x.missingSilent == k.MissingSilent && p.keyMatches(x.Env, k.Env, ident, p.current(x))
	}
	scan := memoScan
	if ident {
		scan = docOnlyScan
	}
	end := p.unlinkLagged(k.Pos, link, scan)
	for x := *link; x != nil && link != end; link, x = &x.Next, x.Next {
		p.Spend()
		if same(x) {
			replaced = x
			break
		}
	}
	var ixSlot uint64
	var ix memoIndex
	if replaced == nil && *link != nil && !ident {
		// A long chain: the index finds the entry to replace.
		p.unlinkLagged(k.Pos, link, -1)
	}
	if replaced == nil && *link != nil && !ident {
		ix = t.indexAt(k.Pos, slots[i], p.chargeMemoHash)
		h := p.countedHash(int32(k.Rule), int32(k.Min), k.MissingSilent, k.Env)
		ixSlot, replaced = ix.probe(h, same)
		if replaced != nil {
			// Walk to the link of the entry; the walk is charged like any chain walk.
			link = &slots[i]
			for *link != replaced {
				p.Spend()
				link = &(*link).Next
			}
		}
	}
	if replaced != nil && ix == nil {
		// The entry replaced among the first of a long chain is indexed too.
		if ix = t.Index[k.Pos]; ix != nil {
			h := p.hashMemoKey(replaced.Rule, replaced.Min, replaced.missingSilent, replaced.Env)
			ixSlot, _ = ix.probe(h, func(y *MemoEntry) bool { return y == replaced })
		}
	}
	if replaced != nil && replaced != e && ident && p.MemoAll {
		p.countKeyMatch(replaced.Env, k.Env)
	}
	p.Spend()
	// No checkpoint may separate the link swap from retiring its old entry.
	// In particular, do not publish a partially copied slot array or key.
	t.Slots = slots
	e.Rule, e.Min, e.Pos, e.Env = int32(k.Rule), int32(k.Min), k.Pos, k.Env
	e.Seen = p.Gen
	e.missingSilent = k.MissingSilent
	if ix != nil {
		n := len(ix)
		ix[ixSlot] = e
		t.IndexBytes += uint64(len(ix)-n) * memoIndexEntryBytes
	}
	if replaced != nil {
		e.Next = replaced.Next
		*link = e
		if replaced != e {
			t.Retire(replaced)
		}
		return
	}
	e.Next = slots[i]
	slots[i] = e
}

func (p *Parser) memoFirstCall(pos, r int) bool {
	if !BudgetControlsEnabled || p.Control == nil {
		return p.Memo.firstCall(pos, r)
	}
	p.Spend()
	t := p.Memo
	if t.Calls == nil || !t.Calls[r].Eager {
		i := pos*t.Stride + r
		w := i >> 6
		if w >= len(t.Seen) {
			n := len(t.Seen) + max(w+1-len(t.Seen), len(t.Seen), 64)
			// The new words are the spend, whether they come from spare capacity or an allocation.
			p.chargeValueBytes(n - len(t.Seen))
			seen := t.Seen
			if n <= cap(seen) {
				seen = seen[:n]
				clear(seen[len(t.Seen):])
			} else {
				seen = make([]uint64, n)
				copy(seen, t.Seen)
			}
			// The original estimator now needs only scalar operations. Keep
			// its discount and eager thresholds, and publish after all copies.
			t.Seen = seen
		}
	}
	return t.firstCall(pos, r)
}
