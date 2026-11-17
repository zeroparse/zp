package libgo

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

// The elements gathered and the values made of the node and the typed library under conformance: every
// operation runs through its binding in terms_bind_exec_test.go or tterms_bind_exec_test.go, which the
// test that copies the library prints with the library's printer, so that the suite runs the code that
// the library's compositions run.

// termStateMachine binds the operations of the elements gathered and the values made of the node or the
// typed library, and the rest through the machine of the parse state.
type termStateMachine struct {
	conform.Impl
	p      *Parser
	tp     *TParser
	binder conform.Binder
	used   func() uint64
	ids    map[any]int64
}

type meteredTermState struct{ *termStateMachine }

func (m meteredTermState) Used() uint64 { return m.used() }

func newTermStateMachine(cfg ref.Config, typed, controlled bool) conform.Impl {
	m := &termStateMachine{ids: map[any]int64{}}
	if typed {
		tm := newTypedMachine(cfg, controlled)
		var x *typedMachine
		switch y := tm.(type) {
		case *typedMachine:
			x = y
		case meteredTyped:
			x = y.typedMachine
		}
		m.Impl, m.p, m.tp = tm, x.p.Parser, x.p
		m.binder = conform.Binder{Methods: confTTermStateBindings(x.p), Conv: termStateConv{}}
	} else {
		rm := newRTMachine(cfg, controlled, false, false)
		var x *rtMachine
		switch y := rm.(type) {
		case *rtMachine:
			x = y
		case meteredRT:
			x = y.rtMachine
		}
		m.Impl, m.p = rm, x.p
		m.binder = conform.Binder{Methods: confTermStateBindings(x.p), Conv: termStateConv{}}
	}
	if controlled {
		m.used = func() uint64 { return m.p.Control.budgetUsed() }
		return meteredTermState{m}
	}
	return m
}

func (m *termStateMachine) Act() any {
	if m.tp != nil {
		return &TActCtx{P: m.tp}
	}
	return &ActCtx{P: m.p}
}

// value returns a new element or struct of the library with the identifier id: a node, or a value of the
// typed library.
func (m *termStateMachine) value(id int64) any {
	var v any = new(core.Node)
	if m.tp != nil {
		v = new(TMatch)
	}
	m.ids[v] = id
	return v
}

func (m *termStateMachine) Elem(id int64) any { return m.value(id) }

func (m *termStateMachine) Struct(id int64) any { return m.value(id) }

// itemsBox holds the elements of a list, which the harness compares by identity.
type itemsBox struct {
	nodes []*core.Node
	vals  []any
}

func (m *termStateMachine) Items(ids []int64) any {
	b := &itemsBox{nodes: []*core.Node{}, vals: []any{}}
	for _, id := range ids {
		v := m.value(id)
		if m.tp != nil {
			b.vals = append(b.vals, v)
		} else {
			b.nodes = append(b.nodes, v.(*core.Node))
		}
	}
	return b
}

func listIDs[T comparable](ids map[any]int64, vs []T) []int64 {
	out := []int64{}
	for _, v := range vs {
		out = append(out, ids[v])
	}
	return out
}

func (m *termStateMachine) Gathered() []int64 {
	if m.tp != nil {
		return listIDs(m.ids, m.tp.KidStack)
	}
	return listIDs(m.ids, m.p.KidStack)
}

func (m *termStateMachine) ItemIDs(items any) []int64 {
	if b := items.(*itemsBox); m.tp != nil {
		return listIDs(m.ids, b.vals)
	} else {
		return listIDs(m.ids, b.nodes)
	}
}

func (m *termStateMachine) Made() []int64 {
	if m.tp != nil {
		return listIDs(m.ids, m.tp.Created)
	}
	return listIDs(m.ids, m.p.created)
}

func (m *termStateMachine) Datum(id int64) any { return m.Impl.(conform.Environment).Datum(id) }

func (m *termStateMachine) Rule(index int64) any { return m.Impl.(conform.Environment).Rule(index) }

func (m *termStateMachine) Call(o *code.Op, args []any) []any {
	if !slices.Contains(conform.GatheredOps, o.Name) && !slices.Contains(conform.MadeOps, o.Name) {
		return m.Impl.Call(o, args)
	}
	defer interrupted()
	return m.binder.Call(o, args)
}

// termStateConv converts the elements of a list to the slice that the bindings take, and the slice that
// term.elemsAbove returns to the elements of a list.
type termStateConv struct{}

func (termStateConv) ToImpl(arg any, t reflect.Type) (reflect.Value, bool) {
	b, ok := arg.(*itemsBox)
	if !ok {
		return reflect.Value{}, false
	}
	if t == reflect.TypeOf(b.vals) {
		return reflect.ValueOf(b.vals), true
	}
	return reflect.ValueOf(b.nodes), true
}

func (termStateConv) FromImpl(v reflect.Value) (any, bool) {
	switch x := v.Interface().(type) {
	case []*core.Node:
		return &itemsBox{nodes: x}, true
	case []any:
		return &itemsBox{vals: x}, true
	}
	return nil, false
}

// termStateSuite is the suite of the operations of the elements gathered or the values made of the node or
// the typed library, metered if controlled; a metered library charges each drop and copy to the unit.
func termStateSuite(ops []string, typed, controlled bool) conform.Suite {
	name := "runtime terms"
	if typed {
		name = "typed library terms"
	}
	if controlled {
		name += " (budget)"
	}
	s := conform.Suite{Name: name, Ops: ops, New: func(c ref.Config) conform.Impl { return newTermStateMachine(c, typed, controlled) }}
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

func TestExecConformanceTermState(t *testing.T) {
	seeds := 300
	if testing.Short() {
		seeds = 30
	}
	for _, fam := range termStateFamilies {
		for _, typed := range []bool{false, true} {
			for _, controlled := range []bool{false, true} {
				fails, ran := termStateConformanceFailures(fam, termStateSuite(fam.ops, typed, controlled), seeds, 120)
				if ran < seeds {
					t.Fatalf("only %d cases ran", ran)
				}
				for _, err := range fails {
					t.Error(err)
				}
			}
		}
	}
}

// brokenTermState is the library with one operation of the elements gathered or the values made replaced.
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

// brokenTermStateCharge is the metered library whose op does its work without charging, as a printed line
// without the charge would, or, if copy is set, charging only for the copy: term.elemsAbove that takes the
// elements off for free.
type brokenTermStateCharge struct {
	meteredTermState
	op   string
	copy bool
}

func (b brokenTermStateCharge) Call(o *code.Op, args []any) []any {
	if o.Name != b.op {
		return b.meteredTermState.Call(o, args)
	}
	m, tp := b.termStateMachine, b.tp
	switch o.Name {
	case "act.pushAll":
		box := args[1].(*itemsBox)
		if tp != nil {
			tp.KidStack = append(tp.KidStack, box.vals...)
		} else {
			m.p.KidStack = append(m.p.KidStack, box.nodes...)
		}
	case "term.dropKids":
		if k := int(args[0].(int64)); tp != nil {
			tp.KidStack = tp.KidStack[:k]
		} else {
			m.p.KidStack = m.p.KidStack[:k]
		}
	case "term.dropCreated":
		if k := int(args[0].(int64)); tp != nil {
			tp.Created = tp.Created[:k]
		} else {
			m.p.created = m.p.created[:k]
		}
	case "term.elemsAbove":
		k := int(args[0].(int64))
		if tp != nil {
			vs := slices.Clone(tp.KidStack[k:])
			if b.copy {
				copySpend(tp.Parser, vs, tp.KidStack[k:])
			}
			tp.KidStack = tp.KidStack[:k]
			return []any{&itemsBox{vals: vs}}
		}
		ns := slices.Clone(m.p.KidStack[k:])
		if b.copy {
			copySpend(m.p, ns, m.p.KidStack[k:])
		}
		m.p.KidStack = m.p.KidStack[:k]
		return []any{&itemsBox{nodes: ns}}
	}
	return nil
}

// TestExecConformanceTermStateDetectsBrokenLib is the negative control: the node and the typed library with
// one operation broken, as a wrong line of its printed code would break it, or with a drop or copy that
// charges nothing, fail the suite.
func TestExecConformanceTermStateDetectsBrokenLib(t *testing.T) {
	for _, fam := range termStateFamilies {
		for _, typed := range []bool{false, true} {
			for _, br := range fam.broken() {
				t.Run(fmt.Sprint(br.Name, " typed=", typed), func(t *testing.T) {
					s := termStateSuite(fam.ops, typed, false)
					s.New = func(c ref.Config) conform.Impl {
						return brokenTermState{newTermStateMachine(c, typed, false).(*termStateMachine), br.Op, br.Edit}
					}
					if fails, _ := termStateConformanceFailures(fam, s, 30, 120); len(fails) == 0 {
						t.Fatal("no case detected the broken library")
					}
				})
			}
			for _, br := range []struct {
				op   string
				copy bool
			}{{"act.pushAll", false}, {"term.dropKids", false}, {"term.elemsAbove", false}, {"term.elemsAbove", true}, {"term.dropCreated", false}} {
				if !slices.Contains(fam.ops, br.op) {
					continue
				}
				what := " charges nothing"
				if br.copy {
					what = " charges only the copy"
				}
				t.Run(fmt.Sprint(br.op, what, " typed=", typed), func(t *testing.T) {
					s := termStateSuite(fam.ops, typed, true)
					s.New = func(c ref.Config) conform.Impl {
						return brokenTermStateCharge{newTermStateMachine(c, typed, true).(meteredTermState), br.op, br.copy}
					}
					if fails, _ := termStateConformanceFailures(fam, s, 30, 120); len(fails) == 0 {
						t.Fatal("no case detected the broken library")
					}
				})
			}
		}
	}
}
