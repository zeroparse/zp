package core

import (
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
