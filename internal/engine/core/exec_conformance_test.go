package core

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/ir/exec/ref"
	"zeroparse.dev/zp/internal/limits"
)

// The engine implements the memo table and deferred memoization of the parser VM (design 065)
// in MemoTable and the Parser's memo functions. The binding calls them as the printed call protocol
// does (call_exec.go): memo.store allocates an entry and stores it with MemoPut.

type engineMachine struct {
	p     *Parser
	vals  map[int64][]any
	rules []*Rule
}

func newEngineMachine(cfg ref.Config, controlled, sparse bool) conform.Impl {
	p := &Parser{Memo: NewMemoTable(), MaxDepth: int(cfg.MaxDepth), DeferMemo: true}
	p.Memo.Stride, p.Memo.Sparse = cfg.Rules, sparse
	m := &engineMachine{p: p, vals: map[int64][]any{}}
	for i := range cfg.Rules {
		r := &Rule{}
		r.Seen = i
		m.rules = append(m.rules, r)
	}
	if !controlled {
		return m
	}
	limit := cfg.Limit
	if limit == 0 {
		limit = limits.Unlimited
	}
	p.Control = NewBudgetControl(nil, limits.Limits{Compute: limit})
	return meteredEngine{m}
}

type meteredEngine struct{ *engineMachine }

func (m meteredEngine) Used() uint64 { return m.p.Control.Used }

func (m *engineMachine) Values(id int64) any {
	if m.vals[id] == nil {
		m.vals[id] = []any{id}
	}
	return id
}

// key returns the memo key of the first four arguments of a memo operation.
func (m *engineMachine) key(args []any) MemoKey {
	return MemoKey{Rule: int(args[0].(int64)), Pos: int(args[1].(int64)), Min: int(args[2].(int64)), Env: m.vals[args[3].(int64)]}
}

func (m *engineMachine) Call(o *exec.Op, args []any) (res []any) {
	defer func() {
		if x := recover(); x != nil {
			f, ok := x.(Fatal)
			if !ok {
				panic(x)
			}
			var le *limits.LimitError
			if errors.As(f.Err, &le) && le.Kind == limits.LimitDepth {
				panic(exec.Interrupt{Reason: "nesting"})
			}
			panic(exec.Interrupt{Reason: "limit"})
		}
	}()
	p := m.p
	switch o.Name {
	case "memo.lookup":
		e, found := p.MemoGet(m.key(args))
		return []any{e, found}
	case "memo.store":
		e := p.Memo.Alloc()
		*e = MemoEntry{Ok: args[4].(bool), End: int(args[5].(int64)), silent: args[6].(bool), Errs: engineErrors(args[8])}
		p.MemoPut(m.key(args), e)
		return []any{e}
	case "memo.ok":
		return []any{args[0].(*MemoEntry).Ok}
	case "memo.end":
		return []any{int64(args[0].(*MemoEntry).End)}
	case "memo.silent":
		return []any{args[0].(*MemoEntry).silent}
	case "seen.first":
		p.Pos = int(args[1].(int64))
		return []any{p.FirstCall(m.rules[args[0].(int64)])}
	case "budget.charge":
		p.Control.Charge(args[0].(uint64))
		return nil
	case "budget.used":
		return []any{p.Control.Used}
	}
	panic("engine binding: no operation " + o.Name)
}

// engineErrors returns the errors that an entry of the engine holds for the recovered errors of an
// argument of memo.store: none for nil, and one error for the errors that conform.ErrsOp makes.
func engineErrors(arg any) []*SyntaxError {
	if arg == nil {
		return nil
	}
	return []*SyntaxError{{}}
}

var (
	engineMemoOps   = []string{"memo.lookup", "memo.store", "memo.ok", "memo.end", "memo.silent", "seen.first"}
	engineBudgetOps = []string{"budget.charge", "budget.used"}
)

func engineSuites() []conform.Suite {
	var suites []conform.Suite
	for _, controlled := range []bool{false, true} {
		for _, sparse := range []bool{false, true} {
			ops := append([]string(nil), engineMemoOps...)
			if controlled {
				ops = append(ops, engineBudgetOps...)
			}
			suites = append(suites, conform.Suite{
				Name: fmt.Sprintf("engine (controlled=%v, sparse seen=%v)", controlled, sparse),
				Ops:  ops,
				New:  func(c ref.Config) conform.Impl { return newEngineMachine(c, controlled, sparse) },
			})
		}
	}
	return suites
}

// engineConformanceFailures runs the handwritten cases and seeds randomized cases of n calls on s,
// and returns the failures and the number of cases that ran.
func engineConformanceFailures(s conform.Suite, seeds, n int) (fails []error, ran int) {
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
		run(conform.Random(fmt.Sprint("random_", seed), rng, s.Ops, n, ref.Config{Rules: 3 + seed%2*130, Limit: limit, MaxDepth: 4}))
	}
	return fails, ran
}

func TestEngineExecConformance(t *testing.T) {
	seeds := 300
	if testing.Short() {
		seeds = 30
	}
	for _, s := range engineSuites() {
		t.Run(s.Name, func(t *testing.T) {
			fails, ran := engineConformanceFailures(s, seeds, 150)
			if ran < seeds {
				t.Fatalf("only %d cases ran", ran)
			}
			for _, err := range fails {
				t.Error(err)
			}
		})
	}
}

// TestEngineExecConformanceDetectsBrokenEngine is the negative control: deliberately broken
// variants of the engine's memo operations must fail conformance.
func TestEngineExecConformanceDetectsBrokenEngine(t *testing.T) {
	type broken struct {
		op         string
		call       func(m *engineMachine, args []any) []any
		controlled bool
		// want is a part of the failure message that names the reason, when the variant has one.
		want string
	}
	variants := map[string]broken{
		// A lookup that forgets the level, as a key without Min would.
		"lookup ignores the level": {op: "memo.lookup", call: func(m *engineMachine, args []any) []any {
			k := m.key(args)
			k.Min = 0
			e, found := m.p.MemoGet(k)
			return []any{e, found}
		}},
		// A store that links a new entry behind the chain instead of replacing the entry under the
		// same key: lookups keep finding the old entry.
		"store appends without replacing": {op: "memo.store", call: func(m *engineMachine, args []any) []any {
			k := m.key(args)
			e := m.p.Memo.Alloc()
			*e = MemoEntry{Ok: args[4].(bool), End: int(args[5].(int64)), Rule: int32(k.Rule), Min: int32(k.Min), Env: k.Env, Pos: k.Pos}
			t := m.p.Memo
			for k.Pos >= len(t.Slots) {
				t.Slots = append(t.Slots, nil)
			}
			link := &t.Slots[k.Pos]
			for *link != nil {
				link = &(*link).Next
			}
			*link = e
			return []any{e}
		}},
		// A first-call record that drops the rule (every rule shares one record).
		"seen ignores the rule": {op: "seen.first", call: func(m *engineMachine, args []any) []any {
			m.p.Pos = int(args[1].(int64))
			return []any{m.p.FirstCall(m.rules[0])}
		}},
		// A lookup that charges each entry it passes twice: within the results, beyond the bound.
		"lookup charges twice": {op: "memo.lookup", controlled: true, want: "more than the bound", call: func(m *engineMachine, args []any) []any {
			k := m.key(args)
			if i := k.Pos - m.p.Memo.Base; i >= 0 && i < len(m.p.Memo.Slots) {
				for e := m.p.Memo.Slots[i]; e != nil; e = e.Next {
					m.p.Spend()
					m.p.Spend()
				}
			}
			e, found := m.p.MemoGet(k)
			return []any{e, found}
		}},
	}
	for name, b := range variants {
		t.Run(name, func(t *testing.T) {
			s := conform.Suite{Name: "broken: " + name, Ops: engineMemoOps, New: func(c ref.Config) conform.Impl {
				if b.controlled {
					return meteredBrokenEngine{brokenEngine{newEngineMachine(c, true, false).(meteredEngine).engineMachine, b.op, b.call}}
				}
				return brokenEngine{newEngineMachine(c, false, false).(*engineMachine), b.op, b.call}
			}}
			if b.controlled {
				s.Ops = append(append([]string(nil), engineMemoOps...), engineBudgetOps...)
			}
			fails, _ := engineConformanceFailures(s, 200, 150)
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
		})
	}
}

type brokenEngine struct {
	*engineMachine
	op   string
	call func(m *engineMachine, args []any) []any
}

func (b brokenEngine) Call(o *exec.Op, args []any) []any {
	if o.Name == b.op {
		return b.call(b.engineMachine, args)
	}
	return b.engineMachine.Call(o, args)
}

type meteredBrokenEngine struct{ brokenEngine }

func (m meteredBrokenEngine) Used() uint64 { return m.p.Control.Used }
