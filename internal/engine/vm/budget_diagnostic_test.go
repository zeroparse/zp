//go:build !zp_budget_controls_reference

package vm

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/engine/core"
	"zeroparse.dev/zp/internal/value"
	"zeroparse.dev/zp/runtime/noderep"
	"zeroparse.dev/zp/runtime/parse"
)

// TestBudgetDiagnosticRecoveredLists checks the lists of recovered errors under the controls: a
// snapshot (rec.since) copies the entries after its mark, charging each, and an interruption
// publishes no part of it; a replay (rec.add) records the list as one entry, sharing it, at no
// charge, so it has no copy to interrupt and runs only to completion.
func TestBudgetDiagnosticRecoveredLists(t *testing.T) {
	for _, name := range []string{"memo replay", "memo snapshot", "LR snapshot", "LR replay", "OperatorsRule snapshot", "iterative OperatorsRule snapshot", "OperatorsRule replay"} {
		for _, mode := range []string{"complete", "budget", "cancel"} {
			replay := strings.HasSuffix(name, "replay")
			if replay && mode != "complete" {
				continue
			}
			t.Run(name+"/"+mode, func(t *testing.T) {
				a := &parse.SyntaxError{Messages: []string{"retained-a"}}
				b := &parse.SyntaxError{Expected: []string{"retained-b"}}
				prefix := &parse.SyntaxError{Messages: []string{"prefix"}}
				records := make([]*parse.SyntaxError, 320)
				for i := range records {
					records[i] = []*parse.SyntaxError{a, b, a}[i%3]
				}
				before, _ := json.Marshal(records)
				retained := value.Ptr(value.Data{Text: "retained", Fields: noderep.Fields{{Name: "keep", Value: "stable"}}})
				p := diagnosticParser(parse.Limits{})
				list := core.NewErrList(records)
				p.Memo, p.Recovered = core.NewMemoTable(), []core.RecEntry{core.ErrEntry(prefix)}
				p.Memo.Slots = make([]*core.MemoEntry, 1)
				r := &core.Rule{ID: 7}
				key := core.MemoKey{Rule: 7}
				steps := uint64(len(records))
				if replay {
					steps = 0
				}
				var run, completed, aborted func()
				assertReplay := func() {
					if len(p.Recovered) != 2 || p.Recovered[0] != core.ErrEntry(prefix) || p.Recovered[1].List() != list.List() {
						t.Fatal("recovered replay changed the prefix or did not share the list")
					}
					if !slices.Equal(p.RecoveredErrors(), append([]*parse.SyntaxError{prefix}, records...)) {
						t.Fatal("recovered replay changed pointer order")
					}
				}
				assertSnapshot := func(got core.RecEntry) {
					if got.List() == nil || !slices.Equal(got.Errors(), records) {
						t.Fatal("snapshot lost pointers")
					}
				}
				pushRecords := func() {
					for _, e := range records {
						p.PushRecovered(e)
					}
				}
				aborted = func() {}
				switch name {
				case "memo replay":
					p.Memo.Slots[0] = &core.MemoEntry{Rule: 7, Ok: true, End: 1, Node: retained, Errs: list}
					steps += 2 // lookup and entry visit; the replay is free
					var ok, hit bool
					var val *parse.Node
					run = func() { val, ok, hit = p.CallBegin(r, 0, &core.CallState{}) }
					completed = func() {
						assertReplay()
						if !ok || !hit || val != retained {
							t.Fatal("memo hit result differs")
						}
					}
				case "memo snapshot":
					// A call at 0 that begins after the prefix and recovers the records.
					var st core.CallState
					p.CallBegin(r, 0, &st)
					p.Pos = 1
					pushRecords()
					steps += 2 // atomic memo put after the snapshot
					run = func() { p.CallEnd(r, &st, retained, true) }
					completed = func() {
						assertSnapshot(p.Memo.Slots[0].Errs)
						if len(p.Recovered) != 2 || p.Recovered[1].List() != p.Memo.Slots[0].Errs.List() {
							t.Fatal("the snapshot did not take the place of the records")
						}
					}
					aborted = func() {
						if p.Memo.Slots[0] != nil {
							t.Fatal("partial memo snapshot published")
						}
					}
				case "LR snapshot":
					g := p.GrowBegin(key, &core.Rule{})
					seed := g.Best
					p.Pos = 1
					pushRecords()
					steps++ // the growth step
					run = func() {
						if p.GrowStep(&g, retained, true); !p.GrowNext(&g) {
							t.Fatal("LR seed did not grow")
						}
					}
					completed = func() {
						assertSnapshot(g.Best.Errs)
						if s, _ := p.GrowingSeed(key); s != g.Best || len(p.Recovered) != 1 {
							t.Fatal("LR publication state differs")
						}
					}
					aborted = func() {
						if s, _ := p.GrowingSeed(key); s != seed {
							t.Fatal("partial LR seed published")
						}
					}
				case "LR replay":
					g := core.GrowState{Rec: 1, Best: &core.MemoEntry{Ok: true, End: 1, Node: retained, Errs: list}}
					run = func() { p.GrowEnd(r, &g) }
					completed = assertReplay
				case "OperatorsRule snapshot", "iterative OperatorsRule snapshot":
					op := &core.OperatorsRuleOp{Kind: grammar.Prefix, Line: &core.OperatorsRuleLine{Scope: &compile.Scope{}}}
					var best core.OperatorsRuleAttempt
					if name == "OperatorsRule snapshot" {
						steps++ // the operator that the match tries
						op.Line.M = func(p *core.Parser) (*parse.Node, bool) {
							p.Pos = 1
							pushRecords()
							return retained, true
						}
						run = func() { best = p.Longest([]*core.OperatorsRuleOp{op}) }
					} else {
						p.Pos = 1
						pushRecords()
						f := &longestFrame{started: true, st: core.LongestState{L: []*core.OperatorsRuleOp{op}, M0: core.Mark{Recovered: 1}, F: p.NewFrame(0)}}
						run = func() { f.next(ivmMachineOf(p), nil, iresult{ok: true, v: retained}); best = ivmMachineOf(p).attempt }
					}
					completed = func() {
						assertSnapshot(best.Errs)
						if best.Op != op || best.V != retained || len(p.Recovered) != 1 {
							t.Fatal("OperatorsRule winner or rollback differs")
						}
					}
					aborted = func() {
						if best.Op != nil || ivmMachineOf(p).attempt.Op != nil {
							t.Fatal("partial OperatorsRule winner returned")
						}
					}
				case "OperatorsRule replay":
					run = func() { p.ApplyRecovered(&core.OperatorsRuleAttempt{End: 1, Errs: list}) }
					completed = assertReplay
				}
				if mode == "complete" {
					// A limit of 0 is no limit; a free replay runs under a limit of one unit.
					p.Control = core.NewBudgetControl(nil, parse.Limits{Compute: max(steps, 1)})
				} else if mode == "budget" {
					// Less than the copy itself, so failure cannot be at a later put.
					p.Control = core.NewBudgetControl(nil, parse.Limits{Compute: 319})
				} else {
					p.Control = core.NewBudgetControl(newPollingContext(1), parse.Limits{})
				}
				err := catchFatal(t, run)
				if mode == "complete" {
					if err != nil || p.Control.Used != steps {
						t.Fatalf("copy error=%v steps=%d want%d", err, p.Control.Used, steps)
					}
					completed()
				} else {
					want := parse.ErrLimit
					if mode == "cancel" {
						want = context.Canceled
					}
					if !errors.Is(err, want) {
						t.Fatalf("copy interruption=%v want%v", err, want)
					}
					aborted()
				}
				after, _ := json.Marshal(records)
				if string(before) != string(after) || records[0] != a || value.D(retained).Text != "retained" || value.D(retained).Fields[0].Value != "stable" {
					t.Fatal("copy changed retained errors, alias pointers or Node")
				}
			})
		}
	}
}
