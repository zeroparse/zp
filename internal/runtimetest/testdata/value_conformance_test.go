package runtime

import (
	"math"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

// The value primitives of the generated Go runtimes are bound to the functions that evaluate the
// operators of a term: rtBinary, rtUnary and rtBool for node parsers, and trtBinary, trtUnary and
// trtBool for typed parsers. Each fails by panicking with an evalError, which the binding turns into
// the error text the primitive returns.

type goValues struct {
	binary func(op string, l, r any) any
	unary  func(op string, x any) any
	bool_  func(op string, x any) bool
}

func (goValues) Datum(v ref.V) any {
	switch v.Kind {
	case ref.KInt:
		return int(v.I)
	case ref.KStr:
		return v.S
	case ref.KBool:
		return v.B
	}
	return nil
}

func (goValues) Canon(x any) ref.V {
	switch x := x.(type) {
	case int:
		return ref.Int(int64(x))
	case string:
		return ref.Str(x)
	case bool:
		return ref.Bool(x)
	case nil:
		return ref.Nil
	}
	panic("runtime value is not a scalar")
}

// failed runs f and returns the text of the evalError it panics with, "" if it returns.
func failed(f func()) (msg any) {
	defer func() {
		if x := recover(); x != nil {
			e, ok := x.(evalError)
			if !ok {
				panic(x)
			}
			msg = e.msg
		}
	}()
	f()
	return nil
}

func (g goValues) Call(o *exec.Op, args []any) (res []any) {
	switch o.Name {
	case "val.binary":
		var v any
		err := failed(func() { v = g.binary(ops.BinaryOps[args[0].(int64)], args[1], args[2]) })
		return []any{v, err}
	case "val.unary":
		var v any
		err := failed(func() { v = g.unary(ops.UnaryOps[args[0].(int64)], args[1]) })
		return []any{v, err}
	case "val.bool":
		var b bool
		err := failed(func() { b = g.bool_(ops.LogicalOps[args[0].(int64)], args[1]) })
		return []any{b, err}
	}
	panic("runtime does not provide " + o.Name)
}

var (
	nodeValues  = goValues{rtBinary, rtUnary, rtBool}
	typedValues = goValues{trtBinary, trtUnary, trtBool}
)

func TestValueConformance(t *testing.T) {
	for name, impl := range map[string]goValues{"node": nodeValues, "typed": typedValues} {
		s := conform.ValueSuite{Name: "go " + name, Impl: impl, Ops: conform.ValueOps}
		if err := conform.RunValues(s); err != nil {
			t.Error(err)
		}
	}
}

// brokenValues runs mutate on the result of a call.
type brokenValues struct {
	goValues
	mutate func(o *exec.Op, args []any, res []any) []any
}

func (b brokenValues) Call(o *exec.Op, args []any) []any {
	return b.mutate(o, args, b.goValues.Call(o, args))
}

// TestValueConformanceDetectsBrokenRuntime is the negative control: runtime functions that are wrong
// in one way fail the conformance run.
func TestValueConformanceDetectsBrokenRuntime(t *testing.T) {
	binary := func(o *exec.Op, args []any) (string, int, int, bool) {
		if o.Name != "val.binary" {
			return "", 0, 0, false
		}
		l, lok := args[1].(int)
		r, rok := args[2].(int)
		return ops.BinaryOps[args[0].(int64)], l, r, lok && rok
	}
	mutants := map[string]func(o *exec.Op, args []any, res []any) []any{
		"an addition saturates": func(o *exec.Op, args []any, res []any) []any {
			if op, l, r, ok := binary(o, args); ok && op == "+" && l > 0 && r > 0 && res[0].(int) < 0 {
				return []any{math.MaxInt, nil}
			}
			return res
		},
		"a division floors": func(o *exec.Op, args []any, res []any) []any {
			if op, l, r, ok := binary(o, args); ok && op == "/" && r != 0 && l%r != 0 && (l < 0) != (r < 0) {
				return []any{res[0].(int) - 1, nil}
			}
			return res
		},
		"an int equals a string": func(o *exec.Op, args []any, res []any) []any {
			if o.Name == "val.binary" && ops.BinaryOps[args[0].(int64)] == "==" {
				if _, ok := args[1].(int); ok {
					if _, ok := args[2].(string); ok {
						return []any{false, nil}
					}
				}
			}
			return res
		},
		"a string comparison compares bytes of the UTF-16 form": func(o *exec.Op, args []any, res []any) []any {
			if o.Name == "val.binary" && ops.BinaryOps[args[0].(int64)] == "<" {
				if l, ok := args[1].(string); ok {
					if r, ok := args[2].(string); ok && len(l) > 0 && len(r) > 0 && (l[0] >= 0xF0) != (r[0] >= 0xF0) {
						return []any{!res[0].(bool), nil}
					}
				}
			}
			return res
		},
		"the error of a unary minus names no operand": func(o *exec.Op, args []any, res []any) []any {
			if o.Name == "val.unary" && res[1] != nil {
				return []any{res[0], "invalid operand for unary -"}
			}
			return res
		},
	}
	for _, impl := range []struct {
		name string
		v    goValues
	}{{"node", nodeValues}, {"typed", typedValues}} {
		for name, mutate := range mutants {
			s := conform.ValueSuite{Name: "broken " + impl.name + ": " + name, Impl: brokenValues{impl.v, mutate}, Ops: conform.ValueOps}
			if err := conform.RunValues(s); err == nil {
				t.Errorf("%s %s: the conformance run did not notice", impl.name, name)
			}
		}
	}
}
