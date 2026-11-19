package vm

import (
	"testing"

	"zeroparse.dev/zp/internal/runtime/program"
	"zeroparse.dev/zp/internal/runtime/state"
	"zeroparse.dev/zp/pkg/bytecode"
	"zeroparse.dev/zp/pkg/runtime"
	"zeroparse.dev/zp/pkg/runtime/value"
)

// TestFlowFixedExpectations checks that the expectation IDs of Flow bodies count the fixed
// expectations of the parser's table as the module format says: a module file stores the IDs, and
// the VM passes them to the parser as they are.
func TestFlowFixedExpectations(t *testing.T) {
	if bytecode.FixedExps != int(program.NumFixedDescs) {
		t.Fatalf("bytecode.FixedExps = %d; the parser has %d fixed expectations", bytecode.FixedExps, program.NumFixedDescs)
	}
}

// runFlowBody runs the hand-built Flow body code on p with the registers regs set first, until it
// returns, and returns the registers.
func runFlowBody(t *testing.T, p *state.Parser, regs []any, code ...bytecode.FlowInstr) []any {
	t.Helper()
	fr := flowRun{f: &bytecode.Flow{Regs: len(regs), Code: code}, regs: regs}
	if (&Program{}).runFlow(p, &fr) {
		t.Fatal("the body stopped at a CALL")
	}
	return fr.regs
}

// TestFlowRecoveredErrorCount checks RECLEN and RESTORE on the recovered errors directly: in a framed
// body they run only around a lookahead, whose end drops the errors recovered inside it as well, so
// a parse cannot tell a RESTORE that keeps them from one that drops them.
func TestFlowRecoveredErrorCount(t *testing.T) {
	p := stateBudgetParser(runtime.Limits{})
	errs := []*value.SyntaxError{{Pos: 0}, {Pos: 1}, {Pos: 2}}
	for _, e := range errs {
		p.PushRecovered(e)
	}
	p.Pos = 5
	regs := runFlowBody(t, p, []any{nil, 1, 1, nil, true},
		bytecode.FlowInstr{Op: bytecode.FlowRecLen, Dst: []int{0}},
		bytecode.FlowInstr{Op: bytecode.FlowRestore, Args: []int{1, 2}},
		bytecode.FlowInstr{Op: bytecode.FlowRecLen, Dst: []int{3}},
		bytecode.FlowInstr{Op: bytecode.FlowReturn, Args: []int{3, 4}},
	)
	if regs[0] != 3 {
		t.Errorf("RECLEN before RESTORE = %v, want 3", regs[0])
	}
	if regs[3] != 1 {
		t.Errorf("RECLEN after RESTORE = %v, want 1", regs[3])
	}
	if p.Pos != 1 {
		t.Errorf("position after RESTORE = %d, want 1", p.Pos)
	}
	if got := p.RecoveredErrors(); len(got) != 1 || got[0] != errs[0] {
		t.Errorf("recovered errors after RESTORE = %v, want the first one", got)
	}
}

// TestFlowKeepCopies checks that KEEP returns the expectations in storage of the parser's own: the
// expectations of a failed recovery body sit in the record that the skip goes on to overwrite, so a
// result that shared the argument's storage would change with it. Keeping the same expectations
// again returns the same storage, as the match code's recovery does.
func TestFlowKeepCopies(t *testing.T) {
	p := stateBudgetParser(runtime.Limits{})
	exps := []state.ExpID{7, 9}
	regs := runFlowBody(t, p, []any{exps, nil, nil, nil, true},
		bytecode.FlowInstr{Op: bytecode.FlowKeep, Dst: []int{1}, Args: []int{0}},
		bytecode.FlowInstr{Op: bytecode.FlowKeep, Dst: []int{2}, Args: []int{0}},
		bytecode.FlowInstr{Op: bytecode.FlowReturn, Args: []int{3, 4}},
	)
	kept, again := regs[1].([]state.ExpID), regs[2].([]state.ExpID)
	exps[0], exps[1] = 1, 2
	if len(kept) != 2 || kept[0] != 7 || kept[1] != 9 {
		t.Errorf("KEEP = %v after the argument changed, want [7 9]", kept)
	}
	if len(again) != 2 || &again[0] != &kept[0] {
		t.Errorf("KEEP of the same expectations again did not return the kept storage")
	}
	if regs := runFlowBody(t, p, []any{[]state.ExpID(nil), "unset", nil, true},
		bytecode.FlowInstr{Op: bytecode.FlowKeep, Dst: []int{1}, Args: []int{0}},
		bytecode.FlowInstr{Op: bytecode.FlowReturn, Args: []int{2, 3}},
	); regs[1].([]state.ExpID) != nil {
		t.Errorf("KEEP of no expectations = %v, want nil", regs[1])
	}
}
