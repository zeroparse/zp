package runtime

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

// The generated Go runtime is the first implementation of the parser VM's pilot operations under
// conformance (design 065). The binding calls the runtime's own functions; where an operation is
// inline code of a composition, it does what that code does: memo.store allocates an entry and
// stores it with memoPut as the call does. A stream parse also provides the pruning of its memo.

type rtMachine struct {
	p     *Parser
	vals  map[int64][]any
	rules []*Rule
	// metered reports a parse with budget controls, whose charges the suite checks.
	metered bool
	env     rtEnv
}

// rtEnv binds the variable environment of a parser, which the node and the typed runtime share.
type rtEnv struct {
	rules   []*Rule
	nextKey int64
}

// setUp gives p the variables of cfg, through the setup of a grammar.
func (e *rtEnv) setUp(p *Parser, cfg ref.Config) {
	if len(cfg.Vars) == 0 {
		p.g = &grammar{}
		return
	}
	g, names, vals := &grammar{}, make([]string, len(cfg.Vars)), make([]any, len(cfg.Vars))
	for i, id := range cfg.Vars {
		names[i], vals[i] = fmt.Sprint("v", i), ref.Value(id)
	}
	for _, slots := range cfg.RuleVars {
		r := &Rule{}
		for _, slot := range slots {
			r.Vars = append(r.Vars, names[slot])
		}
		g.rules = append(g.rules, r)
		e.rules = append(e.rules, r)
	}
	g.SetVars(names, vals)
	p.g, p.Env, p.envDepth = g, g.initEnv, g.envDepth
}

// call runs an operation of the environment, and reports false for any other.
func (e *rtEnv) call(p *Parser, vals map[int64][]any, o *exec.Op, args []any) ([]any, bool) {
	switch o.Name {
	case "env.lookup":
		return []any{p.variable(int(args[0].(int64)))}, true
	case "env.bind":
		p.bind(int(args[0].(int64)), args[1])
		return nil, true
	case "env.scope":
		return []any{p.Env}, true
	case "env.restore":
		p.Env = args[0].(*Env)
		return nil, true
	case "env.key":
		e.nextKey--
		vals[e.nextKey] = p.memoEnv(args[0].(*Rule))
		return []any{e.nextKey}, true
	}
	return nil, false
}

func newRTMachine(cfg ref.Config, controlled, sparse, stream bool) conform.Impl {
	p := &Parser{MaxDepth: int(cfg.MaxDepth)}
	if stream {
		p.Stream = &streamInput{}
	}
	p.memo.stride, p.memo.sparse = cfg.Rules, sparse
	m := &rtMachine{p: p, vals: map[int64][]any{}, metered: controlled}
	for i := range cfg.Rules {
		m.rules = append(m.rules, &Rule{Seen: i})
	}
	m.env.setUp(p, cfg)
	if controlled {
		limit := cfg.Limit
		if limit == 0 {
			limit = Unlimited
		}
		p.Control = newBudgetControl(nil, Limits{Compute: limit})
	}
	if m.metered {
		return meteredRT{m}
	}
	return m
}

type meteredRT struct{ *rtMachine }

func (m meteredRT) Used() uint64 { return m.p.Control.budgetUsed() }

func (m *rtMachine) Datum(id int64) any { return ref.Value(id) }

func (m *rtMachine) Rule(index int64) any { return m.env.rules[index] }

func (m *rtMachine) Values(id int64) any {
	if m.vals[id] == nil {
		m.vals[id] = []any{id}
	}
	return id
}

func (m *rtMachine) Call(o *exec.Op, args []any) (res []any) {
	defer func() {
		if x := recover(); x != nil {
			f, ok := x.(fatal)
			if !ok {
				panic(x)
			}
			if _, limit := f.err.(*LimitError); limit {
				panic(exec.Interrupt{Reason: "limit"})
			}
			panic(exec.Interrupt{Reason: "nesting"})
		}
	}()
	p := m.p
	if res, ok := m.env.call(p, m.vals, o, args); ok {
		return res
	}
	i := func(k int) int { return int(args[k].(int64)) }
	key := func() (int, int, int, []any) { return i(0), i(1), i(2), m.vals[args[3].(int64)] }
	switch o.Name {
	case "memo.lookup":
		rule, pos, level, env := key()
		e := p.memoGet(rule, pos, level, env)
		return []any{e, e != nil}
	case "memo.store":
		rule, pos, level, env := key()
		e := p.memo.alloc()
		*e = memoEntry{ok: args[4].(bool), end: i(5), silent: args[6].(bool), errs: recoveredErrors(args[8])}
		p.memoPut(rule, pos, level, env, e)
		return []any{e}
	case "memo.ok":
		return []any{args[0].(*memoEntry).ok}
	case "memo.end":
		return []any{int64(args[0].(*memoEntry).end)}
	case "memo.silent":
		return []any{args[0].(*memoEntry).silent}
	case "memo.examine":
		e := args[0].(*memoEntry)
		e.from, e.examined, e.pinned = examinedFromOf(args[1].(bool)), i(2), false
		return nil
	case "memo.examined":
		return []any{int64(args[0].(*memoEntry).examined)}
	case "memo.before":
		return []any{args[0].(*memoEntry).from == fromBefore}
	case "memo.pin":
		args[0].(*memoEntry).pinned = true
		return nil
	case "memo.splice":
		// A Document edit: the Document's memo is the table of its next parse.
		p.memo.splice(documentEdit{i(0), i(1), i(2)})
		return nil
	case "memo.prune":
		p.pruneStreamMemo(i(0))
		return nil
	case "memo.floor":
		return []any{int64(p.Stream.memoBase)}
	case "seen.first":
		p.Pos = i(1)
		return []any{p.FirstCall(m.rules[i(0)])}
	case "budget.charge":
		p.Control.Charge(args[0].(uint64))
		return nil
	case "budget.used":
		return []any{p.Control.budgetUsed()}
	}
	panic("runtime binding: no operation " + o.Name)
}

var (
	rtMemoOps   = []string{"memo.lookup", "memo.store", "memo.ok", "memo.end", "memo.silent", "seen.first"}
	rtBudgetOps = []string{"budget.charge", "budget.used"}
	rtStreamOps = []string{"memo.prune", "memo.floor"}
	rtDocOps    = []string{"memo.examine", "memo.examined", "memo.before", "memo.pin", "memo.splice"}
	// rtDocEditOps are the operations of the randomized cases that edit a Document's memo, with
	// nothing between a store and the edit that follows it that would remove the entry: the cases
	// then check which entries an edit keeps, moves and removes.
	rtDocEditOps = []string{"memo.lookup", "memo.store", "memo.end", "memo.examine", "memo.examined", "memo.before", "memo.pin", "memo.splice"}
	// rtEnvOps are the operations of the variable environment, with the memo operations that keys of
	// the environment reach.
	rtEnvOps = []string{"env.lookup", "env.bind", "env.scope", "env.restore", "env.key", "memo.lookup", "memo.store"}
	// rtEnvExact are the environment operations whose charges equal their declared cost.
	rtEnvExact = []string{"env.lookup", "env.bind"}
)

func rtSuites() []conform.Suite {
	var suites []conform.Suite
	for _, controlled := range []bool{false, true} {
		for _, sparse := range []bool{false, true} {
			// A parse reads a string, a stream or a Document. Streams and Documents memoize on the
			// first call: seen.first is not part of their parse.
			for _, mode := range []string{"string", "stream", "document"} {
				ops := append([]string(nil), rtMemoOps...)
				switch mode {
				case "stream":
					ops = append(slices.DeleteFunc(ops, func(o string) bool { return o == "seen.first" }), rtStreamOps...)
				case "document":
					ops = append(slices.DeleteFunc(ops, func(o string) bool { return o == "seen.first" }), rtDocOps...)
				}
				if controlled {
					ops = append(ops, rtBudgetOps...)
				}
				suites = append(suites, conform.Suite{
					Name: fmt.Sprintf("generated Go runtime (controlled=%v, sparse seen=%v, %s)", controlled, sparse, mode),
					Ops:  ops,
					New:  func(c ref.Config) conform.Impl { return newRTMachine(c, controlled, sparse, mode == "stream") },
				})
			}
		}
	}
	suites = append(suites,
		conform.Suite{Name: "generated Go runtime (document edits)", Ops: rtDocEditOps,
			New: func(c ref.Config) conform.Impl { return newRTMachine(c, false, false, false) }},
		conform.Suite{Name: "generated Go typed runtime (document edits)", Ops: rtDocEditOps,
			New: func(c ref.Config) conform.Impl { return newTypedMachine(c, false) }})
	for _, controlled := range []bool{false, true} {
		ops := []string{"memo.lookup", "memo.store", "memo.ok", "memo.end", "memo.silent"}
		ops = append(ops, rtDocOps...)
		if controlled {
			ops = append(ops, rtBudgetOps...)
		}
		suites = append(suites, conform.Suite{
			Name: fmt.Sprintf("generated Go typed runtime (controlled=%v)", controlled),
			Ops:  ops,
			New:  func(c ref.Config) conform.Impl { return newTypedMachine(c, controlled) },
		})
		ops = slices.Clone(rtEnvOps)
		if controlled {
			ops = append(ops, rtBudgetOps...)
		}
		suites = append(suites, conform.Suite{
			Name:  fmt.Sprintf("generated Go runtime environment (controlled=%v)", controlled),
			Ops:   ops,
			Exact: rtEnvExact,
			New:   func(c ref.Config) conform.Impl { return newRTMachine(c, controlled, false, false) },
		}, conform.Suite{
			Name:  fmt.Sprintf("generated Go typed runtime environment (controlled=%v)", controlled),
			Ops:   ops,
			Exact: rtEnvExact,
			New:   func(c ref.Config) conform.Impl { return newTypedMachine(c, controlled) },
		})
	}
	return suites
}

// recoveredErrors returns the errors that an entry of the runtime holds for the recovered errors of
// an argument of memo.store: none for nil, and one error for the errors that conform.ErrsOp makes.
func recoveredErrors(arg any) []*SyntaxError {
	if arg == nil {
		return nil
	}
	return []*SyntaxError{{}}
}

// typedMachine binds the memo operations of the typed runtime, with the Document records of a typed
// Document (typedDocumentState) beside the entries.
type typedMachine struct {
	p    *TParser
	doc  *typedDocumentState
	vals map[int64][]any
	env  rtEnv
}

func newTypedMachine(cfg ref.Config, controlled bool) conform.Impl {
	p := &TParser{Parser: &Parser{MaxDepth: int(cfg.MaxDepth)}}
	m := &typedMachine{p: p, doc: &typedDocumentState{meta: map[*tmemoEntry]typedDocumentMeta{}}, vals: map[int64][]any{}}
	m.env.setUp(p.Parser, cfg)
	if controlled {
		limit := cfg.Limit
		if limit == 0 {
			limit = Unlimited
		}
		p.Control = newBudgetControl(nil, Limits{Compute: limit})
		return meteredTyped{m}
	}
	return m
}

type meteredTyped struct{ *typedMachine }

func (m meteredTyped) Used() uint64 { return m.p.Control.budgetUsed() }

func (m *typedMachine) Datum(id int64) any { return ref.Value(id) }

func (m *typedMachine) Rule(index int64) any { return m.env.rules[index] }

func (m *typedMachine) Values(id int64) any {
	if m.vals[id] == nil {
		m.vals[id] = []any{id}
	}
	return id
}

func (m *typedMachine) Call(o *exec.Op, args []any) (res []any) {
	defer func() {
		if x := recover(); x != nil {
			f, ok := x.(fatal)
			if !ok {
				panic(x)
			}
			if _, limit := f.err.(*LimitError); limit {
				panic(exec.Interrupt{Reason: "limit"})
			}
			panic(exec.Interrupt{Reason: "nesting"})
		}
	}()
	p := m.p
	if res, ok := m.env.call(p.Parser, m.vals, o, args); ok {
		return res
	}
	i := func(k int) int { return int(args[k].(int64)) }
	switch o.Name {
	case "memo.lookup":
		e := p.memoGet(i(0), i(1), i(2), m.vals[args[3].(int64)])
		return []any{e, e != nil}
	case "memo.store":
		e := p.memoSlab.one()
		*e = tmemoEntry{ok: args[4].(bool), end: i(5), silent: args[6].(bool), errs: recoveredErrors(args[8])}
		p.memoPut(i(0), i(1), i(2), m.vals[args[3].(int64)], e)
		return []any{e}
	case "memo.ok":
		return []any{args[0].(*tmemoEntry).ok}
	case "memo.end":
		return []any{int64(args[0].(*tmemoEntry).end)}
	case "memo.silent":
		return []any{args[0].(*tmemoEntry).silent}
	case "memo.examine":
		m.doc.meta[args[0].(*tmemoEntry)] = typedDocumentMeta{examined: i(2), recorded: true, before: args[1].(bool)}
		return nil
	case "memo.examined":
		return []any{int64(m.doc.meta[args[0].(*tmemoEntry)].examined)}
	case "memo.before":
		return []any{m.doc.meta[args[0].(*tmemoEntry)].before}
	case "memo.pin":
		m.doc.pin(args[0].(*tmemoEntry))
		return nil
	case "memo.splice":
		// A typed Document edit between parses: the next parse starts from the kept chains, without
		// the hash indexes of the last.
		m.doc.slots = p.memoSlots
		m.doc.splice(documentEdit{i(0), i(1), i(2)})
		p.memoSlots, p.memoIndexes, p.memoIndexBytes = m.doc.slots, nil, 0
		return nil
	case "budget.charge":
		p.Control.Charge(args[0].(uint64))
		return nil
	case "budget.used":
		return []any{p.Control.budgetUsed()}
	}
	panic("typed runtime binding: no operation " + o.Name)
}

// conformanceFailures runs the handwritten cases and seeds randomized cases of n calls on s, and
// returns the failures and the number of cases that ran.
func conformanceFailures(s conform.Suite, seeds, n int) (fails []error, ran int) {
	run := func(c conform.Case) {
		skipped, err := conform.Run(s, c)
		if !skipped {
			ran++
		}
		if err != nil {
			fails = append(fails, err)
		}
	}
	for _, c := range conform.Cases() {
		run(c)
	}
	for seed := range seeds {
		// One case in four has a limit that the case may reach; the others run to their end.
		var limit uint64
		if seed%4 == 0 {
			limit = uint64(200 + seed)
		}
		rng := rand.New(rand.NewPCG(uint64(seed), 65))
		run(conform.Random(fmt.Sprint("random_", seed), rng, s.Ops, n, ref.Config{Rules: 3 + seed%2*130, Limit: limit, MaxDepth: 4, Vars: conform.RandomVars(seed), RuleVars: conform.RandomRuleVars}))
	}
	return fails, ran
}

func TestExecConformance(t *testing.T) {
	seeds := 300
	if testing.Short() {
		seeds = 30
	}
	for _, s := range rtSuites() {
		t.Run(s.Name, func(t *testing.T) {
			fails, ran := conformanceFailures(s, seeds, 150)
			if ran < seeds {
				t.Fatalf("only %d cases ran", ran)
			}
			for _, err := range fails {
				t.Error(err)
			}
		})
	}
}

// TestExecConformanceDetectsBrokenRuntime is the negative control: deliberately broken variants of
// the runtime's operations must fail conformance.
func TestExecConformanceDetectsBrokenRuntime(t *testing.T) {
	type broken struct {
		op         string
		call       func(m *rtMachine, args []any) []any
		controlled bool
		stream     bool
		// document runs the cases of the document suite, which store entries without editing them
		// back into reach, instead of the cases that edit a Document's memo.
		document bool
		// want is a part of the failure message that names the reason, when the variant has one.
		want string
	}
	variants := map[string]broken{
		// memoTable.put that links a new entry behind the chain instead of replacing the entry under
		// the same key: lookups keep finding the old entry.
		"store appends without replacing": {op: "memo.store", call: func(m *rtMachine, args []any) []any {
			t := &m.p.memo
			pos := int(args[1].(int64))
			e := t.alloc()
			*e = memoEntry{ok: args[4].(bool), end: int(args[5].(int64)), rule: int32(args[0].(int64)), min: int32(args[2].(int64)), env: m.vals[args[3].(int64)]}
			for pos >= len(t.slots) {
				t.slots = append(t.slots, nil)
			}
			link := &t.slots[pos]
			for *link != nil {
				link = &(*link).next
			}
			*link = e
			return []any{e}
		}},
		// A first-call record that drops the rule from the bit index (every rule shares one bit).
		"seen ignores the rule": {op: "seen.first", call: func(m *rtMachine, args []any) []any {
			m.p.Pos = int(args[1].(int64))
			return []any{m.p.FirstCall(m.rules[0])}
		}},
		// A lookup that forgets the level, as a key without Min would.
		"lookup ignores the level": {op: "memo.lookup", call: func(m *rtMachine, args []any) []any {
			e := m.p.memoGet(int(args[0].(int64)), int(args[1].(int64)), 0, m.vals[args[3].(int64)])
			return []any{e, e != nil}
		}},
		// A prune that shifts the slots by one position too many: the entries at the floor are lost.
		"prune drops the floor's entries": {op: "memo.prune", stream: true, call: func(m *rtMachine, args []any) []any {
			pos := int(args[0].(int64))
			m.p.pruneStreamMemo(pos + 1)
			m.p.Stream.memoBase = pos
			return nil
		}},
		// A prune that keeps the memo and moves only the floor: entries shift to the wrong positions.
		"prune moves only the floor": {op: "memo.prune", stream: true, call: func(m *rtMachine, args []any) []any {
			m.p.Stream.memoBase = int(args[0].(int64))
			return nil
		}},
		// A prune that frees nothing: the floor stays and every entry is kept.
		"prune frees nothing": {op: "memo.prune", stream: true, want: "result 0: model 4, implementation 0", call: func(m *rtMachine, args []any) []any {
			return nil
		}},
		// A prune that charges one unit more than the entries it removes: beyond the bound.
		"prune charges beyond its bound": {op: "memo.prune", stream: true, controlled: true, want: "more than the bound", call: func(m *rtMachine, args []any) []any {
			m.p.pruneStreamMemo(int(args[0].(int64)))
			m.p.Spend()
			return nil
		}},
		// A store before the floor that lands on the floor's slot instead of being dropped: a lookup
		// at the floor finds it.
		"store below the floor is kept": {op: "memo.store", stream: true, want: "model false, implementation true", call: func(m *rtMachine, args []any) []any {
			rule, pos, level, env := int(args[0].(int64)), int(args[1].(int64)), int(args[2].(int64)), m.vals[args[3].(int64)]
			e := m.p.memo.alloc()
			*e = memoEntry{ok: args[4].(bool), end: int(args[5].(int64)), silent: args[6].(bool)}
			m.p.memoPut(rule, max(pos, m.p.Stream.memoBase), level, env, e)
			return []any{e}
		}},
		// A splice that reads an examined end of 0 as no record: it drops an entry at position 0 that
		// examined no input.
		"splice drops an entry that examined nothing at position 0": {op: "memo.splice", document: true, call: func(m *rtMachine, args []any) []any {
			for _, head := range m.p.memo.slots {
				for e := head; e != nil; e = e.next {
					if e.examined == 0 {
						e.from = noRecord
					}
				}
			}
			m.p.memo.splice(documentEdit{int(args[0].(int64)), int(args[1].(int64)), int(args[2].(int64))})
			return nil
		}},
		// A splice that keeps the entries whose examined end is the edit's start plus one.
		"splice keeps an entry that examined the edited unit": {op: "memo.splice", call: func(m *rtMachine, args []any) []any {
			start, end := int(args[0].(int64))+1, int(args[1].(int64))
			m.p.memo.splice(documentEdit{start, max(start, end), int(args[2].(int64))})
			return nil
		}},
		// A splice that moves the entries that examined the unit before them at the edit's end.
		"splice moves an entry that examined the unit before the edit's end": {op: "memo.splice", call: func(m *rtMachine, args []any) []any {
			for _, head := range m.p.memo.slots {
				for e := head; e != nil; e = e.next {
					if e.from == fromBefore {
						e.from = fromPos
					}
				}
			}
			m.p.memo.splice(documentEdit{int(args[0].(int64)), int(args[1].(int64)), int(args[2].(int64))})
			return nil
		}},
		// A splice that moves the entries with recovered errors.
		"splice moves an entry with recovered errors": {op: "memo.splice", call: func(m *rtMachine, args []any) []any {
			for _, head := range m.p.memo.slots {
				for e := head; e != nil; e = e.next {
					e.errs = nil
				}
			}
			m.p.memo.splice(documentEdit{int(args[0].(int64)), int(args[1].(int64)), int(args[2].(int64))})
			return nil
		}},
		// A splice that moves pinned entries.
		"splice moves a pinned entry": {op: "memo.splice", call: func(m *rtMachine, args []any) []any {
			for _, head := range m.p.memo.slots {
				for e := head; e != nil; e = e.next {
					e.pinned = false
				}
			}
			m.p.memo.splice(documentEdit{int(args[0].(int64)), int(args[1].(int64)), int(args[2].(int64))})
			return nil
		}},
		// A lookup that charges each entry it passes twice: within the results, beyond the bound.
		"lookup charges twice": {op: "memo.lookup", controlled: true, call: func(m *rtMachine, args []any) []any {
			if pos := int(args[1].(int64)); pos < len(m.p.memo.slots) {
				for e := m.p.memo.slots[pos]; e != nil; e = e.next {
					m.p.Spend()
					m.p.Spend()
				}
			}
			e := m.p.memoGet(int(args[0].(int64)), int(args[1].(int64)), int(args[2].(int64)), m.vals[args[3].(int64)])
			return []any{e, e != nil}
		}},
	}
	for name, b := range variants {
		t.Run(name, func(t *testing.T) {
			ops := rtMemoOps
			if strings.HasPrefix(b.op, "memo.splice") {
				ops = rtDocEditOps
			}
			if b.document {
				ops = append(slices.DeleteFunc(append([]string(nil), rtMemoOps...), func(o string) bool { return o == "seen.first" }), rtDocOps...)
			}
			if b.stream {
				ops = append(slices.DeleteFunc(append([]string(nil), rtMemoOps...), func(o string) bool { return o == "seen.first" }), rtStreamOps...)
			}
			s := conform.Suite{Name: "broken: " + name, Ops: ops, New: func(c ref.Config) conform.Impl {
				if b.controlled {
					return meteredBrokenRT{brokenRT{newRTMachine(c, true, false, b.stream).(meteredRT).rtMachine, b.op, b.call}}
				}
				return brokenRT{newRTMachine(c, false, false, b.stream).(*rtMachine), b.op, b.call}
			}}
			fails, _ := conformanceFailures(s, 200, 150)
			if len(fails) == 0 {
				t.Fatal("conformance did not detect the broken operation")
			}
			var f *conform.Failure
			if !errors.As(fails[0], &f) {
				t.Fatalf("not a conformance failure: %v", fails[0])
			}
			if !strings.Contains(f.Msg, b.want) {
				t.Fatalf("failed for another reason than %q:\n%v", b.want, fails[0])
			}
			t.Logf("detected:\n%v", fails[0])
		})
	}
}

type brokenRT struct {
	*rtMachine
	op   string
	call func(m *rtMachine, args []any) []any
}

func (b brokenRT) Call(o *exec.Op, args []any) []any {
	if o.Name == b.op {
		return b.call(b.rtMachine, args)
	}
	return b.rtMachine.Call(o, args)
}

type meteredBrokenRT struct{ brokenRT }

func (m meteredBrokenRT) Used() uint64 { return m.p.Control.budgetUsed() }

// TestExecConformanceDetectsBrokenTypedRuntime is the negative control of the typed runtime's
// Document operations: a splice that moves the entries with recovered errors, and one that moves
// the pinned entries, must fail conformance.
func TestExecConformanceDetectsBrokenTypedRuntime(t *testing.T) {
	variants := map[string]func(m *typedMachine){
		"splice moves an entry with recovered errors": func(m *typedMachine) {
			for _, head := range m.p.memoSlots {
				for e := head; e != nil; e = e.next {
					e.errs = nil
				}
			}
		},
		"splice moves a pinned entry": func(m *typedMachine) {
			for e, meta := range m.doc.meta {
				meta.pinned = false
				m.doc.meta[e] = meta
			}
		},
	}
	for name, unpin := range variants {
		t.Run(name, func(t *testing.T) {
			s := conform.Suite{Name: "broken: " + name, Ops: rtDocEditOps, New: func(c ref.Config) conform.Impl {
				return brokenTyped{newTypedMachine(c, false).(*typedMachine), unpin}
			}}
			fails, _ := conformanceFailures(s, 200, 150)
			if len(fails) == 0 {
				t.Fatal("conformance did not detect the broken operation")
			}
		})
	}
}

// brokenTyped runs prepare before each memo.splice of a typed machine.
type brokenTyped struct {
	*typedMachine
	prepare func(m *typedMachine)
}

func (b brokenTyped) Call(o *exec.Op, args []any) []any {
	if o.Name == "memo.splice" {
		b.doc.slots = b.p.memoSlots
		b.prepare(b.typedMachine)
	}
	return b.typedMachine.Call(o, args)
}
