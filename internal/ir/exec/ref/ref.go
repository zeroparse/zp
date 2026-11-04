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
}

// Model is the reference model of the pilot components. It implements exec.Machine.
type Model struct {
	cfg Config

	// memo
	entries map[memoKey]*Entry
	at      map[int64]int // live entries per position
	covered int64         // one more than the largest position stored at, at least floor
	floor   int64         // the memo keeps no entries before floor (memo.prune)

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
	// examined is the end recorded by memo.examine, 0 for none.
	examined int64
	// pruned reports an entry that memo.prune or memo.splice made invalid.
	pruned bool
}

// Pos returns the position of the entry's key.
func (e *Entry) Pos() int64 { return e.key.pos }

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
	case "memo.ok", "memo.end", "memo.silent", "memo.value", "memo.errs", "memo.examined":
		return !args[0].(*Entry).pruned
	case "memo.examine":
		e := args[0].(*Entry)
		return !e.pruned && args[1].(int64) > e.key.pos
	case "memo.splice":
		return args[0].(int64) >= 0
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
		return 2 * int64(m.at[args[1].(int64)])
	case ops.QSlotGrowth:
		// Positions count from the floor; a store before it keeps nothing.
		pos, covered := args[1].(int64)-m.floor, m.covered-m.floor
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
	}
	panic("ref: unknown quantity " + name)
}

// Call implements exec.Machine.
func (m *Model) Call(o *exec.Op, args []any) []any {
	switch o.Name {
	case "memo.lookup":
		e := m.entries[key(args)]
		return []any{e, e != nil}
	case "memo.store":
		return []any{m.put(&Entry{key: key(args), ok: args[4].(bool), end: args[5].(int64), silent: args[6].(bool), value: args[7], errs: args[8]})}
	case "memo.prune":
		m.prune(args[0].(int64))
		return nil
	case "memo.floor":
		return []any{m.floor}
	case "memo.examine":
		args[0].(*Entry).examined = args[1].(int64)
		return nil
	case "memo.examined":
		return []any{args[0].(*Entry).examined}
	case "memo.splice":
		m.splice(args[0].(int64))
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

// Dead reports whether the handle h is no longer valid: an entry that memo.prune removed. An
// implementation may reuse what a dead handle stood for.
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
	if m.entries[e.key] == nil {
		m.at[e.key.pos]++
	}
	m.entries[e.key] = e
	m.covered = max(m.covered, e.key.pos+1)
	return e
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

// splice keeps the entries whose examined end is recorded and at most start, as copies; every
// entry from before it is dead.
func (m *Model) splice(start int64) {
	old := m.entries
	m.entries, m.at, m.covered = map[memoKey]*Entry{}, map[int64]int{}, m.floor
	for _, e := range old {
		e.pruned = true
		if e.examined > 0 && e.examined <= start {
			c := *e
			c.pruned = false
			m.put(&c)
		}
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
