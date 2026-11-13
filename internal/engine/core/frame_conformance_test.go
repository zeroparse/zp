package core

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/ir/exec/ref"
	"zeroparse.dev/zp/runtime/parse"
)

// The engine's capture frames under conformance: every frame operation runs through its binding in
// frame_bind_exec_test.go, which the engine's printer prints from the operation, so that the suite
// runs the code that the engine's compositions run.

// frameMachine binds the frame operations of the engine, and the rest through engineMachine.
type frameMachine struct {
	*engineMachine
	regs   map[string]any
	binder conform.Binder
}

type meteredFrame struct{ *frameMachine }

func (m meteredFrame) Used() uint64 { return m.p.Control.Used }

func newFrameMachine(cfg ref.Config, controlled bool) conform.Impl {
	var em *engineMachine
	switch x := newEngineMachine(cfg, controlled, false).(type) {
	case *engineMachine:
		em = x
	case meteredEngine:
		em = x.engineMachine
	}
	m := &frameMachine{engineMachine: em}
	m.binder = conform.Binder{Methods: confFrameBindings(m.p), Conv: frameConv{}}
	if controlled {
		return meteredFrame{m}
	}
	return m
}

// UseRegisters hands the machine the registers of the run: frame.mark reads the position.
func (m *frameMachine) UseRegisters(regs map[string]any) { m.regs = regs }

func (m *frameMachine) Handle(name string) any {
	if name == conform.OprNodeOp {
		return new(parse.Node)
	}
	panic("engine binding: no handle " + name)
}

func (m *frameMachine) Call(o *exec.Op, args []any) []any {
	p := m.p
	if pos, ok := m.regs[ops.Pos].(int64); ok {
		p.Pos = int(pos)
	}
	switch o.Name {
	case ref.FrameSlotOp:
		if v := args[0].(*Frame).Vals[args[1].(int64)]; v != nil {
			return []any{v}
		}
		return []any{nil}
	case ref.FrameMissingOp:
		p.MissingLen = int(args[0].(int64))
		return nil
	}
	if !strings.HasPrefix(o.Name, "frame.") {
		return m.engineMachine.Call(o, args)
	}
	defer interrupt()
	return m.binder.Call(o, args)
}

// frameConv converts the backtracking points of the engine, which it holds by value: each frame.mark
// is a handle of its own.
type frameConv struct{}

func (frameConv) ToImpl(arg any, t reflect.Type) (reflect.Value, bool) {
	if b, ok := arg.(*markBox); ok {
		return reflect.ValueOf(b.m), true
	}
	return reflect.Value{}, false
}

func (frameConv) FromImpl(v reflect.Value) (any, bool) {
	if x, ok := v.Interface().(Mark); ok {
		return &markBox{x}, true
	}
	return nil, false
}

// frameSuite is the suite of the engine's frame operations, metered if controlled.
func frameSuite(controlled bool) conform.Suite {
	name := "engine frame"
	if controlled {
		name += " (budget)"
	}
	return conform.Suite{Name: name, Ops: conform.FrameSuiteOps(true),
		New: func(c ref.Config) conform.Impl { return newFrameMachine(c, controlled) }}
}

// frameConformanceFailures runs the handwritten cases of the capture frames and seeds randomized cases
// of n calls on s, and returns the failures and the number of cases that ran.
func frameConformanceFailures(s conform.Suite, seeds, n int) (fails []error, ran int) {
	run := func(c conform.Case) {
		skipped, err := conform.Run(s, c)
		if !skipped {
			ran++
		}
		if err != nil {
			fails = append(fails, err)
		}
	}
	for _, c := range conform.FrameCases() {
		run(c)
	}
	for seed := range seeds {
		rng := rand.New(rand.NewPCG(uint64(seed), 67))
		run(conform.RandomFrame(fmt.Sprint("random_", seed), rng, conform.FrameRandomOps(true), n))
	}
	return fails, ran
}

func TestEngineFrameConformance(t *testing.T) {
	seeds := 300
	if testing.Short() {
		seeds = 30
	}
	for _, controlled := range []bool{false, true} {
		fails, ran := frameConformanceFailures(frameSuite(controlled), seeds, 150)
		if ran < seeds {
			t.Fatalf("only %d cases ran", ran)
		}
		for _, err := range fails {
			t.Error(err)
		}
	}
}

// brokenFrame is the engine with one frame operation replaced.
type brokenFrame struct {
	*frameMachine
	op   string
	edit func(call func(op string, args ...any) []any, args []any) []any
}

func (b brokenFrame) Call(o *exec.Op, args []any) []any {
	if o.Name != b.op {
		return b.frameMachine.Call(o, args)
	}
	mod := conform.FrameModule()
	return b.edit(func(op string, a ...any) []any { return b.frameMachine.Call(mod.Op(op), a) }, args)
}

// TestEngineFrameConformanceDetectsBrokenEngine is the negative control: the engine with one frame
// operation broken, as a wrong line of its printed code would break it, fails the suite.
func TestEngineFrameConformanceDetectsBrokenEngine(t *testing.T) {
	for _, br := range conform.FrameBrokenOps(true) {
		t.Run(br.Name, func(t *testing.T) {
			s := frameSuite(false)
			s.New = func(c ref.Config) conform.Impl {
				return brokenFrame{newFrameMachine(c, false).(*frameMachine), br.Op, br.Edit}
			}
			if fails, _ := frameConformanceFailures(s, 30, 150); len(fails) == 0 {
				t.Fatal("no case detected the broken engine")
			}
		})
	}
}
