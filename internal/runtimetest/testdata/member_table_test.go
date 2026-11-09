package runtime

import (
	"context"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec/conform"
)

// The member table (conform.MemberCases) on the composition member of the node and the typed runtime
// (termMember, printed from Exec IR), through Member, which a generated parser calls.

// memberCanon returns the canonical form of a value of the node runtime (conform.MemberCanon).
func memberCanon(x any) string {
	switch x := x.(type) {
	case nil:
		return conform.MemberCanon("nil", nil, nil)
	case int:
		return conform.MemberCanon("int", x, nil)
	case string:
		return conform.MemberCanon("string", x, nil)
	case bool:
		return conform.MemberCanon("bool", x, nil)
	case *Node:
		if x == nil {
			return conform.MemberCanon("nil", nil, nil)
		}
		elems := func() []string {
			var out []string
			for _, k := range x.RawChildren {
				out = append(out, memberCanon(UnboxNode(k)))
			}
			return out
		}
		switch x.RawKind {
		case kindBox:
			return "box " + memberCanon(x.box)
		case kindListValue:
			return conform.MemberCanon("list", nil, elems())
		case kindTupleValue:
			return conform.MemberCanon("tuple", nil, elems())
		}
		return x.Type()
	}
	panic("not a value of the node runtime")
}

// memberRead returns the canonical result of read, or the text of the failure it raises.
func memberRead(read func() any, canon func(any) string) (out string) {
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(evalError)
			if !ok {
				panic(r)
			}
			out = "error: " + e.msg
		}
	}()
	return canon(read())
}

// nodeMemberFixture returns the context of a node parser, with a budget control if controlled, and the
// values of the fixture.
func nodeMemberFixture(controlled bool) (*ActCtx, map[string]any) {
	g := NewGrammar(100)
	for _, s := range conform.MemberStructs {
		var pairs []string
		for _, f := range s.Fields {
			pairs = append(pairs, f[0], f[1])
		}
		g.DeclareStruct(s.Name, pairs...)
	}
	p := &Parser{g: g}
	if controlled {
		p.Control = newBudgetControl(context.Background(), Limits{Compute: 1 << 62})
	}
	c := &ActCtx{P: p}
	node := &Node{RawKind: RawKindSeq, RawStart: 2, RawEnd: 5}
	node.RawFields.set("y", &Node{RawKind: kindBox, box: 7})
	return c, map[string]any{
		conform.MemberNil:   nil,
		conform.MemberInt:   1,
		conform.MemberNode:  node,
		conform.MemberEmpty: c.NewStruct("S"),
		conform.MemberSet:   c.NewStruct("S", "A", 3, "B", "x"),
		conform.MemberList:  c.List(),
	}
}

// memberStruct is a struct of the typed runtime, as a generated struct type is: it holds every field of
// its type, each set or at its zero value.
type memberStruct struct {
	Span
	name   string
	fields map[string]any
}

func (v *memberStruct) TName() string                  { return v.name }
func (v *memberStruct) TText() (string, bool)          { return "", false }
func (v *memberStruct) TKids() []any                   { return nil }
func (v *memberStruct) TStruct() bool                  { return true }
func (v *memberStruct) TFresh() bool                   { return false }
func (v *memberStruct) TSetFresh(bool)                 {}
func (v *memberStruct) TField(name string) (any, bool) { f, ok := v.fields[name]; return f, ok }

// tmemberCanon returns the canonical form of a value of the typed runtime.
func tmemberCanon(x any) string {
	switch x := x.(type) {
	case nil:
		return conform.MemberCanon("nil", nil, nil)
	case int:
		return conform.MemberCanon("int", x, nil)
	case string:
		return conform.MemberCanon("string", x, nil)
	case bool:
		return conform.MemberCanon("bool", x, nil)
	case *TSeq:
		var elems []string
		for _, k := range x.Kids {
			elems = append(elems, tmemberCanon(k))
		}
		switch x.Typ {
		case TypeListValue:
			return conform.MemberCanon("list", nil, elems)
		case TypeTupleValue:
			return conform.MemberCanon("tuple", nil, elems)
		}
		return x.TName()
	case TVal:
		return x.TName()
	}
	panic("not a value of the typed runtime")
}

// typedMemberFixture returns the context of a typed parser and the values of the fixture.
func typedMemberFixture(controlled bool) (*TActCtx, map[string]any) {
	p := &Parser{}
	if controlled {
		p.Control = newBudgetControl(context.Background(), Limits{Compute: 1 << 62})
	}
	c := &TActCtx{P: &TParser{Parser: p}}
	s := func(a int, b string) *memberStruct {
		return &memberStruct{name: "S", fields: map[string]any{"A": a, "B": b, "L": &TSeq{Typ: TypeListValue},
			"T": &TSeq{Typ: TypeTupleValue, Kids: []any{0, ""}}, "In": &memberStruct{name: "Inner", fields: map[string]any{"N": 0}}}}
	}
	node := &TSeq{Typ: "tuple", Span: Span{2, 5}}
	node.set("y", 7)
	return c, map[string]any{
		conform.MemberNil:   nil,
		conform.MemberInt:   1,
		conform.MemberNode:  node,
		conform.MemberEmpty: s(0, ""),
		conform.MemberSet:   s(3, "x"),
		conform.MemberList:  &TSeq{Typ: TypeListValue},
	}
}

func TestValueMemberTable(t *testing.T) {
	for _, controlled := range []bool{false, true} {
		c, vals := nodeMemberFixture(controlled)
		if err := conform.RunMembers("node", func(value, member string) string {
			return memberRead(func() any { return c.Member(vals[value], member) }, memberCanon)
		}); err != nil {
			t.Errorf("controlled=%v: %v", controlled, err)
		}
		tc, tvals := typedMemberFixture(controlled)
		if err := conform.RunMembers("typed", func(value, member string) string {
			return memberRead(func() any { return tc.Member(tvals[value], member) }, tmemberCanon)
		}); err != nil {
			t.Errorf("controlled=%v: %v", controlled, err)
		}
	}
}
