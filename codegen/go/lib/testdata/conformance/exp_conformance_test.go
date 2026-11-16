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
	kept    keptSets
}

// keptSets are the kept sets by identity: the start of their memory and their length.
type keptSets map[keptID][]ExpID

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
	m := &expMachine{rtMachine: rm, kept: keptSets{}}
	if controlled {
		return meteredExp{m}
	}
	return m
}

func (m *expMachine) Exp(id int64) any { return ExpID(id) }

func (m *expMachine) Placeholder() any { return ExpFar }

// keptHandle returns the handle of the kept set k.
func (m *expMachine) keptHandle(k []ExpID) keptID { return m.kept.handle(k) }

// handle returns the handle of the kept set k.
func (ks keptSets) handle(k []ExpID) keptID {
	id := keptID{unsafe.SliceData(k), len(k)}
	if id.n == 0 {
		id.data = nil
	}
	ks[id] = k
	return id
}

func (m *expMachine) Call(o *exec.Op, args []any) (res []any) {
	defer interrupted()
	if res, ok := m.kept.callRecord(m.p, o, args); ok {
		return res
	}
	p := m.p
	switch o.Name {
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

// callRecord runs an operation of the expectation record of p, which the node and the typed library
// share, and reports false for any other.
func (ks keptSets) callRecord(p *Parser, o *exec.Op, args []any) ([]any, bool) {
	res := func(r ...any) ([]any, bool) { return r, true }
	switch o.Name {
	case "exp.isolate":
		return res(&markHandle{p.confExpIsolate(int(args[0].(int64)))})
	case "exp.unisolate":
		far, s := p.confExpUnisolate(args[0].(*markHandle).m)
		return res(int64(far), &setHandle{s})
	case "exp.farthest":
		return res(int64(p.confExpFarthest()))
	case "exp.recording":
		return res(p.confExpRecording())
	case "exp.reset":
		p.confExpReset(int(args[0].(int64)))
		return nil, true
	case "exp.fresh":
		return res(p.confExpFresh())
	case "exp.placeholder":
		p.confExpPlaceholder()
		return nil, true
	case "exp.maybe":
		return res(p.confExpMaybe(args[0].(ExpID)))
	case "exp.count":
		return res(int64(p.confExpCount()))
	case "exp.at":
		return res(p.confExpAt(int(args[0].(int64))))
	case "exp.add":
		p.confExpAdd(args[0].(ExpID))
		return nil, true
	case "exp.setCount":
		return res(int64(p.confExpSetCount(args[0].(*setHandle).s)))
	case "exp.setAt":
		return res(p.confExpSetAt(args[0].(*setHandle).s, int(args[1].(int64))))
	case "exp.keptCount":
		return res(int64(p.confExpKeptCount(ks[args[0].(keptID)])))
	case "exp.keptAt":
		return res(p.confExpKeptAt(ks[args[0].(keptID)], int(args[1].(int64))))
	case "exp.empty":
		return res(ks.handle(p.confExpEmpty()))
	case "exp.last":
		return res(ks.handle(p.confExpLast()))
	case "exp.copy":
		return res(ks.handle(p.confExpCopy(args[0].(*setHandle).s)))
	}
	return nil, false
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
		}, conform.Suite{
			Name: fmt.Sprintf("typed library expectations (controlled=%v)", controlled),
			Ops:  ops,
			New:  func(c ref.Config) conform.Impl { return newTExpMachine(c, controlled) },
		})
	}
	return suites
}

// texpMachine binds the expectation record of the typed library, which is its node Parser's, the
// operations of the memo that keep the expectations of its entries, and the memo through typedMachine.
type texpMachine struct {
	*typedMachine
	kept keptSets
}

type meteredTExp struct{ *texpMachine }

func (m meteredTExp) Used() uint64 { return m.p.Control.budgetUsed() }

func newTExpMachine(cfg ref.Config, controlled bool) conform.Impl {
	var tm *typedMachine
	switch x := newTypedMachine(cfg, controlled).(type) {
	case *typedMachine:
		tm = x
	case meteredTyped:
		tm = x.typedMachine
	}
	tm.p.Expectations = !cfg.NoExpectations
	m := &texpMachine{typedMachine: tm, kept: keptSets{}}
	if controlled {
		return meteredTExp{m}
	}
	return m
}

func (m *texpMachine) Exp(id int64) any { return ExpID(id) }

func (m *texpMachine) Placeholder() any { return ExpFar }

func (m *texpMachine) Call(o *exec.Op, args []any) (res []any) {
	defer interrupted()
	if res, ok := m.kept.callRecord(m.p.Parser, o, args); ok {
		return res
	}
	p := m.p
	switch o.Name {
	case "memo.far":
		return []any{int64(p.confMemoFar(args[0].(*tmemoEntry)))}
	case "memo.expected":
		return []any{m.kept.handle(p.confMemoExpected(args[0].(*tmemoEntry)))}
	case "memo.setExpected":
		p.confMemoSetExpected(args[0].(*tmemoEntry), int(args[1].(int64)), m.kept[args[2].(keptID)])
		return nil
	}
	return m.typedMachine.Call(o, args)
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
		"memo.far reads the end of the entry": func(m *expMachine, args []any) []any {
			return []any{int64(args[0].(*memoEntry).end)}
		},
		"memo.expected returns the set kept last": func(m *expMachine, args []any) []any {
			return []any{m.keptHandle(m.p.lastKept)}
		},
		"memo.setExpected keeps the far position": func(m *expMachine, args []any) []any {
			args[0].(*memoEntry).expected = m.kept[args[2].(keptID)]
			return nil
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

// TestExecConformanceTypedExpDetectsBrokenLib is the negative control of the typed library: an operation
// of the memo that keeps the expectations of an entry broken, as a wrong line of its printed code would
// break it, fails the suite.
func TestExecConformanceTypedExpDetectsBrokenLib(t *testing.T) {
	variants := map[string]func(m *texpMachine, args []any) []any{
		"memo.far reads the end of the entry": func(m *texpMachine, args []any) []any {
			return []any{int64(args[0].(*tmemoEntry).end)}
		},
		"memo.expected returns the set kept last": func(m *texpMachine, args []any) []any {
			return []any{m.kept.handle(m.p.lastKept)}
		},
		"memo.setExpected keeps the far position": func(m *texpMachine, args []any) []any {
			args[0].(*tmemoEntry).expected = m.kept[args[2].(keptID)]
			return nil
		},
	}
	for name, v := range variants {
		t.Run(name, func(t *testing.T) {
			op := strings.Fields(name)[0]
			s := expSuites()[1]
			s.New = func(c ref.Config) conform.Impl {
				return brokenTExp{newTExpMachine(c, false).(*texpMachine), op, v}
			}
			if fails, _ := expConformanceFailures(s, 10, 150); len(fails) == 0 {
				t.Fatal("no case detected the broken library")
			}
		})
	}
}

// brokenTExp is the typed library with one operation replaced.
type brokenTExp struct {
	*texpMachine
	op   string
	call func(m *texpMachine, args []any) []any
}

func (b brokenTExp) Call(o *exec.Op, args []any) []any {
	if o.Name == b.op {
		return b.call(b.texpMachine, args)
	}
	return b.texpMachine.Call(o, args)
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

// brokenExpCharge and brokenTExpCharge are the metered node and typed library whose op charges one unit
// beyond its work.
type brokenExpCharge struct {
	meteredExp
	op string
}

func (b brokenExpCharge) Call(o *exec.Op, args []any) []any {
	res := b.meteredExp.Call(o, args)
	if o.Name == b.op {
		b.p.Control.Charge(1)
	}
	return res
}

type brokenTExpCharge struct {
	meteredTExp
	op string
}

func (b brokenTExpCharge) Call(o *exec.Op, args []any) []any {
	res := b.meteredTExp.Call(o, args)
	if o.Name == b.op {
		b.p.Control.Charge(1)
	}
	return res
}

// TestExecConformanceExpDetectsCharges is the negative control of the charges: the metered library whose
// operation of the memo's expectations charges a unit, which its cost bound does not allow, fails the
// suite.
func TestExecConformanceExpDetectsCharges(t *testing.T) {
	newMetered := map[string]func(c ref.Config, op string) conform.Impl{
		"node": func(c ref.Config, op string) conform.Impl {
			return brokenExpCharge{newExpMachine(c, true).(meteredExp), op}
		},
		"typed": func(c ref.Config, op string) conform.Impl {
			return brokenTExpCharge{newTExpMachine(c, true).(meteredTExp), op}
		},
	}
	for lib, suite := range map[string]conform.Suite{"node": expSuites()[2], "typed": expSuites()[3]} {
		for _, op := range []string{"memo.far", "memo.expected", "memo.setExpected"} {
			t.Run(lib+" "+op+" charges a unit", func(t *testing.T) {
				s := suite
				s.New = func(c ref.Config) conform.Impl { return newMetered[lib](c, op) }
				fails, _ := expConformanceFailures(s, 10, 150)
				if len(fails) == 0 {
					t.Fatal("no case detected the charge")
				}
				if msg := fails[0].Error(); !strings.Contains(msg, "more than the bound") {
					t.Fatalf("the first failure is not the charge: %s", msg)
				}
			})
		}
	}
}
