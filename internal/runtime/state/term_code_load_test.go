package state

import (
	"strings"
	"testing"

	coreimpl "zeroparse.dev/zp/core/impl"
	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/ir/code/action"
	"zeroparse.dev/zp/internal/runtime/nodes"
)

// TestLoadTermRejectsEachCheck checks the operand and pool checks of LoadTerm one by one: each case
// is a valid term, which loads (the negative control), and the same term with one value moved just
// out of range, which LoadTerm rejects with the error of that check.
func TestLoadTermRejectsEachCheck(t *testing.T) {
	strs := []string{"S"}
	types := TermTypes{
		NodeType: func(string) *nodes.NodeTypeKinds { return nil },
		Struct: func(name string) *coreimpl.Kind {
			if name == "S" {
				return coreimpl.SeqKind
			}
			return nil
		},
	}
	const vars = 1
	ret := bytecode.TermInstr{Op: uint8(TRet), A: 0}
	// term returns a term of one value register, one register of every other file, and the code
	// in followed by a return.
	term := func(in ...bytecode.TermInstr) bytecode.Term {
		return bytecode.Term{Regs: [4]int{1, 1, 1, 1}, Code: append(in, ret)}
	}
	for _, tc := range []struct {
		name string
		edit func(x *bytecode.Term, bad bool)
		want string
	}{
		{"sequence register", func(x *bytecode.Term, bad bool) {
			in := bytecode.TermInstr{Op: uint8(TMoveS)}
			if bad {
				in.Dst = 1
			}
			*x = term(in)
		}, "invalid operand"},
		{"field cover register", func(x *bytecode.Term, bad bool) {
			// The integer register that FIELD reads and writes, the one operand no other case pins.
			in := bytecode.TermInstr{Op: uint8(TField)}
			if bad {
				in.Dst = 1
			}
			*x = term(in)
			x.Names = []int{0}
		}, "invalid operand"},
		{"variable", func(x *bytecode.Term, bad bool) {
			in := bytecode.TermInstr{Op: uint8(TVar), A: vars - 1}
			if bad {
				in.A = vars
			}
			*x = term(in)
		}, "invalid operand"},
		{"unary operator", func(x *bytecode.Term, bad bool) {
			in := bytecode.TermInstr{Op: uint8(TUnary), B: int32(len(action.UnaryOps) - 1)}
			if bad {
				in.B++
			}
			*x = term(in)
		}, "invalid operand"},
		{"binary operator", func(x *bytecode.Term, bad bool) {
			in := bytecode.TermInstr{Op: uint8(TBinary), C: int32(len(action.BinaryOps) - 1)}
			if bad {
				in.C++
			}
			*x = term(in)
		}, "invalid operand"},
		{"logical operator", func(x *bytecode.Term, bad bool) {
			in := bytecode.TermInstr{Op: uint8(TLogic), B: int32(len(action.LogicalOps) - 1)}
			if bad {
				in.B++
			}
			*x = term(in)
		}, "invalid operand"},
		{"flag", func(x *bytecode.Term, bad bool) {
			in := bytecode.TermInstr{Op: uint8(THull), B: 1}
			if bad {
				in.B = 2
			}
			*x = term(in)
		}, "invalid operand"},
		{"boolean constant", func(x *bytecode.Term, bad bool) {
			*x = term(bytecode.TermInstr{Op: uint8(TConst)})
			x.Consts = []bytecode.TermConst{{Kind: bytecode.ConstBool, Int: 1}}
			if bad {
				x.Consts[0].Int = 2
			}
		}, "invalid boolean constant"},
		{"slot", func(x *bytecode.Term, bad bool) {
			in := bytecode.TermInstr{Op: uint8(TCap)}
			if bad {
				in.A = -1
			}
			*x = term(in)
		}, "invalid slot"},
		{"register count", func(x *bytecode.Term, bad bool) {
			*x = term()
			x.Regs[0] = 1 << 20
			if bad {
				x.Regs[0]++
			}
		}, "invalid register count"},
		{"negative register count", func(x *bytecode.Term, bad bool) {
			*x = term()
			x.Regs[3] = 0
			if bad {
				x.Regs[3] = -1
			}
		}, "invalid register count"},
		{"struct field count", func(x *bytecode.Term, bad bool) {
			*x = term()
			x.Structs = []bytecode.TermStruct{{Type: 0, Fields: 0}}
			if bad {
				x.Structs[0].Fields = -1
			}
		}, "invalid field count"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var good, bad bytecode.Term
			tc.edit(&good, false)
			tc.edit(&bad, true)
			if _, err := LoadTerm(&good, strs, vars, types); err != nil {
				t.Fatalf("the valid term: %v", err)
			}
			if _, err := LoadTerm(&bad, strs, vars, types); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
		})
	}
}
