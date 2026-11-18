package actionlower

import (
	"slices"
	"testing"

	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/exec"
	"zeroparse.dev/zp/internal/runtime/state"
)

// TestTermDeclareKeepsTheFile checks that a local keeps the register file of its first
// declaration: declaring it again in another file is an error instead of a second register that
// its readers would not see.
func TestTermDeclareKeepsTheFile(t *testing.T) {
	c := &termCompiler{locals: map[string]termReg{}, prog: &state.TermProgram{}}
	n, err := c.declare("x", code.Type{Kind: code.KInt})
	if err != nil {
		t.Fatal(err)
	}
	if again, err := c.declare("x", code.Type{Kind: code.KBool}); err != nil || again != n {
		t.Errorf("declaring x again in its file gives %v, %v; want %v", again, err, n)
	}
	if r, err := c.declare("x", code.Type{Kind: code.KHandle, Handle: exec.Datum}); err == nil {
		t.Errorf("declaring the int x as a value gives the register %v, want an error", r)
	}
	if r, err := c.declare("y", code.Type{Kind: code.KHandle, Handle: "Nope"}); err == nil {
		t.Errorf("a local of a type without a register file gets %v, want an error", r)
	}
}

// TestTermEmitFuses checks the instructions the term compiler fuses as it emits them: a step, the
// field of a struct and the cover of its value become FIELD, and a move just before a return
// becomes the return of its source unless a jump goes to the return, whose other paths need the
// move.
func TestTermEmitFuses(t *testing.T) {
	emit := func(code ...state.TermInstr) []state.TermInstr {
		c := &termCompiler{prog: &state.TermProgram{}}
		for _, in := range code {
			c.emit(in)
		}
		return c.prog.Code
	}
	step := state.TermInstr{Op: state.TStep}
	set := state.TermInstr{Op: state.TSetField, A: 1, C: 2}
	for _, c := range []struct {
		name string
		code []state.TermInstr
		want []state.TermInstr
	}{
		{"field", []state.TermInstr{step, set, {Op: state.TCover, A: 1, Dst: 3, C: 3}},
			[]state.TermInstr{{Op: state.TField, Dst: 3, A: 1, C: 2}}},
		{"cover into another register", []state.TermInstr{step, set, {Op: state.TCover, A: 1, Dst: 3, C: 4}},
			[]state.TermInstr{step, set, {Op: state.TCover, A: 1, Dst: 3, C: 4}}},
		{"cover of another value", []state.TermInstr{step, set, {Op: state.TCover, A: 1, B: 5, Dst: 3, C: 3}},
			[]state.TermInstr{step, set, {Op: state.TCover, A: 1, B: 5, Dst: 3, C: 3}}},
		{"field without a step", []state.TermInstr{set, {Op: state.TCover, A: 1, Dst: 3, C: 3}},
			[]state.TermInstr{set, {Op: state.TCover, A: 1, Dst: 3, C: 3}}},
		{"moved return", []state.TermInstr{{Op: state.TCap, Dst: 1}, {Op: state.TMoveA, Dst: 2, A: 1}, {Op: state.TRet, A: 2}},
			[]state.TermInstr{{Op: state.TCap, Dst: 1}, {Op: state.TRet, A: 1}}},
		{"return of another register", []state.TermInstr{{Op: state.TMoveA, Dst: 2, A: 1}, {Op: state.TRet, A: 3}},
			[]state.TermInstr{{Op: state.TMoveA, Dst: 2, A: 1}, {Op: state.TRet, A: 3}}},
		{"return a jump goes to", []state.TermInstr{{Op: state.TJumpF, Dst: 2}, {Op: state.TMoveA, Dst: 2, A: 1}, {Op: state.TRet, A: 2}},
			[]state.TermInstr{{Op: state.TJumpF, Dst: 2}, {Op: state.TMoveA, Dst: 2, A: 1}, {Op: state.TRet, A: 2}}},
	} {
		if got := emit(c.code...); !slices.Equal(got, c.want) {
			t.Errorf("%s: emitted %v, want %v", c.name, got, c.want)
		}
	}
}
