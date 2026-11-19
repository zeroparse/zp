package core

import "testing"

// everyNode returns a module whose rules hold a node of every type, with a node reached from two
// places (the literal of a Missing) and a value-free twin.
func everyNode() *Module {
	shared := lit("m")
	op := &OpLine{Body: lit("+")}
	body := seq(
		lit("a"), class('0', '9'), &Any{}, &Anchor{Kind: EndLine},
		choice(lit("b"), &Opt{Body: lit("c")}),
		&Repeat{Min: 0, Max: -1, Body: class('a', 'z')},
		&Look{Body: lit("d"), Neg: true}, &Cut{}, &Succeed{},
		&Call{Rule: 1}, &Nest{Rule: 1, Body: lit("e")},
		&Text{Body: lit("f")}, &Drop{Body: lit("g")},
		&Capture{Slot: 0, Name: "x", Body: lit("h")},
		&Pred{Term: &Const{Value: true}}, &BindNode{Var: "v", Term: &Const{Value: 1}},
		&Label{Body: lit("i")}, &Recover{Body: lit("j"), Skip: &Any{}},
		&Missing{Lit: shared, Body: shared}, &Stream{Repeat: &Repeat{Max: -1, Body: lit("k")}},
		&Scan{Max: -1}, &ClassRun{Classes: []*Class{{Ranges: []Range{{'a', 'b'}}}}},
		&Terminal{Body: lit("l")}, &FailNode{},
	)
	ops := &OperatorsRule{Operands: []*OpLine{{Body: lit("1")}}, Ops: []*Operator{{Kind: "infix", Level: 1, Line: op}}}
	r0 := rule(0, "a", body, "x")
	r1 := rule(1, "b", ops)
	r2 := rule(2, "a_", lit("t"))
	r0.Twin = r2
	m := module(r0, r1, r2)
	Number(m)
	return m
}

// nodesOf returns the nodes, alternatives and lines of the rules of m.
func nodesOf(m *Module) (nodes map[Node]bool, alts map[*Alt]bool, lines map[*OpLine]bool) {
	nodes, alts, lines = map[Node]bool{}, map[*Alt]bool{}, map[*OpLine]bool{}
	for _, r := range m.Rules {
		Walk(r.Body, func(n Node) {
			nodes[n] = true
			switch x := n.(type) {
			case *Choice:
				for _, a := range x.Alts {
					alts[a] = true
				}
			case *OperatorsRule:
				for _, l := range x.Lines() {
					lines[l] = true
				}
			}
		})
	}
	return
}

func TestNumber(t *testing.T) {
	m := everyNode()
	ids := map[NodeID]bool{}
	for _, r := range m.Rules {
		Walk(r.Body, func(n Node) {
			id := n.Attrs().ID
			if id == 0 || id > m.LastID {
				t.Errorf("%T: ID %d, LastID %d", n, id, m.LastID)
			}
			ids[id] = true
		})
	}
	// The literal of the Missing is reached twice and has one ID.
	if nodes, _, _ := nodesOf(m); len(ids) != len(nodes) {
		t.Errorf("%d IDs for %d nodes", len(ids), len(nodes))
	}
	last := m.LastID
	Number(m)
	if m.LastID != last {
		t.Errorf("numbering again changed LastID from %d to %d", last, m.LastID)
	}
	if id := m.NewID(); id != last+1 {
		t.Errorf("NewID = %d, want %d", id, last+1)
	}
}
