//go:build !zp_budget_controls_reference

package vm

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/core/noderep"
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/engine/state"
	"zeroparse.dev/zp/internal/value"
)

func TestBudgetStateVMChildAssembly(t *testing.T) {
	for _, op := range []bytecode.Op{bytecode.OpSeq, bytecode.OpEndRepeat} {
		for _, lim := range []core.Limits{{Compute: 64}, {Compute: 10000}} {
			p := stateBudgetParser(lim)
			start := 3
			retained := value.Ptr(value.Data{Text: "retained", Fields: noderep.Fields{{Name: "keep", Value: "unchanged"}}})
			machineOf(p).vals = append(machineOf(p).vals, &start)
			want := make([]*core.Node, 320)
			for i := range want {
				if i%3 != 0 {
					want[i] = retained
				}
				machineOf(p).vals = append(machineOf(p).vals, state.NodeOrNil(want[i]))
			}
			instruction := bytecode.Instr{Op: op, A: 320}
			if op == bytecode.OpEndRepeat {
				instruction.A = 1
				machineOf(p).reps = []repState{{base: 1, count: 320}}
			}
			vm := &Program{m: &bytecode.Module{Code: []bytecode.Instr{instruction, {Op: bytecode.OpEnd}}}}
			body := vmBody{}
			var result *core.Node
			err := catchFatal(t, func() { _, result, _ = machineOf(p).step(vm, &body, false, false, nil) })
			if lim.Compute == 64 {
				if !errors.Is(err, core.ErrLimit) || result != nil || len(machineOf(p).vals) != 321 {
					t.Fatalf("%v assembly interruption err=%v result=%v stack=%d", op, err, result, len(machineOf(p).vals))
				}
			} else if err != nil || result == nil || !reflect.DeepEqual(value.D(result).Children, want) || value.D(result).Start != 3 {
				t.Fatalf("%v assembled children/position differs: %v", op, err)
			}
			if value.D(retained).Text != "retained" || value.D(retained).Fields[0].Value != "unchanged" {
				t.Fatal("assembly changed retained node")
			}
		}
	}
}

func TestBudgetStateVMCutAndFailureUnwind(t *testing.T) {
	for _, cut := range []bool{false, true} {
		p := stateBudgetParser(core.Limits{Compute: 64})
		for range 320 {
			machineOf(p).ents = append(machineOf(p).ents, vmEntry{kind: eChoice, cut: true})
		}
		vm := &Program{m: &bytecode.Module{Code: []bytecode.Instr{{Op: bytecode.OpCut}, {Op: bytecode.OpEnd}}}}
		if cut {
			for i := range machineOf(p).ents {
				machineOf(p).ents[i].kind = eLabel
			}
			machineOf(p).ents[0].kind = eChoice
			machineOf(p).ents[0].cut = false
		}
		err := catchFatal(t, func() {
			if cut {
				body := vmBody{}
				machineOf(p).step(vm, &body, false, false, nil)
			} else {
				machineOf(p).unwind(vm.m, 0)
			}
		})
		if !errors.Is(err, core.ErrLimit) {
			t.Fatalf("cut=%t err=%v", cut, err)
		}
		if cut && machineOf(p).ents[0].cut {
			t.Fatal("cut reached its target after budget exhaustion")
		}
		if !cut && len(machineOf(p).ents) != 256 {
			t.Fatalf("unwind retained %d entries, want256", len(machineOf(p).ents))
		}
		p.Control = state.NewBudgetControl(newPollingContext(1), core.Limits{})
		if err := catchFatal(t, func() {
			if cut {
				body := vmBody{}
				machineOf(p).step(vm, &body, false, false, nil)
			} else {
				machineOf(p).unwind(vm.m, 0)
			}
		}); !errors.Is(err, context.Canceled) {
			t.Fatalf("cut=%t cancellation=%v", cut, err)
		}
	}
}

func TestBudgetStateOperatorsRuleFrameSnapshotAndRestoration(t *testing.T) {
	for _, iterative := range []bool{false, true} {
		for _, limit := range []uint64{128, 449, 10000} {
			// The recursive form charges the operator it tries before the copies under test.
			compute := limit
			if !iterative {
				compute++
			}
			p := stateBudgetParser(core.Limits{Compute: compute})
			retained := value.Ptr(value.Data{Text: "retained", Fields: noderep.Fields{{Name: "keep", Value: "unchanged"}}})
			before, _ := json.Marshal(retained)
			op := &state.OperatorsRuleOp{Kind: grammar.Prefix, Line: &state.OperatorsRuleLine{Scope: &compile.Scope{Names: make([]string, 320)}}}
			setup := func(f *state.Frame) {
				for i := range f.Vals {
					f.Vals[i] = retained
				}
				p.Trail = []state.Undo{{F: f, Slot: 0, Old: nil}}
				p.Pos = 1
			}
			var best state.OperatorsRuleAttempt
			err := catchFatal(t, func() {
				if iterative {
					f := &longestFrame{started: true, st: state.LongestState{L: []*state.OperatorsRuleOp{op}, F: p.NewFrame(320)}}
					setup(f.st.F)
					f.next(ivmMachineOf(p), nil, iresult{ok: true, v: retained})
					best = ivmMachineOf(p).attempt
				} else {
					op.Line.M = func(p *state.Parser) (*core.Node, bool) { setup(p.Frame); return retained, true }
					best = p.Longest([]*state.OperatorsRuleOp{op})
				}
			})
			if limit < 10000 {
				if !errors.Is(err, core.ErrLimit) || best.Op != nil {
					t.Fatalf("iterative=%t limit=%d err=%v partial=%v", iterative, limit, err, best.Op)
				}
				if p.Control.Used != compute {
					t.Fatalf("snapshot/restore boundary used=%d want%d", p.Control.Used, compute)
				}
			} else if err != nil || best.Op != op || best.Frame.Vals[0] != retained {
				t.Fatalf("iterative=%t winning captures lost: %v", iterative, err)
			}
			after, _ := json.Marshal(retained)
			if string(before) != string(after) {
				t.Fatal("OperatorsRule frame copy changed public node")
			}
		}
	}
}

func TestBudgetResetAndVMRestoreInterruptionIsFatal(t *testing.T) {
	for _, name := range []string{"reset", "restore"} {
		t.Run(name, func(t *testing.T) {
			p := stateBudgetParser(core.Limits{Compute: 2})
			f, nodes := undoFixture(p)
			p.Frame = f
			p.Pos = 99
			p.SetVars([]any{1})
			p.Recovered = []state.RecEntry{state.ErrEntry(&core.SyntaxError{Messages: []string{"temporary"}})}
			if name == "reset" {
				m := state.Mark{Pos: 4, Trail: 0}
				if err := catchFatal(t, func() { p.Reset(m) }); !errors.Is(err, core.ErrLimit) {
					t.Fatalf("reset interruption = %v, want ErrLimit", err)
				}
				if p.Pos != 4 || len(p.Trail) != 4 {
					t.Fatalf("reset partial state: pos=%d trail=%d", p.Pos, len(p.Trail))
				}
			} else {
				machineOf(p).vals = []any{"keep", "discard-one", "discard-two"}
				machineOf(p).reps = []repState{{count: 1}, {count: 2}}
				save := vmSave{pos: 4, vals: 1, reps: 1, trail: 0, recovered: 0, env: nil, frame: f}
				if err := catchFatal(t, func() { machineOf(p).restore(&save) }); !errors.Is(err, core.ErrLimit) {
					t.Fatalf("VM restore interruption = %v, want ErrLimit", err)
				}
				if p.Pos != 4 || len(machineOf(p).vals) != 1 || len(machineOf(p).reps) != 1 || len(p.Trail) != 4 {
					t.Fatalf("restore partial state: pos=%d vals=%d reps=%d trail=%d", p.Pos, len(machineOf(p).vals), len(machineOf(p).reps), len(p.Trail))
				}
			}
			// The budget permits the two newest undo records; the oldest remains pending.
			if f.Vals[0] != nodes[4] || f.Vals[1] != nodes[2] {
				t.Fatalf("partial reverse restore slots=(%p,%p), want (%p,%p)", f.Vals[0], f.Vals[1], nodes[4], nodes[2])
			}
			if value.D(nodes[0]).Text != "old-zero" || value.D(nodes[0]).Fields[0].Value != "zero" {
				t.Fatal("partial rollback mutated retained node data")
			}
		})
	}
}

func TestBudgetDocumentRunStateClearReservation(t *testing.T) {
	marker := value.Ptr(value.Data{Text: "retained"})
	r := &state.RunState{Old: []state.RunElem{{V: marker}}, Mid: make([]state.RunElem, 129), Post: []state.RunElem{{V: marker}}}
	for i := range r.Mid {
		r.Mid[i].V = marker
	}
	p := stateBudgetParser(core.Limits{Compute: 128})
	mc := machineOf(p)
	if err := catchFatal(t, func() { mc.freeRunState(r) }); !errors.Is(err, core.ErrLimit) {
		t.Fatalf("clear interruption = %v", err)
	}
	if len(mc.freeRuns) != 0 || r.Old != nil || r.Mid[128].V != marker || r.Post[0].V != marker || value.D(marker).Text != "retained" {
		t.Fatal("incomplete clear was published or changed pointees")
	}
	p.Control = state.NewBudgetControl(nil, core.Limits{Compute: 130})
	mc.freeRunState(r)
	if len(mc.freeRuns) != 1 || r.Mid[128].V != nil || r.Post[0].V != nil {
		t.Fatal("completed clear did not release references")
	}
}
