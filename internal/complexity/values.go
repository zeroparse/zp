package complexity

import "zeroparse.dev/zp/internal/ir/core"

// values are the value conditions of the rules of a module: where a predicate can read a value.
// The rules of the view that change values, which keep only ≡E,X, are off there, because
// spec/proofs/grammar-laws.md states ≡E for an expression only in a context that reads no value
// of it in a predicate. Elsewhere the rules apply.
type values struct {
	// reads holds, for each rule, the names of its captures that a predicate or a binding of the
	// rule reads.
	reads []map[string]bool
	// valued holds the rules whose value reaches such a capture: the rules called, through
	// positions that keep their value, from inside such a capture, and so on from their bodies.
	valued []bool
}

// valueReaders computes the value conditions of the rules of m.
func valueReaders(m *core.Module) *values {
	v := &values{reads: make([]map[string]bool, len(m.Rules)), valued: make([]bool, len(m.Rules))}
	var work []int
	mark := func(n core.Node) {
		for _, id := range valueCalls(n) {
			if !v.valued[id] {
				v.valued[id] = true
				work = append(work, id)
			}
		}
	}
	for i, r := range m.Rules {
		reads := map[string]bool{}
		core.Walk(r.Body, func(n core.Node) {
			var t core.Term
			switch n := n.(type) {
			case *core.Pred:
				t = n.Term
			case *core.BindNode:
				t = n.Term
			default:
				return
			}
			core.WalkTerm(t, func(t core.Term) {
				if s, ok := t.(*core.SlotRef); ok {
					reads[s.Name] = true
				}
			})
		})
		if len(reads) == 0 {
			continue
		}
		v.reads[i] = reads
		core.Walk(r.Body, func(n core.Node) {
			if c, ok := n.(*core.Capture); ok && reads[c.Name] {
				mark(c.Body)
			}
		})
	}
	for len(work) > 0 {
		id := work[len(work)-1]
		work = work[:len(work)-1]
		mark(m.Rules[id].Body)
	}
	return v
}

// observed reports whether a predicate of the rule cur can read a value in ns: one contains a
// capture whose name a predicate of the rule reads, or calls, in positions that keep their value, a
// rule whose value reaches such a capture. The rules that change values do not rewrite such a
// node, whose captures they would move or merge.
func (v *values) observed(cur int, ns ...core.Node) bool {
	reads := v.reads[cur]
	for _, n := range ns {
		found := false
		core.Walk(n, func(x core.Node) {
			if c, ok := x.(*core.Capture); ok && reads[c.Name] {
				found = true
			}
		})
		if found {
			return true
		}
		for _, id := range valueCalls(n) {
			if v.valued[id] {
				return true
			}
		}
	}
	return false
}

// valueCalls returns the rules that n calls in positions that can keep their value: all but those
// under a discard, a negative lookahead, and a positive lookahead outside its captures. A capture
// in a positive lookahead keeps its value beside the tuple of its scope, where actions and
// predicates read it. A negative lookahead has no captures and a capture under a discard is an
// error, so no value survives them.
func valueCalls(n core.Node) []int {
	dropped := map[*core.Call]bool{}
	drop := func(n core.Node) {
		core.Walk(n, func(x core.Node) {
			if c, ok := x.(*core.Call); ok {
				dropped[c] = true
			}
		})
	}
	core.Walk(n, func(x core.Node) {
		switch x := x.(type) {
		case *core.Drop:
			drop(x.Body)
		case *core.Look:
			if x.Neg {
				drop(x.Body)
				return
			}
			kept := map[*core.Call]bool{}
			core.Walk(x.Body, func(y core.Node) {
				if c, ok := y.(*core.Capture); ok {
					core.Walk(c.Body, func(z core.Node) {
						if r, ok := z.(*core.Call); ok {
							kept[r] = true
						}
					})
				}
			})
			core.Walk(x.Body, func(y core.Node) {
				if c, ok := y.(*core.Call); ok && !kept[c] {
					dropped[c] = true
				}
			})
		}
	})
	var out []int
	core.Walk(n, func(x core.Node) {
		if c, ok := x.(*core.Call); ok && !dropped[c] {
			out = append(out, c.Rule)
		}
	})
	return out
}
