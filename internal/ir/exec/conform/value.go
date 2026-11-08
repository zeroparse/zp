package conform

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

// ValueImpl is an implementation of the value primitives (ops.AddValue) under test.
type ValueImpl interface {
	exec.Machine
	// Datum returns the implementation's value for the scalar v of the model.
	Datum(v ref.V) any
	// Canon returns the scalar of the model that the implementation's value stands for.
	Canon(x any) ref.V
}

// ValueSuite is an implementation of the value primitives to check.
type ValueSuite struct {
	// Name names the implementation in failures.
	Name string
	Impl ValueImpl
	// Ops are the value operations the implementation provides; the others are skipped.
	Ops []string
}

// ValueOps are the operations of the value primitives.
var ValueOps = []string{"binary", "unary", "logic"}

// ValuePool returns the scalars that the value conformance applies the operations to: nil, ints at the
// limits of 64 bits and of the integers that a double holds exactly, and the neighbors that make the
// results wrap, strings that differ in their first, last and only code point and in their bytes and
// in their UTF-16 code units, and both bools. Every combination of two is applied to the binary
// operators.
func ValuePool() []ref.V {
	pool := []ref.V{ref.Nil, ref.Bool(false), ref.Bool(true)}
	for _, i := range []int64{0, 1, -1, 2, -2, 3, 7, -7, 3037000499, 3037000500, 3037000501, 1 << 31, 1 << 53, 1<<53 + 1, -(1<<53 + 1),
		1 << 62, math.MaxInt64, math.MaxInt64 - 1, math.MinInt64, math.MinInt64 + 1} {
		pool = append(pool, ref.Int(i))
	}
	for _, s := range []string{"", "a", "b", "ab", "ba", "é", "é", "z", "ÿ", "Ā", "\U0001F600", "￿", "\U0001F600x", "a\U0001F600"} {
		pool = append(pool, ref.Str(s))
	}
	return pool
}

// ValueSequences returns the lists and tuples that the value conformance indexes, besides the
// scalars of the pool, which are not lists: an empty list, a list of scalars of every kind, and a
// tuple. Their elements are scalars, so that the results and the failures of an index hold no type
// name of a structured value.
func ValueSequences() []ref.V {
	return []ref.V{ref.List(), ref.List(ref.Int(10), ref.Str("a"), ref.Bool(true), ref.Nil, ref.Int(math.MinInt64)), ref.Tuple(ref.Int(5), ref.Int(6))}
}

// scalarEqual reports whether a and b are the same scalar.
func scalarEqual(a, b ref.V) bool {
	return a.Kind == b.Kind && a.I == b.I && a.S == b.S && a.B == b.B
}

// ValueCall is one call of a value operation: the code of the operator and the arguments after it.
type ValueCall struct {
	Op   string // the operation, such as binary
	Code int64  // the index into ops.BinaryOps, ops.UnaryOps or ops.LogicalOps
	Args []ref.V
}

// Operator returns the operator of the call: "+" for binary with the code of +.
func (c ValueCall) Operator() string {
	switch c.Op {
	case "binary":
		return ops.BinaryOps[c.Code]
	case "unary":
		return ops.UnaryOps[c.Code]
	}
	return ops.LogicalOps[c.Code]
}

// Key identifies the call, so that an implementation that was run on all calls beforehand can be
// asked for the result of one.
func (c ValueCall) Key() string {
	s := fmt.Sprintf("%s %d", c.Op, c.Code)
	for _, a := range c.Args {
		s += fmt.Sprintf(" %d:%q:%d", a.Kind, a.S, a.I)
		if a.Kind == ref.KList || a.Kind == ref.KTuple {
			s += a.String()
		}
		if a.B {
			s += "t"
		}
	}
	return s
}

// ValueCalls returns the calls that the value conformance makes of the operations named: every
// binary operator on every pair of the pool, the index of every sequence and every scalar by every
// scalar, and every unary and logical operator on every value.
func ValueCalls(provided []string) []ValueCall {
	pool := ValuePool()
	var calls []ValueCall
	if slices.Contains(provided, "binary") {
		for op := range ops.BinaryOps[:ops.OpIndex] {
			for _, l := range pool {
				for _, r := range pool {
					calls = append(calls, ValueCall{"binary", int64(op), []ref.V{l, r}})
				}
			}
		}
		for _, l := range append(ValueSequences(), pool...) {
			for _, r := range pool {
				calls = append(calls, ValueCall{"binary", int64(ops.OpIndex), []ref.V{l, r}})
			}
		}
		// A sequence has no equality, with a value of any kind but nil.
		for _, op := range []int{ops.OpEq, ops.OpNe} {
			for _, l := range ValueSequences() {
				for _, r := range append(ValueSequences(), pool...) {
					calls = append(calls, ValueCall{"binary", int64(op), []ref.V{l, r}}, ValueCall{"binary", int64(op), []ref.V{r, l}})
				}
			}
		}
	}
	for _, name := range []string{"unary", "logic"} {
		if !slices.Contains(provided, name) {
			continue
		}
		codes := ops.UnaryOps
		if name == "logic" {
			codes = ops.LogicalOps
		}
		for op := range codes {
			for _, x := range pool {
				calls = append(calls, ValueCall{name, int64(op), []ref.V{x}})
			}
		}
	}
	return calls
}

// RunValues makes every call of ValueCalls on the model and on the implementation, and returns the
// first difference: in a result value, in a failure, or in the text of its error.
func RunValues(s ValueSuite) error {
	model := ref.Values{}
	for _, c := range ValueCalls(s.Ops) {
		o := &exec.Op{Name: c.Op}
		mArgs, iArgs := []any{c.Code}, []any{c.Code}
		shown := c.Operator()
		for _, a := range c.Args {
			mArgs, iArgs = append(mArgs, a), append(iArgs, s.Impl.Datum(a))
			shown += ", " + a.String()
		}
		want, got := model.Call(o, mArgs), s.Impl.Call(o, iArgs)
		if len(got) != len(want) {
			return fmt.Errorf("%s: %s(%s) returns %d results, not %d", s.Name, c.Op, shown, len(got), len(want))
		}
		for i := range want {
			var w, g ref.V
			if b, ok := want[i].(bool); ok {
				gb, ok := got[i].(bool)
				if !ok {
					return fmt.Errorf("%s: %s(%s) result %d is %T, not a bool", s.Name, c.Op, shown, i, got[i])
				}
				w, g = ref.Bool(b), ref.Bool(gb)
			} else {
				w, g = want[i].(ref.V), s.Impl.Canon(got[i])
			}
			if structured(c) && w.Kind == ref.KStr && g.Kind == ref.KStr && w.S != g.S && afterFor(w.S) == afterFor(g.S) {
				// The name of the type of a structured value is each target's own (a list is []_ in one
				// and node[[]_] in another): the text agrees from the operator on.
				continue
			}
			if !scalarEqual(w, g) {
				return fmt.Errorf("%s: %s(%s) result %d: model %s, implementation %s", s.Name, c.Op, shown, i, w, g)
			}
		}
	}
	return nil
}

// structured reports whether a call has a sequence as an argument.
func structured(c ValueCall) bool {
	for _, a := range c.Args {
		if a.Kind == ref.KList || a.Kind == ref.KTuple {
			return true
		}
	}
	return false
}

// afterFor returns the text of an error message from " for ", the operator and what follows it, or
// "" if it has none.
func afterFor(msg string) string {
	if i := strings.Index(msg, " for "); i >= 0 {
		return msg[i:]
	}
	return ""
}
