package core

import (
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/limits"
	"zeroparse.dev/zp/internal/value"
)

// builtinFunc is a function of the fixture of the built-in table, which is not a value of a term.
type builtinFunc struct{}

func (builtinFunc) Arity() int                  { return 1 }
func (builtinFunc) Apply(x, _ any) (any, error) { return x, nil }

// TestEngineValueBuiltinTable runs the built-in table (conform.BuiltinCases) on the engine's compositions
// length, text, element and items (termLength, termText, termElement and termItems, printed from Exec IR),
// through the methods that the backends call.
func TestEngineValueBuiltinTable(t *testing.T) {
	for _, controlled := range []bool{false, true} {
		prog := &SharedProgram{}
		InitSharedProgram(prog, compiletest.Program(t, "@start\nrule main = \"a\"\n", compile.Options{}))
		p := &Parser{Prog: prog, Input: NewInput(conform.BuiltinInput, Bytes)}
		if controlled {
			p.Control = NewBudgetControl(nil, limits.Limits{Compute: 1 << 62})
		}
		c := &EvalCtx{P: p}
		text := p.NewNode(value.Data{Kind: value.MatchKind, Terminal: true, Text: "xyz", Start: 2, End: 4})
		vals := map[string]any{
			conform.MemberNil:     nil,
			conform.MemberInt:     1,
			conform.BuiltinString: "ab",
			conform.BuiltinBool:   true,
			conform.BuiltinText:   text,
			conform.BuiltinNode:   p.NewNode(value.Data{Kind: value.SeqKind, Start: 2, End: 5, Children: []*value.Node{text, text}}),
			conform.MemberList:    c.ListNode(nil),
			conform.BuiltinFunc:   builtinFunc{},
		}
		one, _ := c.Element("list", 1)
		two, _ := c.Element("list", 2)
		vals[conform.BuiltinPair] = c.ListNode([]*value.Node{one, two})
		err := conform.RunBuiltins("engine", func(call, v string) string {
			var got any
			var err error
			switch call {
			case conform.CallLen:
				got, err = c.Builtin("len", []any{vals[v]})
			case conform.CallText:
				got, err = c.TextString(vals[v])
			case conform.CallElement:
				var n *value.Node
				if n, err = c.Element(conform.BuiltinFn(call), vals[v]); n != nil {
					got = n
				}
			default:
				var items []*value.Node
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
