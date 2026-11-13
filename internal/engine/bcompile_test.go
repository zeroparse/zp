package engine

import (
	"errors"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/gentest"
)

// TestModuleCompilesCorpus checks that every test grammar compiles to bytecode and that all
// branch targets are within the instruction range.
func TestModuleCompilesCorpus(t *testing.T) {
	t.Parallel()
	for _, c := range gentest.GenCorpus(t) {
		m := mustCompile(t, c.Src).Module()
		for i, in := range m.Code {
			switch in.Op {
			case bytecode.OpJump, bytecode.OpChoice, bytecode.OpCommit, bytecode.OpIter, bytecode.OpNext, bytecode.OpLook, bytecode.OpRecover, bytecode.OpEndRecover:
				if in.A < 0 || int(in.A) > len(m.Code) {
					t.Errorf("%s: code %d: jump to %d", c.Name, i, in.A)
				}
			case bytecode.OpGuard:
				if in.C <= int32(i) || int(in.C) > len(m.Code) {
					t.Errorf("%s: code %d: guard jumps to %d", c.Name, i, in.C)
				}
			}
		}
		if !strings.Contains(m.Disassemble(), "RETURN") {
			t.Errorf("%s: no RETURN", c.Name)
		}
	}
}

// TestModuleErrorSharedByPrograms checks that an internal error of the bytecode compilation is
// reported by every program sharing the static program, not only by the one whose call compiled it:
// a later program must not get a nil module. The error is recorded directly, since no grammar makes
// the compilation fail.
func TestModuleErrorSharedByPrograms(t *testing.T) {
	t.Parallel()
	a := mustCompile(t, `rule main = "a"`)
	b := newProgram(a.Static(), nil, nil)
	s := a.Static()
	want := errors.New("internal error: injected")
	s.BytecodeOnce.Do(func() { s.BytecodeErr = want })
	for name, p := range map[string]*Program{"first": a, "sharing": b} {
		func() {
			defer func() {
				if got := recover(); got != want {
					t.Errorf("%s: Module() panicked with %v, want %v", name, got, want)
				}
			}()
			m := p.Module()
			t.Errorf("%s: Module() returned %v without panicking", name, m)
		}()
	}
}
