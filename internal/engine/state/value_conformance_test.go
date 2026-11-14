package state

import (
	"context"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/core/impl"
	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/ir/exec/ref"
	"zeroparse.dev/zp/internal/value"
)

// engineValues binds the compositions of the operators to the engine's printed term_exec.go
// (termBinary, termUnary and termLogic).
type engineValues struct {
	// p is the parser that runs the call; a limited parse has a budget control.
	p *Parser
}

func (e engineValues) parser() *Parser {
	if e.p == nil {
		return &Parser{}
	}
	return e.p
}

func (engineValues) Datum(v ref.V) any {
	if v.Kind == ref.KList || v.Kind == ref.KTuple {
		p := &Parser{}
		kids := make([]*core.Node, len(v.Items))
		for i, x := range v.Items {
			if x.Kind != ref.KNil {
				kids[i] = p.NewNode(value.Data{Kind: coreimpl.BoxKind, Box: engineValues{}.Datum(x)})
			}
		}
		return p.NewNode(value.Data{Kind: coreimpl.ListValueKind, Children: kids})
	}
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

func (e engineValues) Call(o *exec.Op, args []any) []any {
	switch o.Name {
	case "binary":
		v, err := e.parser().ValueBinary(ops.BinaryOps[args[0].(int64)], args[1], args[2])
		return []any{v, errDatum(err)}
	case "unary":
		v, err := e.parser().ValueUnary(ops.UnaryOps[args[0].(int64)], args[1])
		return []any{v, errDatum(err)}
	case "logic":
		b, err := e.parser().ValueLogic(ops.LogicalOps[args[0].(int64)], args[1])
		return []any{b, errDatum(err)}
	}
	panic("engine does not provide " + o.Name)
}

func TestEngineValueConformance(t *testing.T) {
	controlled := &Parser{Input: Input{Control: NewBudgetControl(context.Background(), core.Limits{Compute: 1 << 62})}}
	for name, impl := range map[string]engineValues{"engine": {}, "engine controlled": {controlled}} {
		s := conform.ValueSuite{Name: name, Impl: impl, Ops: []string{"binary", "unary", "logic"}}
		if err := conform.RunValues(s); err != nil {
			t.Error(err)
		}
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
		if o.Name != "binary" || ops.BinaryOps[args[0].(int64)] != op {
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
			if o.Name == "binary" && ops.BinaryOps[args[0].(int64)] == "<" {
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
		s := conform.ValueSuite{Name: name, Impl: brokenEngineValues{mutate: mutate}, Ops: []string{"binary", "unary", "logic"}}
		if err := conform.RunValues(s); err == nil {
			t.Errorf("%s: the conformance run did not notice", name)
		}
	}
}

// TestEngineValueBoxedOperands holds == to the rule that a box counts as the value it holds, which the
// pool of the conformance cannot show: it has no box. A box of an int equals the int, a box of nil
// equals nil, and a box of a string is not equal to a string with other bytes; a box is no operand of
// an arithmetic operator.
func TestEngineValueBoxedOperands(t *testing.T) {
	p := &Parser{}
	box := func(v any) *core.Node { return p.NewNode(value.Data{Kind: coreimpl.BoxKind, Box: v}) }
	for _, c := range []struct {
		op   string
		l, r any
		want any
	}{
		{"==", box(1), 1, true},
		{"==", 1, box(1), true},
		{"==", box("a"), "a", true},
		{"==", box("a"), "b", false},
		{"!=", box(true), true, false},
		{"==", box(nil), nil, true},
		{"==", box(1), nil, false},
		{"==", box(1), box(2), false},
	} {
		got, err := p.ValueBinary(c.op, c.l, c.r)
		if err != nil || got != c.want {
			t.Errorf("%v %s %v = %v, %v; want %v", c.l, c.op, c.r, got, err, c.want)
		}
	}
	if _, err := p.ValueBinary("+", box(1), 1); err == nil {
		t.Error("a box is an operand of +")
	}
}
