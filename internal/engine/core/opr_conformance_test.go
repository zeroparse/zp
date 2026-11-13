package core

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/ir/exec/ref"
	"zeroparse.dev/zp/runtime/parse"
)

// The engine's Operators Rule descriptors, matches of operators and records of the split forms of the
// loop under conformance: every operation runs through its binding in opr_bind_exec_test.go, which the
// engine's printer prints from the operation, so that the suite runs the code that the engine's
// compositions run.

// oprMachine binds the Operators Rule operations of the engine, and the rest through engineMachine.
type oprMachine struct {
	*engineMachine
	oprRules []*Rule
	lists    map[listKey]*listBox
	regs     map[string]any
	zeroMark *markBox
	binder   conform.Binder
}

// listBox is the handle of a list of operators; every empty list is one, as the model has it.
type listBox struct{ s []*OperatorsRuleOp }

type listKey struct {
	first *OperatorsRuleOp
	n     int
}

type attemptBox struct{ a OperatorsRuleAttempt }

type markBox struct{ m Mark }

func newOprMachine(cfg ref.Config) conform.Impl {
	m := &oprMachine{engineMachine: newEngineMachine(cfg, false, false).(*engineMachine), lists: map[listKey]*listBox{}}
	id := 0
	line := func(scope int64) *OperatorsRuleLine {
		return &OperatorsRuleLine{Scope: &compile.Scope{Names: make([]string, scope)}}
	}
	ops := func(os []ref.OprOp) []*OperatorsRuleOp {
		var l []*OperatorsRuleOp
		for _, o := range os {
			id++
			l = append(l, &OperatorsRuleOp{Id: id, Kind: o.Kind, Assoc: o.Assoc, Level: int(o.Level), Line: line(o.Scope)})
		}
		return l
	}
	for _, r := range cfg.Oprs {
		or := &OperatorsRule{Prefix: ops(r.Prefix), Led: ops(r.Led)}
		for _, s := range r.Operands {
			or.Operands = append(or.Operands, line(s))
		}
		if r.Skip {
			or.Skip = func(p *Parser) (*parse.Node, bool) { return nil, true }
		}
		m.oprRules = append(m.oprRules, &Rule{OperatorsRule: or})
	}
	m.zeroMark = &markBox{}
	m.binder = conform.Binder{Methods: confOprBindings(m.p), Conv: oprConv{m}}
	return m
}

func (m *oprMachine) Rule(index int64) any { return m.oprRules[index] }

// UseRegisters hands the machine the registers of the run: opr.attempt reads the position.
func (m *oprMachine) UseRegisters(regs map[string]any) { m.regs = regs }

func (m *oprMachine) Handle(name string) any {
	switch name {
	case conform.OprFrameOp:
		return &Frame{}
	case conform.OprMarkOp:
		return &markBox{m.p.Mark()}
	case conform.OprNodeOp:
		return new(parse.Node)
	case conform.OprRecordOp("oprLongest"):
		return &LongestState{}
	case conform.OprRecordOp("oprNud"):
		return &NudState{}
	case conform.OprRecordOp("oprExpr"):
		return &ExprState{}
	}
	panic("engine binding: no handle " + name)
}

func (m *oprMachine) Call(o *exec.Op, args []any) []any {
	if strings.HasPrefix(o.Name, "opr") {
		if pos, ok := m.regs["pos"].(int64); ok {
			m.p.Pos = int(pos)
		}
		return m.binder.Call(o, args)
	}
	return m.engineMachine.Call(o, args)
}

// oprConv converts the handles of the Operators Rule operations of the engine.
type oprConv struct{ m *oprMachine }

func (c oprConv) ToImpl(arg any, t reflect.Type) (reflect.Value, bool) {
	switch a := arg.(type) {
	case *listBox:
		return reflect.ValueOf(a.s), true
	case *attemptBox:
		return reflect.ValueOf(a.a), true
	case *markBox:
		return reflect.ValueOf(a.m), true
	case *errorList:
		return reflect.ValueOf(a.e), true
	}
	return reflect.Value{}, false
}

func (c oprConv) FromImpl(v reflect.Value) (any, bool) {
	switch x := v.Interface().(type) {
	case []*OperatorsRuleOp:
		k := listKey{n: len(x)}
		if len(x) > 0 {
			k.first = x[0]
		}
		b := c.m.lists[k]
		if b == nil {
			b = &listBox{x}
			c.m.lists[k] = b
		}
		return b, true
	case OperatorsRuleAttempt:
		if x.Op == nil {
			// The null attempt is no handle.
			return nil, true
		}
		return &attemptBox{x}, true
	case Mark:
		if x == (Mark{}) {
			return c.m.zeroMark, true
		}
		return &markBox{x}, true
	case RecEntry:
		return c.m.handle(x), true
	}
	return nil, false
}

// oprSuite is the suite of the engine's Operators Rule operations.
func oprSuite() conform.Suite {
	return conform.Suite{Name: "engine opr", Ops: conform.OprSuiteOps(), New: newOprMachine}
}

// oprConformanceFailures runs the handwritten cases of the Operators Rule family and seeds randomized
// cases of n calls on s, and returns the failures and the number of cases that ran.
func oprConformanceFailures(s conform.Suite, seeds, n int) (fails []error, ran int) {
	run := func(c conform.Case) {
		skipped, err := conform.Run(s, c)
		if !skipped {
			ran++
		}
		if err != nil {
			fails = append(fails, err)
		}
	}
	for _, c := range conform.OprCases() {
		run(c)
	}
	for seed := range seeds {
		rng := rand.New(rand.NewPCG(uint64(seed), 66))
		run(conform.RandomOpr(fmt.Sprint("random_", seed), rng, conform.OprOps, n, conform.OprConfig()))
	}
	return fails, ran
}

func TestEngineOprConformance(t *testing.T) {
	seeds := 300
	if testing.Short() {
		seeds = 30
	}
	fails, ran := oprConformanceFailures(oprSuite(), seeds, 150)
	if ran < seeds {
		t.Fatalf("only %d cases ran", ran)
	}
	for _, err := range fails {
		t.Error(err)
	}
}

// brokenOpr is the engine with one operation of the Operators Rule family replaced.
type brokenOpr struct {
	*oprMachine
	op   string
	edit func(call func(op string, args ...any) []any, args []any) []any
}

func (b brokenOpr) Call(o *exec.Op, args []any) []any {
	if o.Name != b.op {
		return b.oprMachine.Call(o, args)
	}
	mod := conform.OprModule()
	return b.edit(func(op string, a ...any) []any { return b.oprMachine.Call(mod.Op(op), a) }, args)
}

// TestEngineOprConformanceDetectsBrokenEngine is the negative control: the engine with one operation
// of the family broken, as a wrong line of its printed code would break it, fails the suite.
func TestEngineOprConformanceDetectsBrokenEngine(t *testing.T) {
	for _, br := range conform.OprBrokenOps() {
		t.Run(br.Name, func(t *testing.T) {
			s := oprSuite()
			s.New = func(c ref.Config) conform.Impl { return brokenOpr{newOprMachine(c).(*oprMachine), br.Op, br.Edit} }
			if fails, _ := oprConformanceFailures(s, 60, 250); len(fails) == 0 {
				t.Fatal("no case detected the broken engine")
			}
		})
	}
}
