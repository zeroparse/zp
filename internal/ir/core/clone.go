package core

import "slices"

// Clone returns a copy of m that a pass may change without changing m: new rules, nodes,
// alternatives and Operators Rule lines, with the same IDs and attributes, and new lists of the
// rules, starts, expectations, classes, variables and the call graph. A node reached from two places
// of m is one node of the copy. What no pass changes in place is shared: the character classes, the
// first terminals, the action expressions, the slots and the checked types.
func (m *Module) Clone() *Module {
	c := &cloner{nodes: map[Node]Node{}, lines: map[*OpLine]*OpLine{}}
	out := *m
	out.Rules = make([]*Rule, len(m.Rules))
	rules := map[*Rule]*Rule{}
	for i, r := range m.Rules {
		if r == nil {
			continue
		}
		nr := *r
		nr.Body = c.node(r.Body)
		out.Rules[i] = &nr
		rules[r] = &nr
	}
	for _, r := range out.Rules {
		if r != nil && r.Twin != nil {
			r.Twin = rules[r.Twin]
		}
	}
	out.Starts = slices.Clone(m.Starts)
	out.Exps = slices.Clone(m.Exps)
	out.Classes = slices.Clone(m.Classes)
	out.Vars = slices.Clone(m.Vars)
	out.Calls = slices.Clone(m.Calls)
	out.SCC = slices.Clone(m.SCC)
	out.Cyclic = slices.Clone(m.Cyclic)
	return &out
}

// cloner copies the nodes of a module, each once.
type cloner struct {
	nodes map[Node]Node
	lines map[*OpLine]*OpLine
}

func (c *cloner) node(n Node) Node {
	if n == nil {
		return nil
	}
	if x, ok := c.nodes[n]; ok {
		return x
	}
	var out Node
	switch x := n.(type) {
	case *Lit:
		y := *x
		out = &y
	case *ClassNode:
		y := *x
		out = &y
	case *Any:
		y := *x
		out = &y
	case *Anchor:
		y := *x
		out = &y
	case *Seq:
		y := *x
		out = &y
	case *Choice:
		y := *x
		out = &y
	case *Opt:
		y := *x
		out = &y
	case *Repeat:
		y := *x
		out = &y
	case *Look:
		y := *x
		out = &y
	case *Cut:
		y := *x
		out = &y
	case *Succeed:
		y := *x
		out = &y
	case *FailNode:
		y := *x
		out = &y
	case *Call:
		y := *x
		out = &y
	case *Nest:
		y := *x
		out = &y
	case *OperatorsRule:
		y := *x
		out = &y
	case *Text:
		y := *x
		out = &y
	case *Drop:
		y := *x
		out = &y
	case *Capture:
		y := *x
		out = &y
	case *Pred:
		y := *x
		out = &y
	case *BindNode:
		y := *x
		out = &y
	case *Label:
		y := *x
		out = &y
	case *Recover:
		y := *x
		out = &y
	case *Missing:
		y := *x
		out = &y
	case *Stream:
		y := *x
		out = &y
	case *Scan:
		y := *x
		out = &y
	case *ClassRun:
		y := *x
		out = &y
	case *Terminal:
		y := *x
		out = &y
	default:
		panic("core: Clone of an unknown node")
	}
	c.nodes[n] = out
	switch y := out.(type) {
	case *Seq:
		y.Items = make([]Node, len(y.Items))
		for i, it := range n.(*Seq).Items {
			y.Items[i] = c.node(it)
		}
	case *Choice:
		y.Alts = make([]*Alt, len(y.Alts))
		for i, a := range n.(*Choice).Alts {
			na := *a
			na.Body = c.node(a.Body)
			y.Alts[i] = &na
		}
	case *Opt:
		y.Body = c.node(y.Body)
	case *Repeat:
		y.Body = c.node(y.Body)
	case *Look:
		y.Body = c.node(y.Body)
	case *Nest:
		y.Body = c.node(y.Body)
	case *OperatorsRule:
		y.Skip = c.line(y.Skip)
		y.Operands = make([]*OpLine, len(y.Operands))
		for i, l := range n.(*OperatorsRule).Operands {
			y.Operands[i] = c.line(l)
		}
		y.Ops = make([]*Operator, len(y.Ops))
		for i, op := range n.(*OperatorsRule).Ops {
			nop := *op
			nop.Line = c.line(op.Line)
			y.Ops[i] = &nop
		}
	case *Text:
		y.Body = c.node(y.Body)
	case *Drop:
		y.Body = c.node(y.Body)
	case *Capture:
		y.Body = c.node(y.Body)
	case *Label:
		y.Body = c.node(y.Body)
	case *Recover:
		y.Body = c.node(y.Body)
		y.Skip = c.node(y.Skip)
	case *Missing:
		if y.Lit != nil {
			y.Lit = c.node(y.Lit).(*Lit)
		}
		y.Body = c.node(y.Body)
	case *Stream:
		if y.Repeat != nil {
			y.Repeat = c.node(y.Repeat).(*Repeat)
		}
	case *ClassRun:
		y.Classes = slices.Clone(y.Classes)
	case *Terminal:
		y.Body = c.node(y.Body)
	}
	return out
}

func (c *cloner) line(l *OpLine) *OpLine {
	if l == nil {
		return nil
	}
	if x, ok := c.lines[l]; ok {
		return x
	}
	nl := *l
	c.lines[l] = &nl
	nl.Body = c.node(l.Body)
	return &nl
}
