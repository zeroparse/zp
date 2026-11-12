package core

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/value"
)

// TestLoadTermRejectsEachCheck checks the operand and pool checks of LoadTerm one by one: each case
// is a valid term, which loads (the negative control), and the same term with one value moved just
// out of range, which LoadTerm rejects with the error of that check.
func TestLoadTermRejectsEachCheck(t *testing.T) {
	strs := []string{"S"}
	types := TermTypes{
		NodeType: func(string) *value.NodeTypeKinds { return nil },
		Struct: func(name string) *value.NodeKind {
			if name == "S" {
				return value.SeqKind
			}
			return nil
		},
	}
	const vars = 1
	ret := bytecode.TermInstr{Op: uint8(tRet), A: 0}
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
			in := bytecode.TermInstr{Op: uint8(tMoveS)}
			if bad {
				in.Dst = 1
			}
			*x = term(in)
		}, "invalid operand"},
		{"variable", func(x *bytecode.Term, bad bool) {
			in := bytecode.TermInstr{Op: uint8(tVar), A: vars - 1}
			if bad {
				in.A = vars
			}
			*x = term(in)
		}, "invalid operand"},
		{"unary operator", func(x *bytecode.Term, bad bool) {
			in := bytecode.TermInstr{Op: uint8(tUnary), B: int32(len(ops.UnaryOps) - 1)}
			if bad {
				in.B++
			}
			*x = term(in)
		}, "invalid operand"},
		{"binary operator", func(x *bytecode.Term, bad bool) {
			in := bytecode.TermInstr{Op: uint8(tBinary), C: int32(len(ops.BinaryOps) - 1)}
			if bad {
				in.C++
			}
			*x = term(in)
		}, "invalid operand"},
		{"logical operator", func(x *bytecode.Term, bad bool) {
			in := bytecode.TermInstr{Op: uint8(tLogic), B: int32(len(ops.LogicalOps) - 1)}
			if bad {
				in.B++
			}
			*x = term(in)
		}, "invalid operand"},
		{"flag", func(x *bytecode.Term, bad bool) {
			in := bytecode.TermInstr{Op: uint8(tHull), B: 1}
			if bad {
				in.B = 2
			}
			*x = term(in)
		}, "invalid operand"},
		{"boolean constant", func(x *bytecode.Term, bad bool) {
			*x = term(bytecode.TermInstr{Op: uint8(tConst)})
			x.Consts = []bytecode.TermConst{{Kind: bytecode.ConstBool, Int: 1}}
			if bad {
				x.Consts[0].Int = 2
			}
		}, "invalid boolean constant"},
		{"slot", func(x *bytecode.Term, bad bool) {
			in := bytecode.TermInstr{Op: uint8(tCap)}
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
