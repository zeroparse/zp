package core

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/value"
	"zeroparse.dev/zp/runtime/noderep"
	"zeroparse.dev/zp/runtime/parse"
)

// engineMemberCanon returns the canonical form of a value of the engine (conform.MemberCanon).
func engineMemberCanon(x any) string {
	switch x := x.(type) {
	case nil:
		return conform.MemberCanon("nil", nil, nil)
	case int:
		return conform.MemberCanon("int", x, nil)
	case string:
		return conform.MemberCanon("string", x, nil)
	case bool:
		return conform.MemberCanon("bool", x, nil)
	case *parse.Node:
		if x == nil {
			return conform.MemberCanon("nil", nil, nil)
		}
		var elems []string
		for _, k := range value.D(x).Children {
			elems = append(elems, engineMemberCanon(value.UnboxNode(k)))
		}
		switch value.D(x).Kind {
		case noderep.BoxKind:
			return "box " + engineMemberCanon(value.D(x).Box)
		case noderep.ListValueKind:
			return conform.MemberCanon("list", nil, elems)
		case noderep.TupleValueKind:
			return conform.MemberCanon("tuple", nil, elems)
		}
		return x.Type()
	}
	panic("not a value of the engine")
}

// engineMemberFixture returns an evaluation context of a parser of a program that declares the struct
// types of the member table, with a budget control if controlled, and the values of the fixture.
func engineMemberFixture(t *testing.T, controlled bool) (*EvalCtx, map[string]any) {
	t.Helper()
	var src strings.Builder
	for _, s := range []string{"type Inner = { N int }", "type S = { A int; B string; L []int; T (int, string); In Inner }"} {
		src.WriteString(s + "\n")
	}
	src.WriteString("@start\nrule main = \"a\"\n")
	prog := &SharedProgram{}
	InitSharedProgram(prog, compiletest.Program(t, src.String(), compile.Options{}))
	for _, s := range conform.MemberStructs {
		for _, f := range s.Fields {
			if got := prog.StructZeroOf(s.Name).Zeros[f[0]]; got != f[1] {
				t.Fatalf("%s.%s has the zero descriptor %q, not %q of the member table", s.Name, f[0], got, f[1])
			}
		}
	}
	p := &Parser{Prog: prog}
	if controlled {
		p.Control = NewBudgetControl(nil, parse.Limits{Compute: 1 << 62})
	}
	c := &EvalCtx{P: p}
	node := p.NewNode(value.Data{Kind: noderep.SeqKind, Start: 2, End: 5})
	noderep.SetField(&value.D(node).Fields, "y", p.NewNode(value.Data{Kind: noderep.BoxKind, Box: 7}))
	empty, err := c.NewStruct("S", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	set, err := c.NewStruct("S", []string{"A", "B"}, []any{3, "x"})
	if err != nil {
		t.Fatal(err)
	}
	return c, map[string]any{
		conform.MemberNil:   nil,
		conform.MemberInt:   1,
		conform.MemberNode:  node,
		conform.MemberEmpty: empty,
		conform.MemberSet:   set,
		conform.MemberList:  c.ListNode(nil),
	}
}

// TestEngineValueMemberTable runs the member table (conform.MemberCases) on the engine's composition
// member (termMember, printed from Exec IR), through Member, which the backends call.
func TestEngineValueMemberTable(t *testing.T) {
	for _, controlled := range []bool{false, true} {
		c, vals := engineMemberFixture(t, controlled)
		err := conform.RunMembers("engine", func(v, member string) string {
			got, err := c.Member(vals[v], member)
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
