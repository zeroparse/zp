package state

import (
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	ir "zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/ir/exec/conform"
)

// TestEngineValueIterTable runs the iteration table (conform.IterCases) on the engine's compositions itemAt and
// filterKeep (termItemAt and termFilterKeep, printed from Exec IR), as terms compiled and run by the engine, which run the loops of the lowered built-ins.
func TestEngineValueIterTable(t *testing.T) {
	for _, controlled := range []bool{false, true} {
		prog := &SharedProgram{}
		cp := compiletest.Program(t, "@start\nrule main = \"a\"\n", compile.Options{})
		InitSharedProgram(prog, cp)
		p := &Parser{Prog: prog, Input: NewInput(conform.BuiltinInput, core.Bytes)}
		if controlled {
			p.Control = NewBudgetControl(nil, core.Limits{Compute: 1 << 62})
		}
		c := &EvalCtx{P: p}
		one, _ := element(c, "list", 1)
		two, _ := element(c, "list", 2)
		vals := map[string]any{
			conform.MemberNil:     nil,
			conform.MemberInt:     1,
			conform.BuiltinString: "ab",
			conform.BuiltinBool:   true,
			conform.MemberList:    c.ListNode(nil),
			conform.BuiltinPair:   c.ListNode([]*core.Node{one, two}),
		}
		// The budget-controls reference build charges nothing, so its units are not checked.
		err := conform.RunIters("engine", false, func(call, v string) (string, int) {
			var before uint64
			if controlled {
				before = p.Control.Used
			}
			got, err := iterate(c, cp, call, vals[v])
			steps := 0
			if controlled {
				steps = int(p.Control.Used - before)
			}
			if err != nil {
				return "error: " + err.Error(), steps
			}
			return engineMemberCanon(got), steps
		})
		if err != nil {
			t.Errorf("controlled=%v: %v", controlled, err)
		}
	}
}

// TestEngineValueItemAtOpensBoxes holds the primitive the loops of the iterating built-ins read their elements
// with: the elements of a list travel in boxes, and the function of the built-in is passed the ints themselves,
// from either end. A compiled term opens a box where it reads a parameter, so the terms of the table above
// cannot show a primitive that passes the box.
func TestEngineValueItemAtOpensBoxes(t *testing.T) {
	prog := &SharedProgram{}
	InitSharedProgram(prog, compiletest.Program(t, "@start\nrule main = \"a\"\n", compile.Options{}))
	p := &Parser{Prog: prog, Input: NewInput(conform.BuiltinInput, core.Bytes)}
	c := &EvalCtx{P: p}
	one, _ := element(c, "list", 1)
	two, _ := element(c, "list", 2)
	items := []*core.Node{one, two}
	for _, tc := range []struct {
		i     int
		right bool
		want  any
	}{{0, false, 1}, {1, false, 2}, {0, true, 2}, {1, true, 1}} {
		if got := p.termItemAt(items, tc.i, tc.right); got != tc.want {
			t.Errorf("itemAt(%d, right=%v) = %#v, want %#v", tc.i, tc.right, got, tc.want)
		}
	}
}

// iterate runs the call of the iteration table over x as a term: the built-in over the value x, with the function
// of the table, which returns the element it is passed (for filter, whether it is the int 1). The term is
// compiled and run by the engine, so the loop that runs is the production loop. The term's own charges (its
// subterms) are part of what the parser spends, so the table's steps are not checked here; the charge of each
// element is held by the term charge goldens.
func iterate(c *EvalCtx, cp *compile.Program, call string, x any) (any, error) {
	op := func(name string) *ir.Local { return &ir.Local{Kind: ir.LocalOp, Name: name} }
	lambda := func(params ...string) (*ir.Lambda, *ir.Local) {
		l := &ir.Lambda{Params: params}
		e := &ir.Local{Kind: ir.LocalParam, Name: params[len(params)-1], Lambda: l}
		l.Body = e
		return l, e
	}
	var term ir.Term
	switch call {
	case conform.CallFoldl, conform.CallFoldr:
		f, _ := lambda("acc", "e")
		term = &ir.Builtin{Func: call, Args: []ir.Term{&ir.Const{Value: nil}, op("x"), f}}
	case conform.CallMap:
		f, _ := lambda("e")
		term = &ir.Builtin{Func: call, Args: []ir.Term{op("x"), f}}
	case conform.CallFilter:
		f, e := lambda("e")
		f.Body = &ir.Binary{Op: "==", L: e, R: &ir.Const{Value: 1}}
		term = &ir.Builtin{Func: call, Args: []ir.Term{op("x"), f}}
	default: // conform.CallFilterSelf: the function returns the element, which is not a bool
		f, _ := lambda("e")
		term = &ir.Builtin{Func: conform.CallFilter, Args: []ir.Term{op("x"), f}}
	}
	prog, err := CompileTerm(term, TermEnv{Prog: cp, Scope: &compile.Scope{}})
	if err != nil {
		panic(err)
	}
	return prog.Eval(&EvalCtx{P: c.P, Locals: &Local{Name: "x", Val: x}})
}
