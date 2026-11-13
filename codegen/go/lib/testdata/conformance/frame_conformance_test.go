package libgo

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

// The capture frames of the node and the typed library under conformance: every frame operation runs
// through its binding in frame_bind_exec_test.go or tframe_bind_exec_test.go, which the test that
// copies the library prints with the library's printer, so that the suite runs the code that the
// library's compositions run. The library has no missing-token record. The typed library reuses the
// frames that frame.close ends.

// frameMachine binds the frame operations of the node or the typed library, and the rest through
// the machine of the pilot operations.
type frameMachine struct {
	conform.Impl
	p      *Parser
	regs   map[string]any
	binder conform.Binder
	used   func() uint64
}

type meteredFrame struct{ *frameMachine }

func (m meteredFrame) Used() uint64 { return m.used() }

func newFrameMachine(cfg ref.Config, typed, controlled bool) conform.Impl {
	m := &frameMachine{}
	if typed {
		tm := newTypedMachine(cfg, controlled)
		var x *typedMachine
		switch y := tm.(type) {
		case *typedMachine:
			x = y
		case meteredTyped:
			x = y.typedMachine
		}
		m.Impl, m.p = tm, x.p.Parser
		m.binder = conform.Binder{Methods: confTFrameBindings(x.p), Conv: frameConv{}}
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
		m.binder = conform.Binder{Methods: confFrameBindings(x.p), Conv: frameConv{}}
	}
	if controlled {
		m.used = func() uint64 { return m.p.Control.budgetUsed() }
		return meteredFrame{m}
	}
	return m
}

func (m *frameMachine) Datum(id int64) any { return m.Impl.(conform.Environment).Datum(id) }

func (m *frameMachine) Rule(index int64) any { return m.Impl.(conform.Environment).Rule(index) }

func (m *frameMachine) Err(id int64) any { return m.Impl.(conform.Recovery).Err(id) }

func (m *frameMachine) Recovered() []int64 { return m.Impl.(conform.Recovery).Recovered() }

func (m *frameMachine) Unrolled() int { return m.Impl.(conform.Recovery).Unrolled() }

func (m *frameMachine) List(l any) (visits, errs int) { return m.Impl.(conform.Recovery).List(l) }

// UseRegisters hands the machine the registers of the run: frame.mark reads the position.
func (m *frameMachine) UseRegisters(regs map[string]any) { m.regs = regs }

func (m *frameMachine) Handle(name string) any {
	if name == conform.OprNodeOp {
		return new(core.Node)
	}
	panic("library binding: no handle " + name)
}

func (m *frameMachine) Call(o *exec.Op, args []any) []any {
	if pos, ok := m.regs[ops.Pos].(int64); ok {
		m.p.Pos = int(pos)
	}
	if o.Name == ref.FrameSlotOp {
		var v any
		switch f := args[0].(type) {
		case *CaptureFrame:
			if x := f.Vals[args[1].(int64)]; x != nil {
				v = x
			}
		case *TCaptureFrame:
			v = f.Vals[args[1].(int64)]
		}
		return []any{v}
	}
	if !strings.HasPrefix(o.Name, "frame.") {
		return m.Impl.Call(o, args)
	}
	defer interrupted()
	return m.binder.Call(o, args)
}

// frameConv converts the backtracking points of the library, which it holds by value: each frame.mark
// is a handle of its own.
type frameConv struct{}

type frameMarkBox struct{ m Mark }

func (frameConv) ToImpl(arg any, t reflect.Type) (reflect.Value, bool) {
	if b, ok := arg.(*frameMarkBox); ok {
		return reflect.ValueOf(b.m), true
	}
	return reflect.Value{}, false
}

func (frameConv) FromImpl(v reflect.Value) (any, bool) {
	if x, ok := v.Interface().(Mark); ok {
		return &frameMarkBox{x}, true
	}
	return nil, false
}

// frameSuite is the suite of the frame operations of the node or the typed library, metered if
// controlled.
func frameSuite(typed, controlled bool) conform.Suite {
	name := "runtime frame"
	if typed {
		name = "typed library frame"
	}
	if controlled {
		name += " (budget)"
	}
	return conform.Suite{Name: name, Ops: conform.FrameSuiteOps(false),
		New: func(c ref.Config) conform.Impl { return newFrameMachine(c, typed, controlled) }}
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
		run(conform.RandomFrame(fmt.Sprint("random_", seed), rng, conform.FrameRandomOps(false), n))
	}
	return fails, ran
}

func TestExecConformanceFrame(t *testing.T) {
	seeds := 300
	if testing.Short() {
		seeds = 30
	}
	for _, typed := range []bool{false, true} {
		for _, controlled := range []bool{false, true} {
			fails, ran := frameConformanceFailures(frameSuite(typed, controlled), seeds, 150)
			if ran < seeds {
				t.Fatalf("only %d cases ran", ran)
			}
			for _, err := range fails {
				t.Error(err)
			}
		}
	}
}

// brokenFrame is the library with one frame operation replaced.
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

// TestExecConformanceFrameDetectsBrokenLib is the negative control: the node and the typed library
// with one frame operation broken, as a wrong line of its printed code would break it, fail the suite.
func TestExecConformanceFrameDetectsBrokenLib(t *testing.T) {
	for _, typed := range []bool{false, true} {
		for _, br := range conform.FrameBrokenOps(false) {
			t.Run(fmt.Sprint(br.Name, " typed=", typed), func(t *testing.T) {
				s := frameSuite(typed, false)
				s.New = func(c ref.Config) conform.Impl {
					return brokenFrame{newFrameMachine(c, typed, false).(*frameMachine), br.Op, br.Edit}
				}
				if fails, _ := frameConformanceFailures(s, 30, 150); len(fails) == 0 {
					t.Fatal("no case detected the broken library")
				}
			})
		}
	}
}
