package core

import (
	"testing"

	"zeroparse.dev/zp/core/noderep"
)

// Clone copies a tree built without a generator: subtrees, slice views and cycles stay shared in the
// copy as they are in the original, and nothing of the copy aliases the original.
func TestCloneKeepsSharingAndCycles(t *testing.T) {
	leaf := &Node{}
	shared := []*Node{leaf, {}}
	values := []any{leaf, shared}
	root := &Node{}
	a := &Node{}
	b := &Node{}
	root.d.Children = []*Node{a, b, leaf}
	// a and b hold the same slice view in a field; b also holds it as its children.
	a.d.Fields = noderep.Fields{{Name: "xs", Value: shared}, {Name: "vs", Value: values}}
	b.d.Fields = noderep.Fields{{Name: "xs", Value: shared}, {Name: "back", Value: root}}
	b.d.Children = shared

	c := root.Clone()
	if c == root || c.d.Children[0] == a {
		t.Fatal("Clone returned the original nodes")
	}
	ca, cb, cleaf := c.d.Children[0], c.d.Children[1], c.d.Children[2]
	if c.d.Children[2] == leaf {
		t.Fatal("leaf was not copied")
	}
	xa, _ := ca.d.Fields.Get("xs")
	xb, _ := cb.d.Fields.Get("xs")
	sa, sb := xa.([]*Node), xb.([]*Node)
	if &sa[0] != &sb[0] || len(sa) != 2 {
		t.Error("a slice held by two fields is not shared in the copy")
	}
	if &sa[0] != &cb.d.Children[0] {
		t.Error("a slice held as fields and as children is not shared in the copy")
	}
	if &sa[0] == &shared[0] {
		t.Error("the copy aliases the original slice")
	}
	if sa[0] != cleaf {
		t.Error("a subtree reached through a slice and a child is not shared in the copy")
	}
	vs, _ := ca.d.Fields.Get("vs")
	if vs.([]any)[0] != cleaf || &vs.([]any)[1].([]*Node)[0] != &sa[0] {
		t.Error("nodes and slices inside a []any are not shared with the copy's")
	}
	back, _ := cb.d.Fields.Get("back")
	if back.(*Node) != c {
		t.Error("a cycle does not lead back to the copy's root")
	}
}
