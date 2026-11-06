// Package ref holds the reference models of the parser VM's state components (design 065): naive
// Go implementations of the contracts in internal/ir/exec/ops, written for clarity, never printed
// or optimized. The conformance suite runs operation sequences on a model and on an implementation
// and compares them. A model also checks the precondition of each call, so that randomized
// sequences stay within what compositions guarantee, and computes the abstract quantities that
// cost bounds are written in.
package ref

import (
	"fmt"
	"math"
	"math/bits"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
)

// Config sets up the components of a model.
type Config struct {
	// Rules is the number of rules whose memoization is deferred (component seen).
	Rules int
	// Limit is the compute limit, 0 for none.
	Limit uint64
	// MaxDepth is the nesting limit.
	MaxDepth int64
	// LazyEdits models a memo that is a gap buffer and applies edits to an entry when it finds it, as
	// the engine's is: it keeps the entries memo.splice removes until a lookup or a store replaces
	// them, and its slots hold the gap as well. The charges of memo.lookup and memo.store count both.
	LazyEdits bool
}

// Model is the reference model of the pilot components. It implements exec.Machine.
type Model struct {
	cfg Config

	// memo
	entries map[memoKey]*Entry
	at      map[int64]int // live entries per position
	covered int64         // one more than the largest position stored at, at least floor
	floor   int64         // the memo keeps no entries before floor (memo.prune)
	// held are the entries that memo.splice removed and that a memo with Config.LazyEdits still holds,
	// with the edits before each was stored or last found. They count in the charges of memo.lookup
	// and memo.store (ops.QKeyScan, ops.QPending); no lookup finds them.
	held  map[memoKey]int64
	edits int64 // the number of memo.splice calls
	// slots, gap and gapLen are the layout of a memo with Config.LazyEdits: the length of its slots,
	// and the position and the length of the gap in them.
	slots, gap, gapLen int64

	// seen
	seen  map[[2]int64]bool
	pages map[[2]int64]bool
	rules []seenCounts

	// budget
	used uint64

	// depth
	depth int64
}

type slotKey struct{ rule, pos, level int64 }

type memoKey struct {
	slotKey
	vals int64
}

// Entry is a memo entry of the model.
type Entry struct {
	key    memoKey
	ok     bool
	end    int64
	silent bool
	value  any
	errs   any
	// recorded reports a record of memo.examine; before and examined are the input it recorded, and
	// pinned reports memo.pin.
	examined                 int64
	recorded, before, pinned bool
	// pruned reports an entry that memo.prune or memo.splice made invalid, or that memo.store replaced.
	pruned bool
	// applied is the number of edits since which the entry was stored or last found: a target that
	// applies edits to an entry when it finds it has applied those before it.
	applied int64
}

// Pos returns the position of the entry's key.
func (e *Entry) Pos() int64 { return e.key.pos }

// from returns the start of the input recorded with e.
func (e *Entry) from() int64 {
	if e.before {
		return e.key.pos - 1
	}
	return e.key.pos
}

type seenCounts struct {
	calls, repeats int32
	eager          bool
}

// New returns a model in the state at the start of a parse.
func New(cfg Config) *Model {
	return &Model{cfg: cfg, entries: map[memoKey]*Entry{}, at: map[int64]int{},
		seen: map[[2]int64]bool{}, pages: map[[2]int64]bool{}, rules: make([]seenCounts, cfg.Rules)}
}

// Values returns the model's Values handle for the conformance identifier id: the identifier itself.
// Two Values are equal when their identifiers are; each stands for a single value.
func (m *Model) Values(id int64) any { return id }

func key(args []any) memoKey {
	return memoKey{slotKey{args[0].(int64), args[1].(int64), args[2].(int64)}, args[3].(int64)}
}

// Allows reports whether a call of o with args meets its precondition in the current state.
func (m *Model) Allows(o *exec.Op, args []any) bool {
	switch o.Name {
	case "memo.store":
		return args[5].(int64) >= args[1].(int64) && args[1].(int64) >= 0
	case "memo.prune":
		return args[0].(int64) >= m.floor
	case "memo.ok", "memo.end", "memo.silent", "memo.value", "memo.errs", "memo.examined", "memo.before":
		return !args[0].(*Entry).pruned
	case "memo.examine":
		e := args[0].(*Entry)
		return !e.pruned && args[2].(int64) >= e.key.pos
	case "memo.pin":
		return !args[0].(*Entry).pruned
	case "memo.splice":
		start, end, delta := args[0].(int64), args[1].(int64), args[2].(int64)
		return 0 <= start && start <= end && start <= end+delta
	case "seen.first":
		r, pos := args[0].(int64), args[1].(int64)
		return r >= 0 && r < int64(m.cfg.Rules) && pos >= 0
	case "depth.exit":
		return m.depth > 0
	}
	return true
}

// Quantity returns the cost quantity name for a call of o with args, in the state before the call.
func (m *Model) Quantity(name string, o *exec.Op, args []any) int64 {
	switch name {
	case ops.QKeyScan:
		// Every Values of a conformance case is one value.
		n := int64(m.at[args[1].(int64)])
		for k := range m.held {
			if k.pos == args[1].(int64) {
				n++
			}
		}
		return 2 * n
	case ops.QPending:
		if !m.cfg.LazyEdits {
			return 0
		}
		k := key(args)
		if e := m.entries[k]; e != nil {
			return m.edits - e.applied
		}
		if applied, ok := m.held[k]; ok {
			return m.edits - applied
		}
		return 0
	case ops.QSlotGrowth:
		// Positions count from the floor; a store before it keeps nothing.
		pos, covered := args[1].(int64)-m.floor, m.covered-m.floor
		if m.cfg.LazyEdits {
			pos, covered = m.slot(args[1].(int64)), m.slots
		}
		if covered > 0 && pos >= covered && bits.Len64(uint64(pos+1)) > bits.Len64(uint64(covered)) {
			return covered
		}
		return 0
	case ops.QPageIndex:
		return args[1].(int64) >> 10
	case ops.QPages:
		return int64(len(m.pages))
	case ops.QSeenIndex:
		return (args[1].(int64)*int64(m.cfg.Rules) + args[0].(int64)) / 64
	case ops.QN:
		return int64(min(args[0].(uint64), math.MaxInt64))
	case ops.QPruned:
		n := 0
		for at, k := range m.at {
			if at < args[0].(int64) {
				n += k
			}
		}
		return int64(n)
	case ops.QCovered:
		return max(m.covered-m.floor, 0)
	}
	panic("ref: unknown quantity " + name)
}

// Call implements exec.Machine.
func (m *Model) Call(o *exec.Op, args []any) []any {
	switch o.Name {
	case "memo.lookup":
		e := m.entries[key(args)]
		if e != nil {
			e.applied = m.edits
		}
		return []any{e, e != nil}
	case "memo.store":
		return []any{m.put(&Entry{key: key(args), ok: args[4].(bool), end: args[5].(int64), silent: args[6].(bool), value: args[7], errs: args[8], applied: m.edits})}
	case "memo.prune":
		m.prune(args[0].(int64))
		return nil
	case "memo.floor":
		return []any{m.floor}
	case "memo.examine":
		e := args[0].(*Entry)
		e.recorded, e.before, e.examined, e.pinned = true, args[1].(bool), args[2].(int64), false
		return nil
	case "memo.examined":
		return []any{args[0].(*Entry).examined}
	case "memo.before":
		return []any{args[0].(*Entry).before}
	case "memo.pin":
		args[0].(*Entry).pinned = true
		return nil
	case "memo.splice":
		m.splice(args[0].(int64), args[1].(int64), args[2].(int64))
		return nil
	case "memo.ok":
		return []any{args[0].(*Entry).ok}
	case "memo.end":
		return []any{args[0].(*Entry).end}
	case "memo.silent":
		return []any{args[0].(*Entry).silent}
	case "memo.value":
		return []any{args[0].(*Entry).value}
	case "memo.errs":
		return []any{args[0].(*Entry).errs}
	case "seen.first":
		return []any{m.first(args[0].(int64), args[1].(int64))}
	case "budget.charge":
		n := args[0].(uint64)
		if m.Exceeds(n) {
			panic(exec.Interrupt{Reason: "limit"})
		}
		m.charge(n)
		return nil
	case "budget.used":
		return []any{m.used}
	case "depth.enter":
		m.depth++
		if m.depth > m.cfg.MaxDepth {
			panic(exec.Interrupt{Reason: "nesting"})
		}
		return nil
	case "depth.exit":
		m.depth--
		return nil
	case "depth.current":
		return []any{m.depth}
	}
	panic(fmt.Sprintf("ref: no model of %s", o.Name))
}

// Dead reports whether the handle h is no longer valid: an entry that memo.prune or memo.splice
// removed, or that memo.store replaced. An implementation may reuse what a dead handle stood for.
func (m *Model) Dead(h any) bool {
	e, ok := h.(*Entry)
	return ok && e.pruned
}

// Exceeds reports whether charging n units would exceed the compute limit.
func (m *Model) Exceeds(n uint64) bool {
	l := m.cfg.Limit
	return l != 0 && (m.used > l || n > l-m.used)
}

// Absorb adds the n units that an implementation charged inside another operation to the budget
// and reports whether they exceed the limit (then the units used are unchanged). The conformance
// suite calls it, because the charges inside operations are the implementation's: within the
// operation's cost bound, they join the budget that budget.charge and budget.used see.
func (m *Model) Absorb(n uint64) (exceeded bool) {
	if m.Exceeds(n) {
		return true
	}
	m.charge(n)
	return false
}

func (m *Model) charge(n uint64) {
	if math.MaxUint64-m.used < n {
		m.used = math.MaxUint64
	} else {
		m.used += n
	}
}

// put stores e under its key, replacing the entry there; before the floor it keeps nothing.
func (m *Model) put(e *Entry) *Entry {
	if e.key.pos < m.floor {
		return e
	}
	if old := m.entries[e.key]; old == nil {
		m.at[e.key.pos]++
	} else if old != e {
		// The replaced entry is dead: an implementation may reuse it for a later entry.
		old.pruned = true
	}
	delete(m.held, e.key)
	m.entries[e.key] = e
	m.covered = max(m.covered, e.key.pos+1)
	m.slots = max(m.slots, m.slot(e.key.pos)+1)
	return e
}

// slot returns the index in the slots of a memo with Config.LazyEdits of position pos.
func (m *Model) slot(pos int64) int64 {
	if pos >= m.gap {
		return pos + m.gapLen
	}
	return pos
}

// moveGap applies an edit to the layout of a memo with Config.LazyEdits: the gap takes the positions
// the edit replaces, and gives the positions of the new text, widened with room for later insertions
// when it holds too few.
func (m *Model) moveGap(start, end, delta int64) {
	m.slots = max(m.slots, end+m.gapLen+1)
	m.gapLen += end - start
	n := end - start + delta
	if n > m.gapLen {
		more := n - m.gapLen + max(1024, m.slots/16)
		m.slots += more
		m.gapLen += more
	}
	m.gap, m.gapLen = start+n, m.gapLen-n
}

// prune removes the entries before pos and makes pos the floor.
func (m *Model) prune(pos int64) {
	for k := range m.entries {
		if k.pos < pos {
			m.entries[k].pruned = true
			delete(m.entries, k)
			delete(m.at, k.pos)
		}
	}
	m.floor = pos
	m.covered = max(m.covered, pos)
}

// splice applies an edit: it keeps the entries before start, moves the movable entries after end
// by delta, as copies, and removes the others; every entry from before it is dead.
func (m *Model) splice(start, end, delta int64) {
	m.edits++
	m.moveGap(start, end, delta)
	// hold keeps a removed entry as a memo with Config.LazyEdits does: the positions from start to end
	// are decided at once, and the others move with the input.
	hold := func(k memoKey, applied int64, held map[memoKey]int64) {
		switch {
		case !m.cfg.LazyEdits:
			return
		case k.pos < start:
		case k.pos > end:
			k.pos += delta
		default:
			return
		}
		held[k] = applied
	}
	old, oldHeld := m.entries, m.held
	m.entries, m.at, m.covered, m.held = map[memoKey]*Entry{}, map[int64]int{}, m.floor, map[memoKey]int64{}
	for k, applied := range oldHeld {
		hold(k, applied, m.held)
	}
	for _, e := range old {
		e.pruned = true
		c := *e
		c.pruned = false
		switch {
		case !e.recorded:
			hold(e.key, e.applied, m.held)
			continue
		case e.examined <= start:
		case e.from() >= end && !e.pinned && e.errs == nil:
			c.key.pos += delta
			c.end += delta
			c.examined += delta
		default:
			hold(e.key, e.applied, m.held)
			continue
		}
		m.put(&c)
	}
}

func (m *Model) first(rule, pos int64) bool {
	c := &m.rules[rule]
	if c.eager {
		return false
	}
	calls := uint32(c.calls) + 1
	if calls == 1<<31 {
		calls >>= 1
		c.repeats >>= 1
	}
	c.calls = int32(calls)
	k := [2]int64{rule, pos}
	m.pages[[2]int64{rule, pos >> 10}] = true
	if !m.seen[k] {
		m.seen[k] = true
		return true
	}
	c.repeats++
	if c.repeats > c.calls>>4 {
		c.eager = true
	}
	return false
}
