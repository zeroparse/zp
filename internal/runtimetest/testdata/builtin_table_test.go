package runtime

import (
	"context"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec/conform"
)

// The built-in table (conform.BuiltinCases) on the compositions length, text, element and items of the
// node and the typed runtime (termLength, termText, termElement and termItems, printed from Exec IR),
// through the methods that a generated parser calls.

// nodeBuiltinFixture returns the context of a node parser over conform.BuiltinInput, with a budget
// control if controlled, and the values of the fixture.
func nodeBuiltinFixture(controlled bool) (*ActCtx, map[string]any) {
	p := &Parser{g: NewGrammar(100), unit: Bytes, bs: conform.BuiltinInput}
	if controlled {
		p.Control = newBudgetControl(context.Background(), Limits{Compute: 1 << 62})
	}
	c := &ActCtx{P: p}
	text := &Node{RawKind: RawKindMatch, RawTerminal: true, RawText: "xyz", RawStart: 2, RawEnd: 4}
	return c, map[string]any{
		conform.MemberNil:     nil,
		conform.MemberInt:     1,
		conform.BuiltinString: "ab",
		conform.BuiltinBool:   true,
		conform.BuiltinText:   text,
		conform.BuiltinNode:   &Node{RawKind: RawKindSeq, RawStart: 2, RawEnd: 5, RawChildren: []*Node{text, text}},
		conform.BuiltinPair:   c.List(1, 2),
		conform.MemberList:    c.List(),
		conform.BuiltinFunc:   func(x any) any { return x },
	}
}

// typedBuiltinFixture returns the context of a typed parser over conform.BuiltinInput and the values of
// the fixture.
func typedBuiltinFixture(controlled bool) (*TActCtx, map[string]any) {
	p := &Parser{unit: Bytes, bs: conform.BuiltinInput}
	if controlled {
		p.Control = newBudgetControl(context.Background(), Limits{Compute: 1 << 62})
	}
	c := &TActCtx{P: &TParser{Parser: p}}
	text := &TMatch{TNode: TNode[string]{Span: Span{2, 4}, Value: "xyz"}}
	return c, map[string]any{
		conform.MemberNil:     nil,
		conform.MemberInt:     1,
		conform.BuiltinString: "ab",
		conform.BuiltinBool:   true,
		conform.BuiltinText:   text,
		conform.BuiltinNode:   &TSeq{Typ: "tuple", Span: Span{2, 5}, Kids: []any{text, text}},
		conform.BuiltinPair:   &TSeq{Typ: TypeListValue, Kids: []any{1, 2}},
		conform.MemberList:    &TSeq{Typ: TypeListValue},
		conform.BuiltinFunc:   func(x any) any { return x },
	}
}

func TestValueBuiltinTable(t *testing.T) {
	for _, controlled := range []bool{false, true} {
		c, vals := nodeBuiltinFixture(controlled)
		if err := conform.RunBuiltins("node", func(call, value string) string {
			v := vals[value]
			return memberRead(func() any {
				switch call {
				case conform.CallLen:
					return c.Length(v)
				case conform.CallText:
					return c.TextStr(v)
				case conform.CallElement:
					if n := c.asNode(conform.BuiltinFn(call), v); n != nil {
						return n
					}
					return nil
				}
				return c.listNode(c.ListItems(conform.BuiltinFn(call), v))
			}, memberCanon)
		}); err != nil {
			t.Errorf("controlled=%v: %v", controlled, err)
		}
		tc, tvals := typedBuiltinFixture(controlled)
		if err := conform.RunBuiltins("typed", func(call, value string) string {
			v := tvals[value]
			return memberRead(func() any {
				switch call {
				case conform.CallLen:
					return tc.Length(v)
				case conform.CallText:
					return tc.TextStr(v)
				case conform.CallElement:
					e := tc.asNode(conform.BuiltinFn(call), v)
					if k := valKind(e); k == valKindInt || k == valKindString || k == valKindBool {
						// The typed runtime keeps a scalar element as it is, where the node runtime boxes it.
						return builtinBox{e}
					}
					return e
				}
				return &TSeq{Typ: TypeListValue, Kids: tc.ListItems(conform.BuiltinFn(call), v)}
			}, func(x any) string {
				if b, ok := x.(builtinBox); ok {
					return "box " + tmemberCanon(b.v)
				}
				return tmemberCanon(x)
			})
		}); err != nil {
			t.Errorf("controlled=%v: %v", controlled, err)
		}
	}
}

// builtinBox marks an element of the typed runtime that the node runtime boxes.
type builtinBox struct{ v any }
