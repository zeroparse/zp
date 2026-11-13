package state

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

// The engine's doc operations under conformance: every operation runs through its binding in
// doc_bind_exec_test.go, which the engine's printer prints from the operation, so that the suite runs
// the code that the engine's compositions run. The engine does not observe calls, so it has neither
// doc.examinedBegin nor doc.examinedEnd; the cases that call them are skipped.

// docMachine binds the doc operations of the engine.
type docMachine struct{ *engineMachine }

func newDocMachine(cfg ref.Config) conform.Impl {
	em := newEngineMachine(cfg, false, false).(*engineMachine)
	em.p.MemoAll = cfg.Document
	return &docMachine{em}
}

func (m *docMachine) DocStat(i int64) int64 {
	if i == 0 {
		return int64(m.p.Stats.Reused)
	}
	return int64(m.p.Stats.Evaluated)
}

func (m *docMachine) Call(o *exec.Op, args []any) (res []any) {
	defer interrupt()
	p := m.p
	switch o.Name {
	case "doc.enter":
		high, low := p.confDocEnter(int(args[0].(int64)))
		return []any{int64(high), int64(low)}
	case "doc.leave":
		p.confDocLeave(int(args[0].(int64)), int(args[1].(int64)))
		return nil
	case "doc.active":
		return []any{p.confDocActive()}
	case "doc.touch":
		p.confDocTouch(int(args[0].(int64)))
		return nil
	case "doc.touchFrom":
		p.confDocTouchFrom(int(args[0].(int64)))
		return nil
	case "doc.low":
		return []any{int64(p.confDocLow())}
	case "doc.high":
		return []any{int64(p.confDocHigh())}
	case "doc.reused":
		p.confDocReused()
		return nil
	case "doc.evaluated":
		p.confDocEvaluated()
		return nil
	}
	return m.engineMachine.Call(o, args)
}

// docSuite is the suite of the engine's doc operations: its doc.enter starts the range of a call as
// the empty range at its position.
func docSuite() conform.Suite {
	return conform.Suite{Name: "engine doc", Ops: conform.DocCoreOps, DocEmptyEnter: true, New: newDocMachine}
}

// docConformanceFailures runs the handwritten cases of the doc operations and seeds randomized cases
// of n calls on s, and returns the failures and the number of cases that ran.
func docConformanceFailures(s conform.Suite, seeds, n int) (fails []error, ran int) {
	run := func(c conform.Case) {
		skipped, err := conform.Run(s, c)
		if !skipped {
			ran++
		}
		if err != nil {
			fails = append(fails, err)
		}
	}
	for _, c := range conform.DocCases() {
		run(c)
	}
	for seed := range seeds {
		rng := rand.New(rand.NewPCG(uint64(seed), 66))
		run(conform.RandomDoc(fmt.Sprint("random_", seed), rng, s.Ops, n, ref.Config{Document: seed%3 != 2}))
	}
	return fails, ran
}

func TestEngineDocConformance(t *testing.T) {
	seeds := 300
	if testing.Short() {
		seeds = 30
	}
	s := docSuite()
	fails, ran := docConformanceFailures(s, seeds, 150)
	if ran < seeds {
		t.Fatalf("only %d cases ran", ran)
	}
	for _, err := range fails {
		t.Error(err)
	}
}

// TestEngineDocConformanceDetectsBrokenEngine is the negative control: the engine with one doc
// operation broken, as a wrong line of its printed code would break it, fails the suite.
func TestEngineDocConformanceDetectsBrokenEngine(t *testing.T) {
	variants := map[string]func(m *docMachine, args []any) []any{
		"doc.enter keeps the range of the enclosing call": func(m *docMachine, args []any) []any {
			high, low := m.p.Hw, m.p.lw
			m.p.confDocEnter(int(args[0].(int64)))
			m.p.Hw, m.p.lw = max(m.p.Hw, high), min(m.p.lw, low)
			return []any{int64(high), int64(low)}
		},
		"doc.enter starts one unit past": func(m *docMachine, args []any) []any {
			high, low := m.p.confDocEnter(int(args[0].(int64)))
			m.p.Hw++
			return []any{int64(high), int64(low)}
		},
		"doc.leave ignores the start of the call": func(m *docMachine, args []any) []any {
			m.p.Hw = max(int(args[0].(int64)), m.p.Hw)
			return nil
		},
		"doc.touch moves the frontier": func(m *docMachine, args []any) []any {
			m.p.Hw = int(args[0].(int64))
			return nil
		},
		"doc.touchFrom moves the start": func(m *docMachine, args []any) []any {
			m.p.lw = int(args[0].(int64))
			return nil
		},
		"doc.reused counts evaluations": func(m *docMachine, args []any) []any {
			m.p.Stats.Evaluated++
			return nil
		},
		"doc.active is false": func(m *docMachine, args []any) []any { return []any{false} },
	}
	for name, v := range variants {
		t.Run(name, func(t *testing.T) {
			op := strings.Fields(name)[0]
			s := docSuite()
			s.New = func(c ref.Config) conform.Impl { return brokenDoc{newDocMachine(c).(*docMachine), op, v} }
			if fails, _ := docConformanceFailures(s, 100, 150); len(fails) == 0 {
				t.Fatal("no case detected the broken engine")
			}
		})
	}
}

// brokenDoc is the engine with one doc operation replaced.
type brokenDoc struct {
	*docMachine
	op   string
	call func(m *docMachine, args []any) []any
}

func (b brokenDoc) Call(o *exec.Op, args []any) []any {
	if o.Name == b.op {
		return b.call(b.docMachine, args)
	}
	return b.docMachine.Call(o, args)
}
