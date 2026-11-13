package state

import (
	"slices"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
)

// TestTermDeclareKeepsTheFile checks that a local keeps the register file of its first
// declaration: declaring it again in another file is an error instead of a second register that
// its readers would not see.
func TestTermDeclareKeepsTheFile(t *testing.T) {
	c := &termCompiler{locals: map[string]termReg{}, prog: &TermProgram{}}
	n, err := c.declare("x", exec.Type{Kind: exec.KInt})
	if err != nil {
		t.Fatal(err)
	}
	if again, err := c.declare("x", exec.Type{Kind: exec.KBool}); err != nil || again != n {
		t.Errorf("declaring x again in its file gives %v, %v; want %v", again, err, n)
	}
	if r, err := c.declare("x", exec.Type{Kind: exec.KHandle, Handle: ops.Datum}); err == nil {
		t.Errorf("declaring the int x as a value gives the register %v, want an error", r)
	}
	if r, err := c.declare("y", exec.Type{Kind: exec.KHandle, Handle: "Nope"}); err == nil {
		t.Errorf("a local of a type without a register file gets %v, want an error", r)
	}
}

// TestTermEmitFuses checks the instructions the term compiler fuses as it emits them: a step, the
// field of a struct and the cover of its value become FIELD, and a move just before a return
// becomes the return of its source unless a jump goes to the return, whose other paths need the
// move.
func TestTermEmitFuses(t *testing.T) {
	emit := func(code ...termInstr) []termInstr {
		c := &termCompiler{prog: &TermProgram{}}
		for _, in := range code {
			c.emit(in)
		}
		return c.prog.code
	}
	step := termInstr{op: tStep}
	set := termInstr{op: tSetField, a: 1, c: 2}
	for _, c := range []struct {
		name string
		code []termInstr
		want []termInstr
	}{
		{"field", []termInstr{step, set, {op: tCover, a: 1, dst: 3, c: 3}},
			[]termInstr{{op: tField, dst: 3, a: 1, c: 2}}},
		{"cover into another register", []termInstr{step, set, {op: tCover, a: 1, dst: 3, c: 4}},
			[]termInstr{step, set, {op: tCover, a: 1, dst: 3, c: 4}}},
		{"cover of another value", []termInstr{step, set, {op: tCover, a: 1, b: 5, dst: 3, c: 3}},
			[]termInstr{step, set, {op: tCover, a: 1, b: 5, dst: 3, c: 3}}},
		{"field without a step", []termInstr{set, {op: tCover, a: 1, dst: 3, c: 3}},
			[]termInstr{set, {op: tCover, a: 1, dst: 3, c: 3}}},
		{"moved return", []termInstr{{op: tCap, dst: 1}, {op: tMoveA, dst: 2, a: 1}, {op: tRet, a: 2}},
			[]termInstr{{op: tCap, dst: 1}, {op: tRet, a: 1}}},
		{"return of another register", []termInstr{{op: tMoveA, dst: 2, a: 1}, {op: tRet, a: 3}},
			[]termInstr{{op: tMoveA, dst: 2, a: 1}, {op: tRet, a: 3}}},
		{"return a jump goes to", []termInstr{{op: tJumpF, dst: 2}, {op: tMoveA, dst: 2, a: 1}, {op: tRet, a: 2}},
			[]termInstr{{op: tJumpF, dst: 2}, {op: tMoveA, dst: 2, a: 1}, {op: tRet, a: 2}}},
	} {
		if got := emit(c.code...); !slices.Equal(got, c.want) {
			t.Errorf("%s: emitted %v, want %v", c.name, got, c.want)
		}
	}
}
