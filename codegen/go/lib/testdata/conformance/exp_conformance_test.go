package libgo

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"unsafe"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

// The expectation record of the node library under conformance: every operation of the record runs
// through its binding in exp_bind_exec_test.go, which the test that copies the library prints with the
// library's printer, so that the suite runs the code that the library's compositions run.

// expMachine binds the expectation record of the library, and the memo through rtMachine.
type expMachine struct {
	*rtMachine
	metered bool
	// kept are the kept sets by identity: the start of their memory and their length.
	kept map[keptID][]ExpID
}

// keptID is the identity of a kept set of the library, a slice: equal identities are one set.
type keptID struct {
	data *ExpID
	n    int
}

// markHandle and setHandle are new handles for each isolation and each set it returns, which have no
// identity of their own in the library.
type markHandle struct{ m ExpMark }

type setHandle struct{ s []ExpID }

type meteredExp struct{ *expMachine }

func (m meteredExp) Used() uint64 { return m.p.Control.budgetUsed() }

func newExpMachine(cfg ref.Config, controlled bool) conform.Impl {
	var rm *rtMachine
	switch x := newRTMachine(cfg, controlled, false, false).(type) {
	case *rtMachine:
		rm = x
	case meteredRT:
		rm = x.rtMachine
	}
	rm.p.Expectations = !cfg.NoExpectations
	m := &expMachine{rtMachine: rm, kept: map[keptID][]ExpID{}}
	if controlled {
		return meteredExp{m}
	}
	return m
}

func (m *expMachine) Exp(id int64) any { return ExpID(id) }

func (m *expMachine) Placeholder() any { return ExpFar }

// keptHandle returns the handle of the kept set k.
func (m *expMachine) keptHandle(k []ExpID) keptID {
	id := keptID{unsafe.SliceData(k), len(k)}
	if id.n == 0 {
		id.data = nil
	}
	m.kept[id] = k
	return id
}

func (m *expMachine) Call(o *exec.Op, args []any) (res []any) {
	defer interrupted()
	p := m.p
	switch o.Name {
	case "exp.isolate":
		return []any{&markHandle{p.confExpIsolate(int(args[0].(int64)))}}
	case "exp.unisolate":
		far, s := p.confExpUnisolate(args[0].(*markHandle).m)
		return []any{int64(far), &setHandle{s}}
	case "exp.farthest":
		return []any{int64(p.confExpFarthest())}
	case "exp.recording":
		return []any{p.confExpRecording()}
	case "exp.reset":
		p.confExpReset(int(args[0].(int64)))
		return nil
	case "exp.fresh":
		return []any{p.confExpFresh()}
	case "exp.placeholder":
		p.confExpPlaceholder()
		return nil
	case "exp.maybe":
		return []any{p.confExpMaybe(args[0].(ExpID))}
	case "exp.count":
		return []any{int64(p.confExpCount())}
	case "exp.at":
		return []any{p.confExpAt(int(args[0].(int64)))}
	case "exp.add":
		p.confExpAdd(args[0].(ExpID))
		return nil
	case "exp.setCount":
		return []any{int64(p.confExpSetCount(args[0].(*setHandle).s))}
	case "exp.setAt":
		return []any{p.confExpSetAt(args[0].(*setHandle).s, int(args[1].(int64)))}
	case "exp.keptCount":
		return []any{int64(p.confExpKeptCount(m.kept[args[0].(keptID)]))}
	case "exp.keptAt":
		return []any{p.confExpKeptAt(m.kept[args[0].(keptID)], int(args[1].(int64)))}
	case "exp.empty":
		return []any{m.keptHandle(p.confExpEmpty())}
	case "exp.last":
		return []any{m.keptHandle(p.confExpLast())}
	case "exp.copy":
		return []any{m.keptHandle(p.confExpCopy(args[0].(*setHandle).s))}
	case "memo.far":
		return []any{int64(p.confMemoFar(args[0].(*memoEntry)))}
	case "memo.expected":
		return []any{m.keptHandle(p.confMemoExpected(args[0].(*memoEntry)))}
	case "memo.setExpected":
		p.confMemoSetExpected(args[0].(*memoEntry), int(args[1].(int64)), m.kept[args[2].(keptID)])
		return nil
	}
	return m.rtMachine.Call(o, args)
}

// expSuites are the suites of the library's expectation record: with and without charge controls, and
// with the Document memo, whose edits move the expectations of an entry.
func expSuites() []conform.Suite {
	var suites []conform.Suite
	for _, controlled := range []bool{false, true} {
		ops := slices.Clone(conform.ExpOps)
		if controlled {
			ops = append(ops, rtBudgetOps...)
		}
		suites = append(suites, conform.Suite{
			Name: fmt.Sprintf("node library expectations (controlled=%v)", controlled),
			Ops:  ops,
			New:  func(c ref.Config) conform.Impl { return newExpMachine(c, controlled) },
		})
	}
	return suites
}

// expConformanceFailures runs the handwritten cases of the expectation record and seeds randomized
// cases of n calls on s, and returns the failures and the number of cases that ran.
func expConformanceFailures(s conform.Suite, seeds, n int) (fails []error, ran int) {
	run := func(c conform.Case) {
		skipped, err := conform.Run(s, c)
		if !skipped {
			ran++
		}
		if err != nil {
			fails = append(fails, err)
		}
	}
	for _, c := range conform.ExpCases() {
		run(c)
	}
	for seed := range seeds {
		rng := rand.New(rand.NewPCG(uint64(seed), 66))
		run(conform.RandomExp(fmt.Sprint("random_", seed), rng, conform.RecordOps, n, ref.Config{NoExpectations: seed%3 == 2}))
	}
	return fails, ran
}

func TestExecConformanceExp(t *testing.T) {
	seeds := 300
	if testing.Short() {
		seeds = 30
	}
	for _, s := range expSuites() {
		t.Run(s.Name, func(t *testing.T) {
			fails, ran := expConformanceFailures(s, seeds, 150)
			if ran < seeds {
				t.Fatalf("only %d cases ran", ran)
			}
			for _, err := range fails {
				t.Error(err)
			}
		})
	}
}

// TestExecConformanceExpDetectsBrokenLib is the negative control: the library with one operation
// of the record broken, as a wrong line of its printed code would break it, fails the suite.
func TestExecConformanceExpDetectsBrokenLib(t *testing.T) {
	variants := map[string]func(m *expMachine, args []any) []any{
		"exp.reset keeps the entries": func(m *expMachine, args []any) []any {
			m.p.farthest = int(args[0].(int64))
			return nil
		},
		// maybe then answers false for an entry of the record.
		"exp.add leaves the filter": func(m *expMachine, args []any) []any {
			m.p.exp = append(m.p.exp, args[0].(ExpID))
			return nil
		},
		"exp.unisolate returns the restored record": func(m *expMachine, args []any) []any {
			mk := args[0].(*markHandle).m
			far, _ := m.p.Unisolate(mk)
			return []any{int64(far), &setHandle{m.p.exp[mk.base:]}}
		},
		// The kept set is the set itself, which the record overwrites.
		"exp.copy aliases the record": func(m *expMachine, args []any) []any {
			s := args[0].(*setHandle).s
			m.p.lastKept = s
			return []any{m.keptHandle(s)}
		},
		"exp.isolate keeps the farthest position": func(m *expMachine, args []any) []any {
			return []any{&markHandle{m.p.Isolate(m.p.farthest)}}
		},
	}
	for name, v := range variants {
		t.Run(name, func(t *testing.T) {
			op := strings.Fields(name)[0]
			s := conform.Suite{Name: name, Ops: conform.ExpOps, New: func(c ref.Config) conform.Impl {
				return brokenExp{newExpMachine(c, false).(*expMachine), op, v}
			}}
			if fails, _ := expConformanceFailures(s, 100, 150); len(fails) == 0 {
				t.Fatal("no case detected the broken library")
			}
		})
	}
}

// brokenExp is the library with one operation of the record replaced.
type brokenExp struct {
	*expMachine
	op   string
	call func(m *expMachine, args []any) []any
}

func (b brokenExp) Call(o *exec.Op, args []any) []any {
	if o.Name == b.op {
		return b.call(b.expMachine, args)
	}
	return b.expMachine.Call(o, args)
}
