package runtime

import (
	"fmt"
	"slices"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec/conform"
)

// The hull table (conform.HullCases) on the composition hull of the node and the typed runtime (termHull,
// printed from Exec IR), through the methods that a generated parser calls to make a tuple and a list.

func TestValueHullTable(t *testing.T) {
	for _, controlled := range []bool{false, true} {
		c, vals := nodeBuiltinFixture(controlled)
		vals[conform.StructLate] = &Node{RawKind: RawKindMatch, RawTerminal: true, RawText: "q", RawStart: 7, RawEnd: 9}
		c.start, c.end = conform.StructRuleStart, conform.StructRuleEnd
		if err := conform.RunHulls("node", controlled, func(kind string, elems []string) (string, int) {
			var n *Node
			if kind == "tuple" {
				var args []any
				for _, e := range elems {
					args = append(args, vals[e])
				}
				n = c.Tuple(args...).(*Node)
			} else {
				base := len(c.P.KidStack)
				for _, e := range elems {
					c.push("list", vals[e])
				}
				items := slices.Clone(c.P.KidStack[base:])
				c.P.DropKids(base)
				n = c.NewList(items)
			}
			rng := fmt.Sprintf("%d-%d", n.RawStart, n.RawEnd)
			var before uint64
			if controlled {
				before = c.P.Control.budgetUsed()
			}
			c.P.termHull(c, kind == "tuple", n.RawChildren)
			if controlled {
				return rng, int(c.P.Control.budgetUsed() - before)
			}
			return rng, 0
		}); err != nil {
			t.Errorf("controlled=%v: %v", controlled, err)
		}
		tc, tvals := typedBuiltinFixture(controlled)
		tvals[conform.StructLate] = &TMatch{TNode: TNode[string]{Span: Span{7, 9}, Value: "q"}}
		tc.Start, tc.End = conform.StructRuleStart, conform.StructRuleEnd
		if err := conform.RunHulls("typed", controlled, func(kind string, elems []string) (string, int) {
			var args []any
			for _, e := range elems {
				args = append(args, tvals[e])
			}
			var n *TSeq
			if kind == "tuple" {
				n = tc.Tuple(args...).(*TSeq)
			} else {
				n = tc.NewList(args).(*TSeq)
			}
			rng := fmt.Sprintf("%d-%d", n.Start, n.End)
			var before uint64
			if controlled {
				before = tc.P.Control.budgetUsed()
			}
			tc.P.termHull(tc, kind == "tuple", n.Kids)
			if controlled {
				return rng, int(tc.P.Control.budgetUsed() - before)
			}
			return rng, 0
		}); err != nil {
			t.Errorf("controlled=%v: %v", controlled, err)
		}
	}
}
