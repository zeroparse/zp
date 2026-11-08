package core

import (
	"testing"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

// engineValues binds the value primitives to the engine's evaluation of terms (binaryOperation and
// ValueUnary). The logical operators' operand check is inline code of the closure and bytecode
// backends, so val.bool is not provided here; TestTermReferenceEvaluator in internal/gen checks it
// through terms.
type engineValues struct{}

func (engineValues) Datum(v ref.V) any {
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

func (engineValues) Canon(x any) ref.V {
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
	panic("engine value is not a scalar")
}

func errDatum(err error) any {
	if err == nil {
		return nil
	}
	return err.Error()
}

func (engineValues) Call(o *exec.Op, args []any) []any {
	switch o.Name {
	case "val.binary":
		v, err := binaryOp(ops.BinaryOps[args[0].(int64)], args[1], args[2])
		return []any{v, errDatum(err)}
	case "val.unary":
		v, err := (*Parser)(nil).ValueUnary(ops.UnaryOps[args[0].(int64)], args[1])
		return []any{v, errDatum(err)}
	}
	panic("engine does not provide " + o.Name)
}

func TestEngineValueConformance(t *testing.T) {
	s := conform.ValueSuite{Name: "engine", Impl: engineValues{}, Ops: []string{"val.binary", "val.unary"}}
	if err := conform.RunValues(s); err != nil {
		t.Fatal(err)
	}
}

// brokenEngineValues runs mutate on the result of a call.
type brokenEngineValues struct {
	engineValues
	mutate func(o *exec.Op, args []any, res []any) []any
}

func (b brokenEngineValues) Call(o *exec.Op, args []any) []any {
	return b.mutate(o, args, b.engineValues.Call(o, args))
}

// TestEngineValueConformanceDetectsBrokenEvaluation is the negative control: results that are wrong
// in one way each fail the conformance run.
func TestEngineValueConformanceDetectsBrokenEvaluation(t *testing.T) {
	binary := func(o *exec.Op, args []any, op string) (l, r int, ok bool) {
		if o.Name != "val.binary" || ops.BinaryOps[args[0].(int64)] != op {
			return 0, 0, false
		}
		l, lok := args[1].(int)
		r, rok := args[2].(int)
		return l, r, lok && rok
	}
	mutants := map[string]func(o *exec.Op, args []any, res []any) []any{
		"a multiplication saturates": func(o *exec.Op, args []any, res []any) []any {
			if l, r, ok := binary(o, args, "*"); ok && l > 1<<31 && r > 1<<31 {
				return []any{1<<63 - 1, nil}
			}
			return res
		},
		"a remainder of the least int by -1 fails": func(o *exec.Op, args []any, res []any) []any {
			if l, r, ok := binary(o, args, "%"); ok && l == -1<<63 && r == -1 {
				return []any{nil, "overflow"}
			}
			return res
		},
		"a string compares less than a longer string with it as prefix, not": func(o *exec.Op, args []any, res []any) []any {
			if o.Name == "val.binary" && ops.BinaryOps[args[0].(int64)] == "<" {
				if l, ok := args[1].(string); ok {
					if r, ok := args[2].(string); ok && l != "" && len(r) > len(l) && r[:len(l)] == l {
						return []any{false, nil}
					}
				}
			}
			return res
		},
	}
	for name, mutate := range mutants {
		s := conform.ValueSuite{Name: name, Impl: brokenEngineValues{mutate: mutate}, Ops: []string{"val.binary", "val.unary"}}
		if err := conform.RunValues(s); err == nil {
			t.Errorf("%s: the conformance run did not notice", name)
		}
	}
}
