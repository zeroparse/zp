package core

import (
	"fmt"
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	"zeroparse.dev/zp/internal/ir/exec/conform"
	"zeroparse.dev/zp/internal/value"
	"zeroparse.dev/zp/runtime/noderep"
	"zeroparse.dev/zp/runtime/parse"
)

// TestEngineValueHullTable runs the hull table (conform.HullCases) on the engine's composition hull
// (termHull, printed from Exec IR), through the methods that the backends call to make a tuple and a list.
func TestEngineValueHullTable(t *testing.T) {
	for _, controlled := range []bool{false, true} {
		prog := &SharedProgram{}
		InitSharedProgram(prog, compiletest.Program(t, "@start\nrule main = \"a\"\n", compile.Options{}))
		p := &Parser{Prog: prog, Input: NewInput(conform.BuiltinInput, parse.Bytes)}
		if controlled {
			p.Control = NewBudgetControl(nil, parse.Limits{Compute: 1 << 62})
		}
		c := &EvalCtx{P: p, Start: conform.StructRuleStart, End: conform.StructRuleEnd}
		text := p.NewNode(value.Data{Kind: noderep.MatchKind, Terminal: true, Text: "xyz", Start: 2, End: 4})
		vals := map[string]any{
			conform.MemberNil:     nil,
			conform.MemberInt:     1,
			conform.BuiltinString: "ab",
			conform.BuiltinText:   text,
			conform.BuiltinNode:   p.NewNode(value.Data{Kind: noderep.SeqKind, Start: 2, End: 5, Children: []*parse.Node{text, text}}),
			conform.StructLate:    p.NewNode(value.Data{Kind: noderep.MatchKind, Terminal: true, Text: "q", Start: 7, End: 9}),
		}
		err := conform.RunHulls("engine", controlled, func(kind string, elems []string) (string, int) {
			var kids []*parse.Node
			for _, e := range elems {
				n, err := element(c, kind, vals[e])
				if err != nil {
					return "error: " + err.Error(), 0
				}
				kids = append(kids, n)
			}
			var n *parse.Node
			if kind == "tuple" {
				n = c.TupleNode(kids)
			} else {
				n = c.NewList(kids)
			}
			rng := fmt.Sprintf("%d-%d", value.D(n).Start, value.D(n).End)
			var before uint64
			if controlled {
				before = p.Control.Used
			}
			p.termHull(c, kind == "tuple", value.D(n).Children)
			if controlled {
				return rng, int(p.Control.Used - before)
			}
			return rng, 0
		})
		if err != nil {
			t.Errorf("controlled=%v: %v", controlled, err)
		}
	}
}
