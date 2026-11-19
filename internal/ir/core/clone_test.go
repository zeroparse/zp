package core

import (
	"reflect"
	"testing"
)

// TestClone checks that a copy prints as the module does, keeps the IDs, shares no node,
// alternative or line with it, keeps a node reached twice one node, and that changing the copy
// leaves the module as it was.
func TestClone(t *testing.T) {
	m := everyNode()
	Analyze(m)
	before := Print(m, true)
	c := m.Clone()
	if got := Print(c, true); got != before {
		t.Fatalf("copy prints differently:\n%s\nwant:\n%s", got, before)
	}
	mn, ma, ml := nodesOf(m)
	cn, ca, cl := nodesOf(c)
	for n := range cn {
		if mn[n] {
			t.Errorf("the copy shares a %T", n)
		}
	}
	for a := range ca {
		if ma[a] {
			t.Error("the copy shares an alternative")
		}
	}
	for l := range cl {
		if ml[l] {
			t.Error("the copy shares an Operators Rule line")
		}
	}
	if len(cn) != len(mn) {
		t.Errorf("the copy has %d nodes, the module %d", len(cn), len(mn))
	}
	var ids, cids []NodeID
	for i := range m.Rules {
		Walk(m.Rules[i].Body, func(n Node) { ids = append(ids, n.Attrs().ID) })
		Walk(c.Rules[i].Body, func(n Node) { cids = append(cids, n.Attrs().ID) })
	}
	if !reflect.DeepEqual(ids, cids) {
		t.Errorf("IDs of the copy %v, want %v", cids, ids)
	}
	ms := c.Rules[0].Body.(*Seq).Items[18].(*Missing)
	if Node(ms.Lit) != ms.Body {
		t.Error("the literal of the Missing is two nodes in the copy")
	}
	if c.Rules[0].Twin != c.Rules[2] {
		t.Error("the twin of the copy is not the copy's rule")
	}
	ops := c.Rules[1].Body.(*OperatorsRule)
	if ops.Ops[0].Line == m.Rules[1].Body.(*OperatorsRule).Ops[0].Line {
		t.Error("the copy shares an operator's line")
	}
	// Change the copy as passes do: replace nodes, set guards, change attributes.
	for _, r := range c.Rules {
		r.Body = Rewrite(r.Body, func(n Node) Node {
			if l, ok := n.(*Lit); ok {
				return &Lit{Text: l.Text + "!", Exp: l.Exp}
			}
			return n
		})
		Walk(r.Body, func(n Node) {
			n.Attrs().Want = !n.Attrs().Want
			if ch, ok := n.(*Choice); ok {
				for _, a := range ch.Alts {
					a.GuardedClass = true
				}
			}
		})
	}
	c.Rules[0].Name = "changed"
	c.Exps[0] = "changed"
	if got := Print(m, true); got != before {
		t.Errorf("changing the copy changed the module:\n%s\nwant:\n%s", got, before)
	}
}
