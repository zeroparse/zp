package core

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

// The engine's missing-token mode under conformance: every operation of the mode runs through its
// binding in missing_bind_exec_test.go, which the engine's printer prints from the operation, so that the
// suite runs the code that the engine's compositions run. The tokens a case inserts and the entries it
// seeds go through the functions that the grammar's code calls.

// missingMachine binds the missing-token mode of the engine, and the memo and the recovered errors
// through engineMachine.
type missingMachine struct{ *engineMachine }

type meteredMissing struct{ *missingMachine }

func (m meteredMissing) Used() uint64 { return m.p.Control.Used }

func newMissingMachine(cfg ref.Config) conform.Impl { return newMissingMachineControlled(cfg, false) }

func newMissingMachineControlled(cfg ref.Config, controlled bool) conform.Impl {
	var em *engineMachine
	switch x := newEngineMachine(cfg, controlled, false).(type) {
	case *engineMachine:
		em = x
	case meteredEngine:
		em = x.engineMachine
	}
	m := &missingMachine{em}
	if cfg.Missing {
		m.p.Missing = &MissingTokenState{Limit: 1 << 30}
	}
	for i, r := range m.envRules {
		r.Missing = i < len(cfg.MissingRules) && cfg.MissingRules[i]
	}
	if controlled {
		return meteredMissing{m}
	}
	return m
}

// Observe lists the tokens that the mode holds, as the model's Observe does.
func (m *missingMachine) Observe(component string) []string {
	if component != "recovered" || m.p.Missing == nil {
		return nil
	}
	out := []string{}
	for _, e := range m.p.Missing.Path {
		out = append(out, fmt.Sprintf("site %d at %d error %d", e.Site.ID, e.Site.Pos, e.Err.Pos))
	}
	return out
}

func (m *missingMachine) Call(o *exec.Op, args []any) (res []any) {
	defer interrupt()
	p := m.p
	switch o.Name {
	case "missing.active":
		return []any{p.confMissingActive()}
	case "missing.recompute":
		return []any{p.confMissingRecompute(args[0].(*Rule))}
	case "missing.mark":
		return []any{int64(p.confMissingMark())}
	case "missing.reset":
		p.confMissingReset(int(args[0].(int64)))
		return nil
	case "missing.replay":
		p.confMissingReplay(args[0].(*MemoEntry))
		return nil
	case conform.MissingInsertOp, conform.MissingSeedOp:
		// The functions of the grammar's code are not operations: what they charge is not checked.
		c := p.Control
		p.Control = nil
		defer func() { p.Control = c }()
	}
	switch o.Name {
	case conform.MissingInsertOp:
		e := args[2].(*SyntaxError)
		if !p.InsertMissing(MissingTokenEvent{MissingTokenSite{int(args[0].(int64)), int(args[1].(int64))}, e}) {
			return []any{false}
		}
		p.PushRecovered(e)
		return []any{true}
	case conform.MissingSeedOp:
		p.RecordMissingSeed(args[0].(*MemoEntry), int(args[1].(int64)))
		return nil
	}
	return m.engineMachine.Call(o, args)
}

// missingConformanceFailures runs the handwritten cases of the missing-token mode and seeds randomized
// cases of n calls on s, and returns the failures and the number of cases that ran.
func missingConformanceFailures(s conform.Suite, seeds, n int) (fails []error, ran int) {
	run := func(c conform.Case) {
		skipped, err := conform.Run(s, c)
		if !skipped {
			ran++
		}
		if err != nil {
			fails = append(fails, err)
		}
	}
	for _, c := range conform.MissingCases() {
		run(c)
	}
	for seed := range seeds {
		rng := rand.New(rand.NewPCG(uint64(seed), 66))
		run(conform.RandomMissing(fmt.Sprint("random_", seed), rng, n, seed%4 != 3))
	}
	return fails, ran
}

// missingExact are the operations of the mode whose charges the controlled suite holds to their exact
// cost: one unit for each token that missing.reset removes or missing.replay records again, and for each
// error that missing.replay copies.
var missingExact = []string{"missing.reset", "missing.replay"}

func TestEngineMissingConformance(t *testing.T) {
	seeds := 300
	if testing.Short() {
		seeds = 30
	}
	for _, controlled := range []bool{false, true} {
		ops := slices.Clone(conform.MissingOps)
		if controlled {
			ops = append(ops, engineBudgetOps...)
		}
		s := conform.Suite{Name: fmt.Sprintf("engine missing tokens (controlled=%v)", controlled), Ops: ops, Exact: missingExact,
			New: func(c ref.Config) conform.Impl { return newMissingMachineControlled(c, controlled) }}
		t.Run(s.Name, func(t *testing.T) {
			fails, ran := missingConformanceFailures(s, seeds, 150)
			if ran < seeds {
				t.Fatalf("only %d cases ran", ran)
			}
			for _, err := range fails {
				t.Error(err)
			}
		})
	}
}

// TestEngineMissingConformanceDetectsBrokenEngine is the negative control: the engine with one
// operation of the mode broken, as a wrong line of its printed code would break it, fails the suite.
func TestEngineMissingConformanceDetectsBrokenEngine(t *testing.T) {
	variants := map[string]func(m *missingMachine, args []any) []any{
		"missing.mark counts one token less": func(m *missingMachine, args []any) []any {
			return []any{int64(max(m.p.missingMark()-1, 0))}
		},
		// The tokens go from the path but stay installed, so that they cannot be inserted again.
		"missing.reset leaves the tokens installed": func(m *missingMachine, args []any) []any {
			s, mark := m.p.Missing, int(args[0].(int64))
			s.Path = s.Path[:mark]
			m.p.MissingLen = mark
			return nil
		},
		"missing.reset keeps the tokens": func(m *missingMachine, args []any) []any { return nil },
		"missing.recompute ignores the rule": func(m *missingMachine, args []any) []any {
			return []any{m.p.Missing != nil}
		},
		"missing.replay records the errors alone": func(m *missingMachine, args []any) []any {
			m.p.recAdd(args[0].(*MemoEntry).Errs)
			return nil
		},
		// The errors of tokens that are present are recorded again.
		"missing.replay records the errors of present tokens": func(m *missingMachine, args []any) []any {
			e := args[0].(*MemoEntry)
			for _, ev := range m.p.MissingSeedEvents(e) {
				m.p.installMissing(ev, true)
			}
			m.p.recAdd(e.Errs)
			return nil
		},
		"missing.active is true": func(m *missingMachine, args []any) []any { return []any{true} },
	}
	for name, v := range variants {
		t.Run(name, func(t *testing.T) {
			op := strings.Fields(name)[0]
			s := conform.Suite{Name: name, Ops: conform.MissingOps, New: func(c ref.Config) conform.Impl {
				return brokenMissing{newMissingMachine(c).(*missingMachine), op, v}
			}}
			if fails, _ := missingConformanceFailures(s, 100, 150); len(fails) == 0 {
				t.Fatal("no case detected the broken engine")
			}
		})
	}
}

// TestEngineMissingChargesArePinned is the negative control of the charges: the engine whose
// missing.reset or missing.replay charges one unit more or nothing at all fails the controlled suite,
// so that a change of what they charge for the tokens and the errors fails a test.
func TestEngineMissingChargesArePinned(t *testing.T) {
	uncharged := func(f func(m *missingMachine, args []any)) func(m *missingMachine, args []any) []any {
		return func(m *missingMachine, args []any) []any {
			c := m.p.Control
			m.p.Control = nil
			defer func() { m.p.Control = c }()
			f(m, args)
			return nil
		}
	}
	more := func(f func(m *missingMachine, args []any)) func(m *missingMachine, args []any) []any {
		return func(m *missingMachine, args []any) []any {
			f(m, args)
			if m.p.Missing != nil {
				m.p.Spend()
			}
			return nil
		}
	}
	reset := func(m *missingMachine, args []any) { m.p.confMissingReset(int(args[0].(int64))) }
	replay := func(m *missingMachine, args []any) { m.p.confMissingReplay(args[0].(*MemoEntry)) }
	variants := map[string]func(m *missingMachine, args []any) []any{
		"missing.reset charges nothing":   uncharged(reset),
		"missing.reset charges one more":  more(reset),
		"missing.replay charges nothing":  uncharged(replay),
		"missing.replay charges one more": more(replay),
		"missing.replay charges every error": func(m *missingMachine, args []any) []any {
			replay(m, args)
			for range args[0].(*MemoEntry).Errs.AppendErrors(nil) {
				m.p.Spend()
			}
			return nil
		},
	}
	ops := append(slices.Clone(conform.MissingOps), engineBudgetOps...)
	for name, v := range variants {
		t.Run(name, func(t *testing.T) {
			op := strings.Fields(name)[0]
			s := conform.Suite{Name: name, Ops: ops, Exact: missingExact, New: func(c ref.Config) conform.Impl {
				m := newMissingMachineControlled(c, true).(meteredMissing)
				return meteredBrokenMissing{brokenMissing{m.missingMachine, op, v}}
			}}
			if fails, _ := missingConformanceFailures(s, 100, 150); len(fails) == 0 {
				t.Fatal("no case detected the changed charge")
			}
		})
	}
}

type meteredBrokenMissing struct{ brokenMissing }

func (m meteredBrokenMissing) Used() uint64 { return m.p.Control.Used }

// brokenMissing is the engine with one operation of the mode replaced.
type brokenMissing struct {
	*missingMachine
	op   string
	call func(m *missingMachine, args []any) []any
}

func (b brokenMissing) Call(o *exec.Op, args []any) []any {
	if o.Name == b.op {
		defer interrupt()
		return b.call(b.missingMachine, args)
	}
	return b.missingMachine.Call(o, args)
}
