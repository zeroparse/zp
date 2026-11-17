package state

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/exec/conform"
	"zeroparse.dev/zp/internal/ir/code/exec/ref"
)

// The engine's elements gathered and values made under conformance: every operation runs through its
// binding in terms_bind_exec_test.go, which the engine's printer prints from the operation as the backends
// print it into their own packages, so that the suite runs the code that the engine's compositions run.

// termStateMachine binds the operations of the elements gathered and the values made of the engine.
type termStateMachine struct {
	*engineMachine
	binder conform.Binder
	ctx    *EvalCtx
	ids    map[*core.Node]int64
}

type meteredTermState struct{ *termStateMachine }

func (m meteredTermState) Used() uint64 { return m.p.Control.Used }

func newTermStateMachine(cfg ref.Config, controlled bool) conform.Impl {
	var em *engineMachine
	switch x := newEngineMachine(cfg, controlled, false).(type) {
	case *engineMachine:
		em = x
	case meteredEngine:
		em = x.engineMachine
	}
	m := &termStateMachine{engineMachine: em, ctx: &EvalCtx{P: em.p}, ids: map[*core.Node]int64{}}
	m.binder = conform.Binder{Methods: confTermStateBindings(m.p), Conv: termStateConv{}}
	if controlled {
		return meteredTermState{m}
	}
	return m
}

func (m *termStateMachine) Act() any { return m.ctx }

func (m *termStateMachine) node(id int64) *core.Node {
	n := new(core.Node)
	m.ids[n] = id
	return n
}

func (m *termStateMachine) Elem(id int64) any { return m.node(id) }

func (m *termStateMachine) Struct(id int64) any { return m.node(id) }

// itemsBox holds the elements of a list, which the harness compares by identity.
type itemsBox struct{ items []*core.Node }

func (m *termStateMachine) Items(ids []int64) any {
	b := &itemsBox{}
	for _, id := range ids {
		b.items = append(b.items, m.node(id))
	}
	return b
}

func (m *termStateMachine) list(ns []*core.Node) []int64 {
	ids := []int64{}
	for _, n := range ns {
		ids = append(ids, m.ids[n])
	}
	return ids
}

func (m *termStateMachine) Gathered() []int64 { return m.list(m.p.KidStack) }

func (m *termStateMachine) ItemIDs(items any) []int64 { return m.list(items.(*itemsBox).items) }

func (m *termStateMachine) Made() []int64 { return m.list(m.p.Created) }

func (m *termStateMachine) Call(o *code.Op, args []any) []any {
	if !slices.Contains(conform.GatheredOps, o.Name) && !slices.Contains(conform.MadeOps, o.Name) {
		return m.engineMachine.Call(o, args)
	}
	defer interrupt()
	return m.binder.Call(o, args)
}

// termStateConv converts the elements of a list to the slice that the bindings take, and the slice that
// term.elemsAbove returns to the elements of a list.
type termStateConv struct{}

func (termStateConv) ToImpl(arg any, t reflect.Type) (reflect.Value, bool) {
	if b, ok := arg.(*itemsBox); ok {
		return reflect.ValueOf(b.items), true
	}
	return reflect.Value{}, false
}

func (termStateConv) FromImpl(v reflect.Value) (any, bool) {
	if ns, ok := v.Interface().([]*core.Node); ok {
		return &itemsBox{items: ns}, true
	}
	return nil, false
}

// termStateSuite is the suite of the engine's operations of the elements gathered or the values made, metered
// if controlled; a metered engine charges each drop and copy to the unit. The reference build has no budget
// controls, so there the suite checks the behavior alone.
func termStateSuite(ops []string, controlled bool) conform.Suite {
	controlled = controlled && BudgetControlsEnabled
	name := "engine terms"
	if controlled {
		name += " (budget)"
	}
	s := conform.Suite{Name: name, Ops: ops, New: func(c ref.Config) conform.Impl { return newTermStateMachine(c, controlled) }}
	if controlled {
		s.Exact = []string{"act.pushAll", "term.dropKids", "term.elemsAbove", "term.keep", "term.dropCreated"}
	}
	return s
}

// termStateFamily is the cases of the elements gathered or of the values made.
type termStateFamily struct {
	ops    []string
	cases  func() []conform.Case
	random func(name string, rng *rand.Rand, n int) conform.Case
	broken func() []conform.TermStateBroken
}

var termStateFamilies = []termStateFamily{
	{conform.GatheredOps, conform.GatheredCases, conform.RandomGathered, conform.GatheredBrokenOps},
	{conform.MadeOps, conform.MadeCases, conform.RandomMade, conform.MadeBrokenOps},
}

// termStateConformanceFailures runs the handwritten cases of fam and seeds randomized cases of n calls on s,
// and returns the failures and the number of cases that ran.
func termStateConformanceFailures(fam termStateFamily, s conform.Suite, seeds, n int) (fails []error, ran int) {
	run := func(c conform.Case) {
		skipped, err := conform.Run(s, c)
		if !skipped {
			ran++
		}
		if err != nil {
			fails = append(fails, err)
		}
	}
	for _, c := range fam.cases() {
		run(c)
	}
	for seed := range seeds {
		run(fam.random(fmt.Sprint("random_", seed), rand.New(rand.NewPCG(uint64(seed), 71)), n))
	}
	return fails, ran
}

func TestEngineTermStateConformance(t *testing.T) {
	seeds := 300
	if testing.Short() {
		seeds = 30
	}
	for _, fam := range termStateFamilies {
		for _, controlled := range []bool{false, true} {
			fails, ran := termStateConformanceFailures(fam, termStateSuite(fam.ops, controlled), seeds, 120)
			if ran < seeds {
				t.Fatalf("only %d cases ran", ran)
			}
			for _, err := range fails {
				t.Error(err)
			}
		}
	}
}

// brokenTermState is the engine with one operation of the elements gathered or the values made replaced.
type brokenTermState struct {
	*termStateMachine
	op   string
	edit func(call func(op string, args ...any) []any, args []any) []any
}

func (b brokenTermState) Call(o *code.Op, args []any) []any {
	if o.Name != b.op {
		return b.termStateMachine.Call(o, args)
	}
	mod := conform.TermStateModule()
	return b.edit(func(op string, a ...any) []any { return b.termStateMachine.Call(mod.Op(op), a) }, args)
}

// brokenTermStateCharge is the metered engine whose op charges nothing, or half of what it charges if half
// is set: term.elemsAbove charging for the copy and not for taking the elements off.
type brokenTermStateCharge struct {
	meteredTermState
	op   string
	half bool
}

func (b brokenTermStateCharge) Call(o *code.Op, args []any) []any {
	if o.Name != b.op {
		return b.meteredTermState.Call(o, args)
	}
	used := b.p.Control.Used
	defer func() {
		if b.half {
			b.p.Control.Used = used + (b.p.Control.Used-used)/2
		} else {
			b.p.Control.Used = used
		}
	}()
	return b.meteredTermState.Call(o, args)
}

// TestEngineTermStateConformanceDetectsBrokenEngine is the negative control: the engine with one operation
// broken, as a wrong line of its printed code would break it, or with a drop or copy that charges nothing,
// fails the suite.
func TestEngineTermStateConformanceDetectsBrokenEngine(t *testing.T) {
	for _, fam := range termStateFamilies {
		for _, br := range fam.broken() {
			t.Run(br.Name, func(t *testing.T) {
				s := termStateSuite(fam.ops, false)
				s.New = func(c ref.Config) conform.Impl {
					return brokenTermState{newTermStateMachine(c, false).(*termStateMachine), br.Op, br.Edit}
				}
				if fails, _ := termStateConformanceFailures(fam, s, 30, 120); len(fails) == 0 {
					t.Fatal("no case detected the broken engine")
				}
			})
		}
		for _, br := range []struct {
			op   string
			half bool
		}{{"act.pushAll", false}, {"term.dropKids", false}, {"term.elemsAbove", false}, {"term.elemsAbove", true}, {"term.dropCreated", false}} {
			if !slices.Contains(fam.ops, br.op) {
				continue
			}
			name := br.op + " charges nothing"
			if br.half {
				name = br.op + " charges only the copy"
			}
			t.Run(name, func(t *testing.T) {
				if !BudgetControlsEnabled {
					t.Skip("budget controls are unavailable in the reference build")
				}
				s := termStateSuite(fam.ops, true)
				s.New = func(c ref.Config) conform.Impl {
					return brokenTermStateCharge{newTermStateMachine(c, true).(meteredTermState), br.op, br.half}
				}
				if fails, _ := termStateConformanceFailures(fam, s, 30, 120); len(fails) == 0 {
					t.Fatal("no case detected the broken engine")
				}
			})
		}
	}
}
