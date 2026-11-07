//go:build !zp_budget_controls_reference

package core

import (
	"math/bits"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/ir/exec/ref"
	"zeroparse.dev/zp/internal/limits"
	"zeroparse.dev/zp/internal/value"
)

// The tests of this file measure the operations of Document reuse at the bounds that they declare in
// internal/ir/exec/ops. The operations of runs and of moving values have no reference model, so the
// conformance suites cannot state their quantities; here each worst case is built by hand, and the
// quantities are worked out from the declaration. A driver of the operations (ResumeRepeat, the
// REPEAT instruction) charges its own steps between them.

func docParser() *Parser {
	return &Parser{Input: Input{Control: NewBudgetControl(nil, limits.Limits{Compute: 1 << 30})}, Memo: NewMemoTable(),
		Runs: map[RunKey]*RunRecord{}}
}

// spent returns the units that f charges on p.
func spent(p *Parser, f func()) uint64 {
	used := p.Control.Used
	f()
	return p.Control.Used - used
}

// TestAdvanceEntryMatchesItsContract runs both printed forms on random entries and logs against the
// contract of memo.splice, written out here edit by edit. The engine reaches only part of these states
// through each form, since an edit decides the entries at its positions at once.
func TestAdvanceEntryMatchesItsContract(t *testing.T) {
	t.Parallel()
	valid, invalid := 0, 0
	for seed := range uint64(20000) {
		rng := rand.New(rand.NewPCG(seed, 66))
		log := make([]DocEdit, rng.IntN(5))
		for i := range log {
			start := rng.IntN(24)
			end := start + rng.IntN(4)
			log[i] = DocEdit{Start: start, End: end, Delta: rng.IntN(5) - 2}
		}
		pos := rng.IntN(24)
		before := MemoEntry{Pos: pos, From: pos - rng.IntN(2), Examined: pos + rng.IntN(6), End: pos + rng.IntN(3), Far: pos + rng.IntN(7),
			Growing: rng.IntN(16) == 0, Positional: rng.IntN(8) == 0, shift: int32(rng.IntN(3)), shifted: rng.IntN(4) == 0,
			Vgen: uint32(rng.IntN(len(log) + 1))}
		if rng.IntN(8) == 0 {
			before.Errs = ErrEntry(&SyntaxError{})
		}
		want, charged, ok := before, uint64(0), !before.Growing
		if ok {
			for _, ed := range log[before.Vgen:] {
				charged++
				if want.Examined <= ed.Start {
					continue
				}
				if want.From < ed.End || want.Positional || !want.Errs.None() {
					ok = false
					break
				}
				want.Pos += ed.Delta
				want.End += ed.Delta
				want.From += ed.Delta
				want.Examined += ed.Delta
				want.Far += ed.Delta
				want.shift += int32(ed.Delta)
				want.shifted = want.shifted || ed.Delta != 0
			}
		}
		if ok {
			want.Vgen = uint32(len(log))
			valid++
		} else {
			want = before
			invalid++
		}
		plain := before
		if got := AdvanceEntry(&plain, log); got != ok || !reflect.DeepEqual(plain, want) {
			t.Fatalf("seed %d: plain form: %v %+v, want %v %+v, log %v", seed, got, plain, ok, want, log)
		}
		p := docParser()
		budget := before
		if got := p.AdvanceEntry(&budget, log); got != ok || !reflect.DeepEqual(budget, want) {
			t.Fatalf("seed %d: budget form: %v %+v, want %v %+v, log %v", seed, got, budget, ok, want, log)
		}
		if p.Control.Used != charged {
			t.Fatalf("seed %d: charged %d, want %d", seed, p.Control.Used, charged)
		}
	}
	if valid == 0 || invalid == 0 {
		t.Fatalf("%d valid and %d invalid cases", valid, invalid)
	}
}

func TestMemoLookupChargesItsPendingEdits(t *testing.T) {
	t.Parallel()
	lookup := func(p *Parser, k MemoKey) bool {
		e, found := p.MemoGet(k)
		if found && e.Vgen != p.Gen && !p.AdvanceEntry(e, p.Edits[:p.Gen]) {
			found = false
		}
		return found
	}
	store := func(p *Parser, k MemoKey, from, examined int) {
		e := p.Memo.Alloc()
		*e = MemoEntry{Ok: true, End: k.Pos + 1, From: from, Examined: examined, Gen: p.Gen, Vgen: p.Gen}
		p.MemoPut(k, e)
	}
	edit := func(p *Parser, start, end, delta int) {
		p.Edits = EditMemo(p.Memo, p.Edits, DocEdit{Start: start, End: end, Delta: delta})
		p.Gen = uint32(len(p.Edits))
	}
	t.Run("an entry that stays valid applies each edit once", func(t *testing.T) {
		p := docParser()
		k := MemoKey{Rule: 1, Pos: 10}
		store(p, k, 10, 12)
		for range 3 {
			edit(p, 20, 21, 0)
		}
		// One unit, one for the entry scanned, and one for each pending edit.
		if got := spent(p, func() {
			if !lookup(p, k) {
				t.Fatal("the entry examined only input before the edits")
			}
		}); got != 1+1+3 {
			t.Fatalf("first lookup charged %d", got)
		}
		if got := spent(p, func() { lookup(p, k) }); got != 1+1 {
			t.Fatalf("second lookup charged %d", got)
		}
	})
	t.Run("a held entry charges its pending edits on every lookup", func(t *testing.T) {
		p := docParser()
		k := MemoKey{Rule: 1, Pos: 10}
		store(p, k, 10, 12)
		// The last pending edit decides, so the lookup applies them all and the entry keeps them.
		edit(p, 20, 21, 0)
		edit(p, 20, 21, 0)
		edit(p, 11, 11, 0)
		for range 2 {
			if got := spent(p, func() {
				if lookup(p, k) {
					t.Fatal("the last edit lies in the examined input")
				}
			}); got != 1+1+3 {
				t.Fatalf("lookup charged %d", got)
			}
		}
	})
	t.Run("a scan counts held entries", func(t *testing.T) {
		p := docParser()
		held, live := MemoKey{Rule: 1, Pos: 10}, MemoKey{Rule: 2, Pos: 10}
		store(p, held, 10, 12)
		store(p, live, 10, 11)
		edit(p, 11, 11, 0)
		// The newer entry heads the chain: the scan for the held entry passes it, and a lookup of a live
		// entry behind a held one passes the held one in the same way.
		if got := spent(p, func() {
			if lookup(p, held) {
				t.Fatal("the edit lies in the examined input")
			}
		}); got != 1+2+1 {
			t.Fatalf("lookup of the held entry charged %d", got)
		}
		behind := MemoKey{Rule: 3, Pos: 10}
		store(p, behind, 10, 11)
		edit(p, 11, 11, 0)
		// The live entry is second now, and the held one third: only the entries the scan reaches count.
		if got := spent(p, func() {
			if !lookup(p, live) {
				t.Fatal("the entry examined only input before the edits")
			}
		}); got != 1+2+2 {
			t.Fatalf("lookup of the live entry charged %d", got)
		}
	})
	t.Run("an edit charges nothing", func(t *testing.T) {
		p := docParser()
		k := MemoKey{Rule: 1, Pos: 10}
		store(p, k, 10, 12)
		if got := spent(p, func() { edit(p, 10, 12, 5) }); got != 0 {
			t.Fatalf("edit charged %d", got)
		}
	})
}

// TestMemoStoreChargesTheSlotsOfTheGap compares the slots that the model of a lazy memo counts with
// what stores charge, after edits that move and widen the gap.
func TestMemoStoreChargesTheSlotsOfTheGap(t *testing.T) {
	t.Parallel()
	pilot := ops.Pilot()
	store, splice := pilot.Op("memo.store"), pilot.Op("memo.splice")
	grown := 0
	for seed := range uint64(200) {
		rng := rand.New(rand.NewPCG(seed, 65))
		model := ref.New(ref.Config{LazyEdits: true})
		p := docParser()
		for range 60 {
			if rng.IntN(4) == 0 {
				start := rng.IntN(5000)
				end := start + rng.IntN(100)
				delta := rng.IntN(3000) - (end - start)
				model.Call(splice, []any{int64(start), int64(end), int64(delta)})
				p.Edits = EditMemo(p.Memo, p.Edits, DocEdit{Start: start, End: end, Delta: delta})
				p.Gen = uint32(len(p.Edits))
				continue
			}
			pos := rng.IntN(20000)
			args := []any{int64(0), int64(pos), int64(0), int64(0), true, int64(pos), false, nil, nil}
			if model.Quantity(ops.QKeyScan, store, args) != 0 {
				continue // the charges of a scan are the subject of the tests of lookups
			}
			want := model.Quantity(ops.QSlotGrowth, store, args)
			got := spent(p, func() {
				e := p.Memo.Alloc()
				*e = MemoEntry{Ok: true, End: pos, Gen: p.Gen, Vgen: p.Gen}
				p.MemoPut(MemoKey{Pos: pos}, e)
			})
			if got != 2+uint64(want) {
				t.Fatalf("seed %d: a store at %d charged %d, the model counts %d slots", seed, pos, got, want)
			}
			if want != 0 {
				grown++
			}
			model.Call(store, args)
		}
	}
	if grown == 0 {
		t.Fatal("no store grew the slots")
	}
}

// TestMemoChargesMatchTheModel runs random stores, edits and lookups on the engine's lazy memo and on
// the model of a lazy memo, and compares what each lookup and store charges with the quantities the model
// counts: the entries a scan passes (ops.QKeyScan) and the edits a lookup applies (ops.QPending). The
// declared cost is a bound, since a lookup stops at the entry it finds and an entry that fails an edit
// stops there; the charge is equal to it where the model is exact:
//
//   - a lookup of a key that no entry holds, live or dropped, and a store of such a key, pass every entry
//     of the position;
//   - a lookup of the entry at the end of its chain applies every pending edit when the entry stays valid,
//     and the one pending edit that drops it when it has one;
//
// except that the model keeps the edits an entry accounts for as of its last store or lookup, while a
// memo applies an edit at once to the entries at the positions the edit replaces and the one after
// them (memo.splice). For these the model counts the edits since, which the engine does not charge, so
// the declaration is a bound for them by design.
func TestMemoChargesMatchTheModel(t *testing.T) {
	t.Parallel()
	pilot := ops.Pilot()
	lookupOp, storeOp, spliceOp, examineOp := pilot.Op("memo.lookup"), pilot.Op("memo.store"), pilot.Op("memo.splice"), pilot.Op("memo.examine")
	const positions, rule = 14, 1
	var absent, valid, held, overstated, stores int
	for seed := range uint64(300) {
		rng := rand.New(rand.NewPCG(seed, 66))
		model := ref.New(ref.Config{LazyEdits: true})
		p := docParser()
		// decided holds the entries that an edit decided at once: the model counts their edits from before.
		decided := map[*MemoEntry]bool{}
		keyOf := func(pos, v int) MemoKey { return MemoKey{Rule: rule, Pos: pos, Env: []any{int64(v)}} }
		argsOf := func(pos, v int) []any { return []any{int64(rule), int64(pos), int64(0), int64(v)} }
		// chain returns the entries of a position in the order a lookup passes them.
		chain := func(pos int) (es []*MemoEntry) {
			table := p.Memo
			i := pos - table.Base
			if i >= table.Gap {
				i += table.GapLen
			}
			if i < 0 || i >= len(table.Slots) {
				return nil
			}
			for e := table.Slots[i]; e != nil; e = e.Next {
				es = append(es, e)
			}
			return es
		}
		for range 80 {
			pos, v := rng.IntN(positions), rng.IntN(2)
			args := argsOf(pos, v)
			switch op := rng.IntN(10); {
			case op < 3:
				start := rng.IntN(positions)
				end := start + rng.IntN(4)
				delta := rng.IntN(5) - (end - start)
				model.Call(spliceOp, []any{int64(start), int64(end), int64(delta)})
				p.Edits = EditMemo(p.Memo, p.Edits, DocEdit{Start: start, End: end, Delta: delta})
				p.Gen = uint32(len(p.Edits))
				p.Memo.Each(func(e *MemoEntry) {
					if e.Vgen == p.Gen {
						decided[e] = true
					}
				})
			case op < 6:
				before := pos > 0 && rng.IntN(3) == 0
				from := pos
				if before {
					from--
				}
				examined := pos + 1 + rng.IntN(6)
				present := slices.ContainsFunc(chain(pos), func(e *MemoEntry) bool { return e.Env[0] == any(int64(v)) })
				want := uint64(2 + model.Quantity(ops.QKeyScan, storeOp, args) + model.Quantity(ops.QSlotGrowth, storeOp, args))
				got := spent(p, func() {
					e := p.Memo.Alloc()
					*e = MemoEntry{Ok: true, End: pos + 1, From: from, Examined: examined, Gen: p.Gen, Vgen: p.Gen}
					p.MemoPut(keyOf(pos, v), e)
					delete(decided, e)
				})
				if got > want || !present && got != want {
					t.Fatalf("seed %d: a store at %d of a key that is %v charged %d, the model counts %d", seed, pos, map[bool]string{false: "absent", true: "present"}[present], got, want)
				}
				stores++
				entry := model.Call(storeOp, append(slices.Clone(args), true, int64(pos+1), false, nil, nil))[0]
				model.Call(examineOp, []any{entry, before, int64(examined)})
			default:
				es := chain(pos)
				at := slices.IndexFunc(es, func(e *MemoEntry) bool { return e.Env[0] == any(int64(v)) })
				var vgen uint32
				if at >= 0 {
					vgen = es[at].Vgen
				}
				want := uint64(1 + model.Quantity(ops.QKeyScan, lookupOp, args) + model.Quantity(ops.QPending, lookupOp, args))
				var found bool
				got := spent(p, func() {
					e, ok := p.MemoGet(keyOf(pos, v))
					found = ok && (e.Vgen == p.Gen || p.AdvanceEntry(e, p.Edits[:p.Gen]))
				})
				if modelFound := model.Call(lookupOp, args)[1].(bool); found != modelFound {
					t.Fatalf("seed %d: a lookup at %d found %v, the model %v", seed, pos, found, modelFound)
				}
				switch {
				case got > want:
					t.Fatalf("seed %d: a lookup at %d charged %d, more than the model counts, %d", seed, pos, got, want)
				case at < 0:
					if got != want {
						t.Fatalf("seed %d: a lookup at %d of an absent key charged %d, the model counts %d", seed, pos, got, want)
					}
					absent++
				case at != len(es)-1:
				case decided[es[at]]:
					if got < want {
						overstated++
					}
				case found:
					if got != want {
						t.Fatalf("seed %d: a lookup at %d of a valid entry charged %d, the model counts %d", seed, pos, got, want)
					}
					valid++
				case p.Gen-vgen == 1:
					if got != want {
						t.Fatalf("seed %d: a lookup at %d of a dropped entry charged %d, the model counts %d", seed, pos, got, want)
					}
					held++
				}
				if found {
					delete(decided, es[at])
				}
			}
		}
	}
	// Each kind of lookup, and the case that is only a bound, occurred.
	if absent == 0 || valid == 0 || held == 0 || overstated == 0 || stores == 0 {
		t.Fatalf("lookups of absent keys %d, of valid entries %d, of dropped entries %d, bounded by design %d, stores %d", absent, valid, held, overstated, stores)
	}
}

// movedBound returns the bound of doc.move for a value of the given nodes and edges, with the given
// edits to replay.
func movedBound(nodes, edges, edits int) uint64 { return uint64(2*(nodes+edges) + edits) }

func TestDocMoveChargesAtItsBound(t *testing.T) {
	t.Parallel()
	entry := func(p *Parser, n *value.Node) *MemoEntry {
		return &MemoEntry{Node: n, shift: 1, shifted: true, Gen: 0}
	}
	moved := func(p *Parser, e *MemoEntry) uint64 { return spent(p, func() { p.moveResult(e) }) }
	edited := func(n int) *Parser {
		p := docParser()
		for range n {
			p.Edits = append(p.Edits, DocEdit{Delta: 1})
		}
		p.Gen = uint32(n)
		return p
	}
	t.Run("an empty root reaches the bound", func(t *testing.T) {
		// One unit for the copy, and one to forget it: the second pass of a copying target.
		p := edited(1)
		n := value.Ptr(value.Data{Start: 3, End: 3})
		if got := moved(p, entry(p, n)); got != movedBound(1, 0, 0) {
			t.Fatalf("charged %d", got)
		}
	})
	t.Run("a tree of non-empty nodes charges one pass", func(t *testing.T) {
		p := edited(1)
		a, b, f := value.Ptr(value.Data{Start: 2, End: 3}), value.Ptr(value.Data{Start: 3, End: 4}), value.Ptr(value.Data{Start: 4, End: 5})
		n := value.Ptr(value.Data{Start: 2, End: 5, Children: []*value.Node{a, b}, Fields: value.Fields{{Name: "f", Value: f}}})
		if got := moved(p, entry(p, n)); got != 7 {
			t.Fatalf("charged %d", got)
		}
	})
	t.Run("a shared empty node charges each reference", func(t *testing.T) {
		// Visits of the root and of three references, three edges, and two copies to forget.
		p := edited(1)
		c := value.Ptr(value.Data{Start: 3, End: 3})
		n := value.Ptr(value.Data{Start: 3, End: 3, Children: []*value.Node{c, c, c}})
		got := moved(p, entry(p, n))
		if got != 4+3+2 || got > movedBound(2, 3, 0) {
			t.Fatalf("charged %d", got)
		}
	})
	t.Run("each generation replays its edits once", func(t *testing.T) {
		// Nodes last moved through other results, after 1, 2 and 2 edits of 4: the replays charge 3 and
		// 2, once for the generation, and forgetting the two generations charges one each.
		p := edited(4)
		a := value.Ptr(value.Data{Start: 2, End: 3, Gen: 1})
		b := value.Ptr(value.Data{Start: 3, End: 4, Gen: 2})
		c := value.Ptr(value.Data{Start: 4, End: 5, Gen: 2})
		n := value.Ptr(value.Data{Start: 2, End: 5, Children: []*value.Node{a, b, c}})
		got := moved(p, entry(p, n))
		if got != 7+3+2+2 || got > movedBound(4, 3, 3+2) {
			t.Fatalf("charged %d", got)
		}
	})
	t.Run("a value with one node per generation stays within the bound", func(t *testing.T) {
		// Each generation has one node and replays one edit: the replay, the node and the unit that
		// forgets the generation use up the node's second unit.
		p := edited(1)
		n := value.Ptr(value.Data{Start: 2, End: 3})
		p.Edits = append(p.Edits, DocEdit{Delta: 1})
		p.Gen = 2
		value.D(n).Gen = 1
		e := entry(p, n)
		if got := moved(p, e); got != 1+1+1 || got > movedBound(1, 0, 1) {
			t.Fatalf("charged %d", got)
		}
	})
	t.Run("the nodes of the entry's generation replay nothing", func(t *testing.T) {
		p := edited(3)
		n := value.Ptr(value.Data{Start: 2, End: 3, Gen: 1})
		e := &MemoEntry{Node: n, shift: 5, shifted: true, Gen: 1}
		if got := moved(p, e); got != 1 || value.D(n).Start != 7 {
			t.Fatalf("charged %d, start %d", got, value.D(n).Start)
		}
		if e.Gen != 3 || e.shift != 0 || e.shifted {
			t.Fatalf("the entry is not current: %+v", e)
		}
	})
	t.Run("a root that another result moved stays", func(t *testing.T) {
		p := edited(1)
		n := value.Ptr(value.Data{Start: 2, End: 3, Gen: 1})
		if got := moved(p, entry(p, n)); got != 0 || value.D(n).Start != 2 {
			t.Fatalf("charged %d, start %d", got, value.D(n).Start)
		}
	})
	t.Run("a shared node moves once", func(t *testing.T) {
		// The root, three edges, and the shared node once.
		p := edited(1)
		c := value.Ptr(value.Data{Start: 3, End: 4})
		n := value.Ptr(value.Data{Start: 2, End: 5, Children: []*value.Node{c, c}, Fields: value.Fields{{Name: "f", Value: c}}})
		if got := moved(p, entry(p, n)); got != 1+3+1 || value.D(c).Start != 4 {
			t.Fatalf("charged %d, start %d", got, value.D(c).Start)
		}
		p = edited(1)
		c = value.Ptr(value.Data{Start: 3, End: 4})
		n = value.Ptr(value.Data{Start: 2, End: 5, Fields: value.Fields{{Name: "f", Value: c}, {Name: "g", Value: c}}})
		if got := moved(p, entry(p, n)); got != 1+2+1 || value.D(c).Start != 4 {
			t.Fatalf("fields: charged %d, start %d", got, value.D(c).Start)
		}
	})
	t.Run("an empty child stays when nothing moves", func(t *testing.T) {
		p := edited(1)
		c := value.Ptr(value.Data{Start: 3, End: 3})
		n := value.Ptr(value.Data{Start: 2, End: 5, Children: []*value.Node{c}})
		e := &MemoEntry{Node: n, shifted: true}
		if got := moved(p, e); got != 2 || value.D(n).Children[0] != c {
			t.Fatalf("charged %d", got)
		}
	})
	t.Run("an empty node charges its fields", func(t *testing.T) {
		// The copy, one field, and forgetting the copy.
		p := edited(1)
		n := value.Ptr(value.Data{Start: 3, End: 3, Fields: value.Fields{{Name: "f", Value: "x"}}})
		if got := moved(p, entry(p, n)); got != 3 || got > movedBound(1, 1, 0) {
			t.Fatalf("charged %d", got)
		}
	})
	t.Run("many copies are dropped, not forgotten one by one", func(t *testing.T) {
		p := edited(1)
		kids := make([]*value.Node, 1025)
		for i := range kids {
			kids[i] = value.Ptr(value.Data{Start: 3, End: 3})
		}
		n := value.Ptr(value.Data{Start: 3, End: 4, Children: kids})
		if got := moved(p, entry(p, n)); got != 1+2*1025 {
			t.Fatalf("charged %d", got)
		}
	})
	t.Run("an entry with no value charges nothing", func(t *testing.T) {
		p := edited(1)
		if got := moved(p, entry(p, nil)); got != 0 {
			t.Fatalf("charged %d", got)
		}
	})
}

// oldRun returns n elements of length 2 from position 0, each with the expectations exp.
func oldRun(n int, exp []ExpID) []RunElem {
	elems := make([]RunElem, n)
	for i := range elems {
		s := 2 * i
		elems[i] = RunElem{Start: s, end: s + 2, From: s, examined: s + 2, far: s + 2, exp: exp}
	}
	return elems
}

// mergeUnits returns what merging exp at far charges, on a parser in the state of p.
func mergeUnits(far int, exp []ExpID) uint64 {
	p := docParser()
	return spent(p, func() { p.MergeExpected(far, exp) })
}

func TestRunOperationsChargeTheirDeclaredWork(t *testing.T) {
	t.Parallel()
	exp := []ExpID{1, 2, 3}
	t.Run("run.start and run.begin charge nothing", func(t *testing.T) {
		p := docParser()
		p.LastRuns = map[RunKey]*RunRecord{{Site: 1}: {Elems: oldRun(1000, nil)}}
		p.Edits, p.Gen = []DocEdit{{Start: 100, End: 101}}, 1
		p.LastRuns[RunKey{Site: 1}].gen = 0
		var r RunState
		if got := spent(p, func() { p.StartRun(&r, 1, 0, -1, true); p.BeginElem(&r) }); got != 0 || len(r.Old) != 1000 {
			t.Fatalf("charged %d, took over %d elements", got, len(r.Old))
		}
	})
	t.Run("run.prefix charges one and the merge", func(t *testing.T) {
		p := docParser()
		r := RunState{Old: oldRun(2, exp), J: -1, max: -1, edit: DocEdit{Start: 2, End: 3}}
		want := 1 + mergeUnits(2, exp)
		if got := spent(p, func() {
			if p.NextPrefix(&r) == nil {
				t.Fatal("the element examined only input before the edit")
			}
		}); got != want || want != 4 {
			t.Fatalf("charged %d, want %d", got, want)
		}
		if got := spent(p, func() {
			if p.NextPrefix(&r) != nil {
				t.Fatal("the element examined the edit")
			}
		}); got != 1 {
			t.Fatalf("end of prefix charged %d", got)
		}
	})
	t.Run("run.resync charges the search and the scan", func(t *testing.T) {
		for _, at := range []int{0, 511, 1022} {
			p := docParser()
			r := RunState{Old: oldRun(1023, nil), J: -1, max: -1, shiftable: true, edit: DocEdit{Start: -1, End: -1}}
			p.Pos = 2 * at
			want := uint64(bits.Len(1023) + 1023 - at)
			if got := spent(p, func() {
				if !p.Resync(&r) {
					t.Fatal("no resynchronization")
				}
			}); got != want {
				t.Fatalf("at %d: charged %d, want %d", at, got, want)
			}
		}
	})
	t.Run("run.tail charges one, the merge and the move", func(t *testing.T) {
		p := docParser()
		p.Edits, p.Gen = []DocEdit{{Start: 0, End: 0, Delta: 2}}, 1
		r := RunState{Old: oldRun(1, exp), J: 0, max: -1, edit: p.Edits[0]}
		v := value.Ptr(value.Data{Start: 0, End: 0})
		r.Old[0].V = v
		// The element moves to 2 through 4, so its expectations merge at 4.
		want := 1 + mergeUnits(4, exp) + movedBound(1, 0, 0)
		if got := spent(p, func() {
			if p.NextTail(&r) == nil {
				t.Fatal("the element is after the edit")
			}
		}); got != want {
			t.Fatalf("charged %d, want %d", got, want)
		}
		if s := value.D(r.Old[0].V).Start; s != 2 {
			t.Fatalf("the empty value is at %d", s)
		}
	})
	t.Run("run.tail replays no edit for the nodes of the run's generation", func(t *testing.T) {
		p := docParser()
		p.Edits, p.Gen = []DocEdit{{Start: 0, End: 0}, {Start: 0, End: 0, Delta: 2}}, 2
		r := RunState{Old: oldRun(1, nil), J: 0, max: -1, edit: p.Edits[1]}
		r.Old[0].V = value.Ptr(value.Data{Start: 0, End: 2, Gen: 1})
		if got := spent(p, func() { p.NextTail(&r) }); got != 1+1 {
			t.Fatalf("charged %d", got)
		}
	})
	t.Run("run.tail leaves a value that another result moved", func(t *testing.T) {
		p := docParser()
		p.Edits, p.Gen = []DocEdit{{Start: 0, End: 0, Delta: 2}}, 1
		r := RunState{Old: oldRun(1, nil), J: 0, max: -1, edit: p.Edits[0]}
		r.Old[0].V = value.Ptr(value.Data{Start: 2, End: 4, Gen: 1})
		if got := spent(p, func() { p.NextTail(&r) }); got != 1 || value.D(r.Old[0].V).Start != 2 {
			t.Fatalf("charged %d", got)
		}
	})
	t.Run("run.end charges the merge and the growth", func(t *testing.T) {
		p := docParser()
		var r RunState
		r.J = -1
		r.Mid = make([]RunElem, 64)
		p.BeginElem(&r)
		p.Exp = append(p.Exp, 1, 2, 3)
		p.Pos = 2
		// Keeping copies the three expectations, and merging them charges three.
		got := spent(p, func() { p.EndElem(&r, true, nil) })
		if want := uint64(64 + 3 + 3); got != want {
			t.Fatalf("charged %d, want %d", got, want)
		}
		p.BeginElem(&r)
		if got := spent(p, func() { p.EndElem(&r, true, nil) }); got != 0 {
			t.Fatalf("no growth, no expectations: charged %d", got)
		}
	})
	t.Run("run.finish charges the elements of the run", func(t *testing.T) {
		p := docParser()
		r := RunState{Old: oldRun(100, nil), K: 40, J: 60, T: 30, Mid: oldRun(5, nil), Post: oldRun(7, nil), Count: 82}
		if got := spent(p, func() { p.FinishRun(&r) }); got != 40+5+30+7 {
			t.Fatalf("charged %d", got)
		}
		r = RunState{Old: oldRun(100, nil), K: MinRecorded - 1, J: -1, Count: MinRecorded - 1}
		if got := spent(p, func() { p.FinishRun(&r) }); got != 0 {
			t.Fatalf("a short run charged %d", got)
		}
	})
}
