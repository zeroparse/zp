package runtime

import (
	"context"
	"math"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/ir/exec/ref"
	"zeroparse.dev/zp/runtime/noderep"
	"zeroparse.dev/zp/runtime/parse"
)

// The compositions of the operators of terms in the generated Go runtimes (TermBinary, TermUnary and
// TermLogic, printed from Exec IR) are the subjects of the value conformance, for node parsers and for
// typed parsers. Each returns its failure as a value, whose text the binding returns.

type goValues struct {
	binary func(op int, l, r any) (any, any)
	unary  func(op int, x any) (any, any)
	logic  func(op int, x any) (bool, any)
	// typed makes the sequences of the typed runtime.
	typed bool
}

func scalar(v ref.V) any {
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

func (g goValues) Datum(v ref.V) any {
	if v.Kind != ref.KList && v.Kind != ref.KTuple {
		return scalar(v)
	}
	if g.typed {
		kids := make([]any, len(v.Items))
		for i, x := range v.Items {
			kids[i] = scalar(x)
		}
		return &TSeq{Typ: "list", Kids: kids}
	}
	kids := make([]*parse.Node, len(v.Items))
	for i, x := range v.Items {
		if x.Kind != ref.KNil {
			kids[i] = nodeOfData(&Data{Kind: noderep.BoxKind, Box: scalar(x)})
		}
	}
	return nodeOfData(&Data{Kind: noderep.ListValueKind, Children: kids})
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

// text returns the text of the failure e, nil if there is none.
func text(e any) any {
	if e == nil {
		return nil
	}
	return e.(evalError).msg
}

func (g goValues) Call(o *exec.Op, args []any) (res []any) {
	code := int(args[0].(int64))
	switch o.Name {
	case "binary":
		v, e := g.binary(code, args[1], args[2])
		return []any{v, text(e)}
	case "unary":
		v, e := g.unary(code, args[1])
		return []any{v, text(e)}
	case "logic":
		b, e := g.logic(code, args[1])
		return []any{b, text(e)}
	}
	panic("runtime does not provide " + o.Name)
}

var (
	nodeValues  = goValues{(&Parser{}).TermBinary, (&Parser{}).TermUnary, (&Parser{}).TermLogic, false}
	typedParser = &TParser{Parser: &Parser{}}
	typedValues = goValues{typedParser.TermBinary, typedParser.TermUnary, typedParser.TermLogic, true}
)

// controlledValues binds the compositions of a parser that runs under budget controls, whose string
// operations take the bounded paths.
func controlledValues() (node, typed goValues) {
	control := newBudgetControl(context.Background(), parse.Limits{Compute: 1 << 62})
	p := &Parser{Control: control}
	tp := &TParser{Parser: &Parser{Control: control}}
	return goValues{p.TermBinary, p.TermUnary, p.TermLogic, false}, goValues{tp.TermBinary, tp.TermUnary, tp.TermLogic, true}
}

func TestValueConformance(t *testing.T) {
	cn, ct := controlledValues()
	for name, impl := range map[string]goValues{"node": nodeValues, "typed": typedValues, "node controlled": cn, "typed controlled": ct} {
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
		if o.Name != "binary" {
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
			if o.Name == "binary" && ops.BinaryOps[args[0].(int64)] == "==" {
				if _, ok := args[1].(int); ok {
					if _, ok := args[2].(string); ok {
						return []any{false, nil}
					}
				}
			}
			return res
		},
		"a string comparison compares bytes of the UTF-16 form": func(o *exec.Op, args []any, res []any) []any {
			if o.Name == "binary" && ops.BinaryOps[args[0].(int64)] == "<" {
				if l, ok := args[1].(string); ok {
					if r, ok := args[2].(string); ok && len(l) > 0 && len(r) > 0 && (l[0] >= 0xF0) != (r[0] >= 0xF0) {
						return []any{!res[0].(bool), nil}
					}
				}
			}
			return res
		},
		"the error of a unary minus names no operand": func(o *exec.Op, args []any, res []any) []any {
			if o.Name == "unary" && res[1] != nil {
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

// TestValueBoxedOperands holds == of the node runtime to the rule that a box counts as the value it
// holds, which the pool of the conformance cannot show: it has no box.
func TestValueBoxedOperands(t *testing.T) {
	box := func(v any) *parse.Node { return nodeOfData(&Data{Kind: noderep.BoxKind, Box: v}) }
	p := &Parser{}
	for _, c := range []struct {
		op   int
		l, r any
		want any
	}{
		{OpEq, box(1), 1, true},
		{OpEq, 1, box(1), true},
		{OpEq, box("a"), "b", false},
		{OpNe, box(true), true, false},
		{OpEq, box(nil), nil, true},
		{OpEq, box(1), nil, false},
	} {
		got, e := p.TermBinary(c.op, c.l, c.r)
		if e != nil || got != c.want {
			t.Errorf("%v op %d %v = %v, %v; want %v", c.l, c.op, c.r, got, e, c.want)
		}
	}
	if _, e := p.TermBinary(OpAdd, box(1), 1); e == nil {
		t.Error("a box is an operand of +")
	}
}

// TestValueBoxedStringCharges holds == and != of the node runtime on a boxed string to the same
// comparison of the strings it holds, on every pair of strings of the pool and with the box on either
// side or both: the same answer and, under a budget control, the same units. The box only changes how
// the operand is carried, so it must neither skip the charge of the comparison nor add one.
func TestValueBoxedStringCharges(t *testing.T) {
	box := func(v any) *parse.Node { return nodeOfData(&Data{Kind: noderep.BoxKind, Box: v}) }
	var strs []string
	for _, v := range conform.ValuePool() {
		if v.Kind == ref.KStr {
			strs = append(strs, v.S)
		}
	}
	// Each side has its own parser, so that the units one charges do not hide in the other's.
	newSide := func(controlled bool) *Parser {
		p := &Parser{}
		if controlled {
			p.Control = newBudgetControl(context.Background(), parse.Limits{Compute: 1 << 62})
		}
		return p
	}
	used := func(p *Parser) uint64 {
		if p.Control == nil {
			return 0
		}
		return uint64(p.Control.budgetUsed())
	}
	sides := []struct {
		name       string
		boxL, boxR bool
	}{{"left", true, false}, {"right", false, true}, {"both", true, true}}
	charged := false
	for _, controlled := range []bool{false, true} {
		boxed, plain := newSide(controlled), newSide(controlled)
		for _, op := range []int{OpEq, OpNe} {
			for _, l := range strs {
				for _, r := range strs {
					for _, s := range sides {
						var bl, br any = l, r
						if s.boxL {
							bl = box(l)
						}
						if s.boxR {
							br = box(r)
						}
						beforeBoxed, beforePlain := used(boxed), used(plain)
						want, e := plain.TermBinary(op, l, r)
						if e != nil {
							t.Fatalf("controlled=%v: %q op %d %q fails: %v", controlled, l, op, r, e)
						}
						got, e := boxed.TermBinary(op, bl, br)
						if e != nil || got != want {
							t.Errorf("controlled=%v, box %s: %q op %d %q = %v, %v; want %v", controlled, s.name, l, op, r, got, e, want)
						}
						ub, up := used(boxed)-beforeBoxed, used(plain)-beforePlain
						if ub != up {
							t.Errorf("controlled=%v, box %s: %q op %d %q charges %d, the unboxed comparison charges %d", controlled, s.name, l, op, r, ub, up)
						}
						charged = charged || up > 0
					}
				}
			}
		}
	}
	// Equal units are vacuous if no comparison charges any.
	if !charged {
		t.Error("no string comparison of the pool charged a unit under a budget control")
	}
}

// TestValueCompareStrings holds the comparison of two unboxed strings of the typed runtime
// (TActCtx.TermCompare, whose fast path decides in compareStrings) to the composition TermBinary on
// every pair of strings of the pool, for each comparison operator: the same answer and, under a
// budget control, the same units, which a fast path that skips the control would not charge.
// compareStrings is written by hand, so the composition is its reference.
func TestValueCompareStrings(t *testing.T) {
	var strs []string
	for _, v := range conform.ValuePool() {
		if v.Kind == ref.KStr {
			strs = append(strs, v.S)
		}
	}
	// Each side has its own parser, so that the units one charges do not hide in the other's.
	newSide := func(controlled bool) *TParser {
		p := &Parser{}
		if controlled {
			p.Control = newBudgetControl(context.Background(), parse.Limits{Compute: 1 << 62})
		}
		return &TParser{Parser: p}
	}
	used := func(tp *TParser) uint64 {
		if tp.Control == nil {
			return 0
		}
		return uint64(tp.Control.budgetUsed())
	}
	for _, controlled := range []bool{false, true} {
		fast, composed := newSide(controlled), newSide(controlled)
		c := &TActCtx{P: fast}
		for op := OpEq; op <= OpGe; op++ {
			for _, l := range strs {
				for _, r := range strs {
					beforeFast, beforeComposed := used(fast), used(composed)
					want, e := composed.TermBinary(op, l, r)
					if e != nil {
						t.Fatalf("controlled=%v: %q op %d %q fails: %v", controlled, l, op, r, e)
					}
					got, f := c.TermCompare(op, TOperand{Str: l, IsStr: true}, TOperand{Str: r, IsStr: true})
					if f != nil {
						t.Fatalf("controlled=%v: %q op %d %q fails: %v", controlled, l, op, r, f)
					}
					if got != want {
						t.Errorf("controlled=%v: %q op %d %q = %v, want %v", controlled, l, op, r, got, want)
					}
					if uf, uc := used(fast)-beforeFast, used(composed)-beforeComposed; uf != uc {
						t.Errorf("controlled=%v: %q op %d %q charges %d, the composition charges %d", controlled, l, op, r, uf, uc)
					}
				}
			}
		}
	}
}
