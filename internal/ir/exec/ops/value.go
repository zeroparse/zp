package ops

import "zeroparse.dev/zp/internal/ir/exec"

// Val is the component of the value primitives: the operators that act on the scalar values of terms.
// A primitive is a pure function of its arguments and has no cost: the compositions that call it charge.
const Val = "value"

// Binary operators of val.binary, by the code a call passes.
var BinaryOps = []string{"+", "-", "*", "/", "%", "==", "!=", "<", "<=", ">", ">="}

// UnaryOps are the unary operators of val.unary, by the code a call passes.
var UnaryOps = []string{"-", "!"}

// LogicalOps are the operators whose operands val.bool checks, by the code a call passes.
var LogicalOps = []string{"&&", "||"}

// ValueModule returns the pilot declarations with the value primitives. The primitives are not part
// of Pilot until a composition calls them.
func ValueModule() *exec.Module {
	m := Pilot()
	AddValue(m)
	return m
}

// AddValue adds the value component and its primitives to m, which declares the handle Datum.
//
// A value is nil, an int, a string, a bool, or a structured value (a node, list, tuple or struct) that
// the primitives accept only where they say so. An int is 64 bits wide with wraparound. A primitive
// that can fail has a second result: nil on success, otherwise the text of the error as a string value.
// The text is the one the action reports, so that every target fails with the same message.
func AddValue(m *exec.Module) {
	m.AddComponent(Val, `The operators on the values of terms. They read and write no state.`)
	datum := exec.THandle(Datum)
	pure := func(o *exec.Op) {
		o.Component = Val
		m.AddOp(o)
	}
	pure(&exec.Op{Name: "val.binary",
		Params:  []exec.Param{exec.P("op", exec.TInt), exec.P("l", datum), exec.P("r", datum)},
		Results: []exec.Type{datum, datum},
		Contract: `Applies the binary operator BinaryOps[op] to l and r and returns the value and a nil error.
Ints combine with + - * / % and compare with < <= > >=; the result of an arithmetic operator is the
64-bit result with wraparound, of / and % truncated toward zero with the sign of the dividend on %.
A division by zero fails with "division by zero". Strings concatenate with + and compare with < <= > >=
by code point. == and != compare two values of one scalar type, a bool, an int or a string, and any
value with nil, which equals only nil; a box compares as the value it holds. Any other operands fail
with "invalid operands T and U for op", T and U naming the types of the operands, and a == or != of
structured values continues with ": nodes and other structured values cannot be compared; use text(x),
or .Start and .End". On failure the value is nil.`,
		Requires: `op is an index of BinaryOps.`,
		Laws:     []string{"pure: equal arguments give equal results", "== is symmetric and != is its negation"}})
	pure(&exec.Op{Name: "val.unary",
		Params:  []exec.Param{exec.P("op", exec.TInt), exec.P("x", datum)},
		Results: []exec.Type{datum, datum},
		Contract: `Applies the unary operator UnaryOps[op] to x: - negates an int with wraparound (the negation of
the least int is itself) and ! negates a bool. Any other operand fails with "invalid operand T for unary op".
On failure the value is nil.`,
		Requires: `op is an index of UnaryOps.`,
		Laws:     []string{"pure: equal arguments give equal results"}})
	pure(&exec.Op{Name: "val.bool",
		Params:  []exec.Param{exec.P("op", exec.TInt), exec.P("x", datum)},
		Results: []exec.Type{exec.TBool, datum},
		Contract: `Checks that x, an operand of the logical operator LogicalOps[op], is a bool, and returns it
with a nil error. Any other value fails with "invalid operand T for op", and the bool is false.`,
		Requires: `op is an index of LogicalOps.`,
		Laws:     []string{"pure: equal arguments give equal results"}})
}
