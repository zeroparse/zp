package runtime

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

// The observation of the node runtime under conformance: every operation of the observation runs
// through its binding in obs_bind_exec_test.go, which the test that copies the runtime prints with the
// runtime's printer, so that the suite runs the code that the runtime's compositions run. The events
// that the runtime delivers to the trace of the parse are what the model's events stand for.

// obsMachine binds the observation of the runtime, and the rest through rtMachine.
type obsMachine struct {
	*rtMachine
	events []string
}

func newObsMachine(cfg ref.Config) conform.Impl {
	m := &obsMachine{rtMachine: newRTMachine(cfg, false, false, false).(*rtMachine)}
	for i, r := range m.env.rules {
		r.Name = ref.RuleName(int64(i))
	}
	if cfg.Observed {
		m.p.Observation = &observation{trace: m.deliver}
	}
	return m
}

// deliver records an event of the trace as the model lists it.
func (m *obsMachine) deliver(e TraceEvent) {
	switch e.Kind {
	case TraceEnter:
		m.events = append(m.events, fmt.Sprintf("enter %s pos=%d level=%d depth=%d", e.Rule, e.Pos, e.Level, e.Depth))
	case TraceExit:
		m.events = append(m.events, fmt.Sprintf("exit %s matched=%v end=%d examined=%d known=%v memo=%v depth=%d",
			e.Rule, e.Matched, e.End, e.Examined, e.ExaminedKnown, e.Memo, e.Depth))
	}
}

func (m *obsMachine) Observe(component string) []string {
	if component != "observe" {
		return nil
	}
	return append([]string{}, m.events...)
}

func (m *obsMachine) Call(o *exec.Op, args []any) (res []any) {
	defer interrupted()
	p := m.p
	switch o.Name {
	case "obs.active":
		return []any{p.confObsActive()}
	case "obs.memoHit":
		p.confObsMemoHit()
		return nil
	case "obs.enter":
		p.confObsEnter(args[0].(*Rule), int(args[1].(int64)), int(args[2].(int64)))
		return nil
	case "obs.exit":
		p.confObsExit(args[0].(bool), int(args[1].(int64)), int(args[2].(int64)), args[3].(bool))
		return nil
	}
	return m.rtMachine.Call(o, args)
}

// obsConformanceFailures runs the handwritten cases of the observation and seeds randomized cases of n
// calls on s, and returns the failures and the number of cases that ran.
func obsConformanceFailures(s conform.Suite, seeds, n int) (fails []error, ran int) {
	run := func(c conform.Case) {
		skipped, err := conform.Run(s, c)
		if !skipped {
			ran++
		}
		if err != nil {
			fails = append(fails, err)
		}
	}
	for _, c := range conform.ObsCases() {
		run(c)
	}
	for seed := range seeds {
		rng := rand.New(rand.NewPCG(uint64(seed), 66))
		run(conform.RandomObs(fmt.Sprint("random_", seed), rng, n, seed%4 != 3))
	}
	return fails, ran
}

func TestExecConformanceObs(t *testing.T) {
	seeds := 300
	if testing.Short() {
		seeds = 30
	}
	s := conform.Suite{Name: "node runtime observation", Ops: conform.ObsOps, New: newObsMachine}
	fails, ran := obsConformanceFailures(s, seeds, 150)
	if ran < seeds {
		t.Fatalf("only %d cases ran", ran)
	}
	for _, err := range fails {
		t.Error(err)
	}
}

// TestExecConformanceObsDetectsBrokenRuntime is the negative control: the runtime with one operation of
// the observation broken, as a wrong line of its printed code would break it, fails the suite.
func TestExecConformanceObsDetectsBrokenRuntime(t *testing.T) {
	variants := map[string]func(m *obsMachine, args []any) []any{
		"obs.memoHit marks nothing": func(m *obsMachine, args []any) []any { return nil },
		"obs.memoHit marks the outermost call": func(m *obsMachine, args []any) []any {
			if o := m.p.Observation; o != nil && len(o.stack) > 0 {
				o.stack[0].event.Memo = true
			}
			return nil
		},
		"obs.enter reports the binding level of the parent": func(m *obsMachine, args []any) []any {
			m.p.Observation.Enter(args[0].(*Rule).Name, int(args[1].(int64)), 0)
			return nil
		},
		"obs.exit reports the examined input without being told": func(m *obsMachine, args []any) []any {
			m.p.Observation.Exit(args[0].(bool), int(args[1].(int64)), int(args[2].(int64)), true)
			return nil
		},
		"obs.exit reports a failure": func(m *obsMachine, args []any) []any {
			m.p.Observation.Exit(false, int(args[1].(int64)), int(args[2].(int64)), args[3].(bool))
			return nil
		},
		"obs.active is true": func(m *obsMachine, args []any) []any { return []any{true} },
	}
	for name, v := range variants {
		t.Run(name, func(t *testing.T) {
			op := strings.Fields(name)[0]
			s := conform.Suite{Name: name, Ops: conform.ObsOps, New: func(c ref.Config) conform.Impl {
				return brokenObs{newObsMachine(c).(*obsMachine), op, v}
			}}
			if fails, _ := obsConformanceFailures(s, 100, 150); len(fails) == 0 {
				t.Fatal("no case detected the broken runtime")
			}
		})
	}
}

// brokenObs is the runtime with one operation of the observation replaced.
type brokenObs struct {
	*obsMachine
	op   string
	call func(m *obsMachine, args []any) []any
}

func (b brokenObs) Call(o *exec.Op, args []any) []any {
	if o.Name == b.op {
		return b.call(b.obsMachine, args)
	}
	return b.obsMachine.Call(o, args)
}
