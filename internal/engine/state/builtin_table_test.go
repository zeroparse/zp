package state

import (
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/core/impl"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/value"
)

// builtinFunc is a function of the fixture of the built-in table, which is not a value of a term.
type builtinFunc struct{}

func (builtinFunc) Arity() int                  { return 1 }
func (builtinFunc) Apply(x, _ any) (any, error) { return x, nil }

// TestEngineValueBuiltinTable runs the built-in table (conform.BuiltinCases) on the engine's compositions
// length, text, items and convert (termLength, termText, termItems and termConvert,
// printed from Exec IR), through the methods that the backends call.
func TestEngineValueBuiltinTable(t *testing.T) {
	for _, controlled := range []bool{false, true} {
		prog := &SharedProgram{}
		InitSharedProgram(prog, compiletest.Program(t, "@start\nrule main = \"a\"\n", compile.Options{}))
		p := &Parser{Prog: prog, Input: NewInput(conform.BuiltinInput, core.Bytes)}
		if controlled {
			p.Control = NewBudgetControl(nil, core.Limits{Compute: 1 << 62})
		}
		c := &EvalCtx{P: p}
		text := p.NewNode(value.Data{Kind: coreimpl.MatchKind, Terminal: true, Text: "xyz", Start: 2, End: 4})
		vals := map[string]any{
			conform.MemberNil:     nil,
			conform.MemberInt:     1,
			conform.BuiltinString: "ab",
			conform.BuiltinBool:   true,
			conform.BuiltinText:   text,
			conform.BuiltinNode:   p.NewNode(value.Data{Kind: coreimpl.SeqKind, Start: 2, End: 5, Children: []*core.Node{text, text}}),
			conform.MemberList:    c.ListNode(nil),
			conform.BuiltinFunc:   builtinFunc{},
			conform.BuiltinTyped:  value.NewNodeTypeKinds(conform.BuiltinOther).Retype(text),
		}
		nt := value.NewNodeTypeKinds(conform.BuiltinType)
		one, _ := element(c, "list", 1)
		two, _ := element(c, "list", 2)
		vals[conform.BuiltinPair] = c.ListNode([]*core.Node{one, two})
		vals[conform.BuiltinBox] = one
		err := conform.RunBuiltins("engine", func(call, v string) string {
			var got any
			var err error
			switch call {
			case conform.CallLen:
				got, err = p.termLength(vals[v])
			case conform.CallText:
				got, err = p.termText(vals[v])
			case conform.CallConvert:
				got, err = p.Convert(nt, vals[v])
			default:
				var items []*core.Node
				items, err = p.ValueListItems(conform.BuiltinFn(call), vals[v])
				got = c.ListNode(items)
			}
			if err != nil {
				return "error: " + err.Error()
			}
			return engineMemberCanon(got)
		})
		if err != nil {
			t.Errorf("controlled=%v: %v", controlled, err)
		}
	}
}

// element returns v as the composition push makes it an element of a list or a tuple that fn makes, and pops
// it again: the tables build the elements of their fixtures with it.
func element(c *EvalCtx, fn string, v any) (*core.Node, error) {
	base := len(c.P.KidStack)
	if err := c.Push(fn, v); err != nil {
		return nil, err
	}
	n := c.P.KidStack[base]
	c.P.KidStack = c.P.KidStack[:base]
	return n, nil
}
