package core

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/ir/exec/ref"
	"zeroparse.dev/zp/internal/value"
)

// The engine's dependency memo of left recursion under conformance: every ctx operation runs through
// its binding in ctx_bind_exec_test.go, which the engine's printer prints from the operation, so that
// the suite runs the code that the engine's compositions run. The conformance operations that install
// growths and lookaheads set the growths in progress directly and call LookBegin and LookEnd.

// ctxMachine binds the ctx operations of the engine, and the rest through expMachine.
type ctxMachine struct {
	*expMachine
	ctxRules []*Rule
	regs     map[string]any
	binder   conform.Binder
}

// ctxBox is the handle of a context state, which the engine holds by value.
type ctxBox struct{ c CtxState }

type meteredCtx struct{ *ctxMachine }

func (m meteredCtx) Used() uint64 { return m.p.Control.Used }

func newCtxMachine(cfg ref.Config, controlled bool) conform.Impl {
	var em *expMachine
	switch x := newExpMachine(cfg, controlled).(type) {
	case *expMachine:
		em = x
	case meteredExp:
		em = x.expMachine
	}
	m := &ctxMachine{expMachine: em}
	leaders := map[int64]int{}
	for i, cr := range cfg.CtxRules {
		r := &Rule{}
		r.ID, r.Cycle, r.Leader = i, int(cr.Cycle), cr.Leader
		if cr.Leader && cr.Cycle != 0 {
			// The model gives a bit to the leaders that the generated parsers do (ref.CtxLeaderBits).
			k := leaders[cr.Cycle]
			r.LeaderIndex = k
			if k < ref.CtxLeaderBits {
				r.LeaderBit = 1 << k
			}
			leaders[cr.Cycle]++
		}
		m.ctxRules = append(m.ctxRules, r)
	}
	m.binder = conform.Binder{Methods: confCtxBindings(m.p), Conv: ctxConv{m}}
	if controlled {
		return meteredCtx{m}
	}
	return m
}

func (m *ctxMachine) Rule(index int64) any { return m.ctxRules[index] }

// UseRegisters hands the machine the registers of the run: the ctx operations read the position and
// the depth of lookaheads.
func (m *ctxMachine) UseRegisters(regs map[string]any) { m.regs = regs }

func (m *ctxMachine) Handle(name string) any {
	if name == conform.OprNodeOp {
		return new(value.Node)
	}
	panic("engine binding: no handle " + name)
}

func (m *ctxMachine) Call(o *exec.Op, args []any) []any {
	p := m.p
	if pos, ok := m.regs[ops.Pos].(int64); ok {
		p.Pos = int(pos)
	}
	if silent, ok := m.regs[ops.Silent].(int64); ok {
		p.Silent = int(silent)
	}
	if strings.HasPrefix(o.Name, "test.") && o.Name != conform.OprNodeOp && slices.Contains(conform.CtxTestOps, o.Name) {
		// The conformance operations charge nothing.
		control := p.Control
		p.Control = nil
		defer func() { p.Control = control }()
		m.testOp(o.Name, args)
		return nil
	}
	if !strings.HasPrefix(o.Name, "ctx.") && o.Name != "memo.setPresent" {
		return m.expMachine.Call(o, args)
	}
	defer interrupt()
	res := m.binder.Call(o, args)
	k := len(o.Results)
	for i, prm := range o.Params {
		if prm.Type == exec.THandle(ops.Ctx) {
			args[i].(*ctxBox).c = res[k].(*ctxBox).c
			k++
		}
	}
	if slices.Contains(o.Writes, ops.Pos) {
		// The run compares the position that the operation wrote with the model's.
		m.regs[ops.Pos] = int64(p.Pos)
	}
	return res[:len(o.Results)]
}

// testOp runs a conformance operation of the growths.
func (m *ctxMachine) testOp(name string, args []any) {
	p := m.p
	seed := func(ok bool, end int64) *MemoEntry {
		return &MemoEntry{Ok: ok, End: int(end), Examined: int(end), From: p.Pos, Growing: true}
	}
	switch name {
	case ref.CtxGrowOp:
		r := args[0].(*Rule)
		p.Gamma = append(p.Gamma, growRec{key: MemoKey{Rule: r.ID, Pos: p.Pos, Min: int(args[1].(int64))},
			seed: seed(args[2].(bool), args[3].(int64)), id: uint64(args[4].(int64)), bit: r.LeaderBit, idx: int32(r.LeaderIndex), cycle: int32(r.Cycle)})
	case ref.CtxReseedOp:
		g := &p.Gamma[len(p.Gamma)-1]
		pos := p.Pos
		p.Pos = g.key.Pos
		g.seed, g.id = seed(args[0].(bool), args[1].(int64)), uint64(args[2].(int64))
		p.Pos = pos
	case ref.CtxUngrowOp:
		p.dropTable(len(p.Gamma) - 1)
		p.Gamma = p.Gamma[:len(p.Gamma)-1]
		if len(p.Gamma) == 0 {
			clear(p.interned) // as GrowEnd does
		}
	case ref.CtxLookBeginOp:
		p.LookBegin(int(args[0].(int64)))
	case ref.CtxLookEndOp:
		p.LookEnd(args[0].(bool))
	}
}

// ctxConv converts the handles of the ctx operations of the engine.
type ctxConv struct{ m *ctxMachine }

func (c ctxConv) ToImpl(arg any, t reflect.Type) (reflect.Value, bool) {
	switch a := arg.(type) {
	case *ctxBox:
		return reflect.ValueOf(a.c), true
	case *errorList:
		return reflect.ValueOf(a.e), true
	case keptID:
		return reflect.ValueOf(c.m.kept[a]), true
	case int64:
		if t == reflect.TypeFor[KeyVals]() {
			return reflect.ValueOf(KeyVals{Env: c.m.vals[a]}), true
		}
	}
	return reflect.Value{}, false
}

func (c ctxConv) FromImpl(v reflect.Value) (any, bool) {
	switch x := v.Interface().(type) {
	case CtxState:
		return &ctxBox{x}, true
	case ctxHit:
		if x.e == nil {
			return nil, true
		}
	}
	return nil, false
}

// ctxSuites are the suites of the engine's dependency memo, with and without charge controls.
func ctxSuites() []conform.Suite {
	var suites []conform.Suite
	for _, controlled := range []bool{false, true} {
		names := conform.CtxSuiteOps()
		if controlled {
			names = append(names, engineBudgetOps...)
		}
		suites = append(suites, conform.Suite{
			Name: fmt.Sprintf("engine ctx (controlled=%v)", controlled),
			Ops:  names,
			New:  func(c ref.Config) conform.Impl { return newCtxMachine(c, controlled) },
		})
	}
	return suites
}

// ctxConformanceFailures runs the handwritten cases of the dependency memo and seeds randomized cases
// of n calls on s, and returns the failures and the number of cases that ran.
func ctxConformanceFailures(s conform.Suite, seeds, n int) (fails []error, ran int) {
	run := func(c conform.Case) {
		skipped, err := conform.Run(s, c)
		if !skipped {
			ran++
		}
		if err != nil {
			fails = append(fails, err)
		}
	}
	for _, c := range conform.CtxCases() {
		run(c)
	}
	for seed := range seeds {
		run(conform.RandomCtx(fmt.Sprint("random_", seed), rand.New(rand.NewPCG(uint64(seed), 67)), n))
	}
	return fails, ran
}

func TestEngineCtxConformance(t *testing.T) {
	seeds := 300
	if testing.Short() {
		seeds = 30
	}
	for _, s := range ctxSuites() {
		fails, ran := ctxConformanceFailures(s, seeds, 200)
		if ran < seeds {
			t.Fatalf("%s: only %d cases ran", s.Name, ran)
		}
		for _, err := range fails[:min(len(fails), 3)] {
			t.Error(err)
		}
	}
}

// brokenCtx is the engine with one operation of the dependency memo replaced.
type brokenCtx struct {
	*ctxMachine
	op   string
	edit func(call func(op string, args ...any) []any, args []any) []any
}

func (b brokenCtx) Call(o *exec.Op, args []any) []any {
	if o.Name != b.op {
		return b.ctxMachine.Call(o, args)
	}
	mod := conform.CtxModule()
	return b.edit(func(op string, a ...any) []any { return b.ctxMachine.Call(mod.Op(op), a) }, args)
}

// TestEngineCtxConformanceDetectsBrokenEngine is the negative control: the engine with one operation
// of the dependency memo broken, as a wrong line of its printed code would break it, or with one of
// the checks of its reuse broken (ContextMemoMutation), fails the suite. The mutations of how a reuse
// replays the dependencies of observations need reseeded lookaheads under nested reuses, which the
// cases do not reach; the differential tests on cycle grammars detect them.
func TestEngineCtxConformanceDetectsBrokenEngine(t *testing.T) {
	for _, br := range conform.CtxBrokenOps() {
		t.Run(br.Name, func(t *testing.T) {
			s := ctxSuites()[0]
			s.New = func(c ref.Config) conform.Impl {
				return brokenCtx{newCtxMachine(c, false).(*ctxMachine), br.Op, br.Edit}
			}
			if fails, _ := ctxConformanceFailures(s, 60, 250); len(fails) == 0 {
				t.Fatal("no case detected the broken engine")
			}
		})
	}
	for _, mu := range []struct {
		name string
		kind int
	}{{"ignore seeds", CtxIgnoreSeeds}, {"ignore observations", CtxIgnoreObs}, {"ignore present sets", CtxIgnorePresent}} {
		t.Run(mu.name, func(t *testing.T) {
			ContextMemoMutation = mu.kind
			defer func() { ContextMemoMutation = 0 }()
			if fails, _ := ctxConformanceFailures(ctxSuites()[0], 60, 250); len(fails) == 0 {
				t.Fatal("no case detected the broken engine")
			}
		})
	}
}
