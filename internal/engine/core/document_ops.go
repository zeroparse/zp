package core

import "zeroparse.dev/zp/internal/value"

// The engine's implementations of the Document operations (document_exec.go, printed from
// internal/ir/exec/core): memo.splice applies an edit at once only to the entries at the edited positions,
// and records it in the log, from which the printed AdvanceEntry applies it to any other entry when a
// lookup finds the entry; doc.move moves the nodes of a reused result in place.

// DocEdit is a Document edit: the positions [start, end) were replaced by end-start+delta
// positions.
type DocEdit struct{ Start, End, Delta int }

// EditMemo applies the edit ed to the memo table t (memo.splice) and returns the log edits with it.
// The result is what a parse reads through AdvanceEntry; the edit charges nothing.
func EditMemo(t *MemoTable, edits []DocEdit, ed DocEdit) []DocEdit {
	edits = append(edits, ed)
	t.Splice(ed.Start, ed.End, ed.Delta, func(e *MemoEntry) bool { return AdvanceEntry(e, edits) })
	return edits
}

// moveResult brings the positions of the result of the memo entry e, which edits have shifted, up
// to date.
//
// Nodes are moved in place rather than copied, so that reusing a result after an edit costs no
// allocation: a node's positions are valid after its first gen edits, and the later ones are
// replayed. No edit falls inside the input a reused result examined, so all the nodes of its tree
// move together, and for a non-empty node each edit either lies before it (the node moves by the
// edit's delta) or not (it stays), which its positions tell wherever the node is shared. An empty
// node at the point of an insertion is ambiguous: it may belong to a result before the insertion
// and to one after it at the same time. Empty nodes are therefore never modified; they are
// copied, moving with the node that contains them (or, at the root, with the memo entry).
func (p *Parser) moveResult(e *MemoEntry) {
	n := e.Node
	p.moveBase, p.baseShift = e.Gen, int(e.shift)
	e.Gen, e.shift, e.shifted = p.Gen, 0, false
	if n == nil {
		return
	}
	if p.Moved == nil {
		p.Moved = map[*value.Node]*value.Node{}
	}
	if value.D(n).Start == value.D(n).End {
		e.Node = p.MoveEmpty(n, p.baseShift)
	} else if value.D(n).Gen < p.Gen {
		p.MoveTree(n, p.ShiftOf(n))
	}
	p.ClearMovementSpend()
}

// ShiftOf returns how far the non-empty node n of the result being moved moves with the edits
// since the generation of n (Data.Gen). The nodes of a result move together. Those that were current when the entry's
// result was last current (gen at most moveBase) move by the entry's shift; others were moved through
// another result since, and the edits after that are replayed once per generation (shifts).
func (p *Parser) ShiftOf(n *value.Node) int {
	if value.D(n).Gen <= p.moveBase {
		return p.baseShift
	}
	if d, ok := p.Shifts[value.D(n).Gen]; ok {
		return d
	}
	s := int(value.D(n).Start)
	for _, ed := range p.Edits[value.D(n).Gen:p.Gen] {
		p.Spend()
		if s >= ed.End {
			s += ed.Delta
		}
	}
	if p.Shifts == nil {
		p.Shifts = map[uint32]int{}
	}
	p.Shifts[value.D(n).Gen] = s - int(value.D(n).Start)
	return s - int(value.D(n).Start)
}

// MoveTree moves the non-empty node n by shift (ShiftOf(n)), and its descendants, in place.
func (p *Parser) MoveTree(n *value.Node, shift int) {
	p.Spend()
	gen := value.D(n).Gen
	value.D(n).Start += int32(shift)
	value.D(n).End += int32(shift)
	value.D(n).Gen = p.Gen
	for i, c := range value.D(n).Children {
		p.Spend()
		if c != nil && value.D(c).Gen < p.Gen {
			value.D(n).Children[i] = p.moveChild(c, gen, shift)
		}
	}
	for i := range value.D(n).Fields {
		p.Spend()
		if c, ok := value.D(n).Fields[i].Value.(*value.Node); ok && c != nil && value.D(c).Gen < p.Gen {
			value.D(n).Fields[i].Value = p.moveChild(c, gen, shift)
		}
	}
}

// moveChild moves the descendant c of a node of generation gen that moved by shift, and returns
// it or its copy.
func (p *Parser) moveChild(c *value.Node, gen uint32, shift int) *value.Node {
	if value.D(c).Start != value.D(c).End {
		if value.D(c).Gen != gen {
			shift = p.ShiftOf(c)
		}
		p.MoveTree(c, shift)
		return c
	}
	// An empty node is not modified after it is made, so its positions are valid where its
	// parent's were, and it moves with its parent.
	if shift == 0 {
		return c
	}
	return p.MoveEmpty(c, shift)
}

// MoveEmpty returns a copy of the empty node n moved by shift. Its descendants move with it: empty
// ones are copied the same way, and non-empty ones (captured in a lookahead) are moved in place.
// Copies made in the same moveResult are shared.
func (p *Parser) MoveEmpty(n *value.Node, shift int) *value.Node {
	p.Spend()
	if c, ok := p.Moved[n]; ok {
		return c
	}
	c := p.NewNode(*value.D(n))
	value.D(c).Start += int32(shift)
	value.D(c).End += int32(shift)
	p.Moved[n] = c
	if value.D(n).Children != nil {
		value.D(c).Children = p.Nodes(len(value.D(n).Children))
		for i, ch := range value.D(n).Children {
			p.Spend()
			value.D(c).Children[i] = p.moveDescendant(ch, value.D(n).Gen, shift)
		}
	}
	if value.D(n).Fields != nil {
		value.D(c).Fields = p.fields(len(value.D(n).Fields))[:len(value.D(n).Fields)]
		for i, f := range value.D(n).Fields {
			p.Spend()
			if vn, ok := f.Value.(*value.Node); ok {
				f.Value = p.moveDescendant(vn, value.D(n).Gen, shift)
			}
			value.D(c).Fields[i] = f
		}
	}
	return c
}

// moveDescendant moves the descendant c of an empty node of generation gen that moved by shift.
func (p *Parser) moveDescendant(c *value.Node, gen uint32, shift int) *value.Node {
	switch {
	case c == nil:
		return nil
	case value.D(c).Start == value.D(c).End:
		return p.MoveEmpty(c, shift)
	case value.D(c).Gen < p.Gen:
		return p.moveChild(c, gen, shift)
	}
	return c
}

// moveValue moves the nodes of the value v of an element recorded in the generation base by
// shift, as moveResult moves the result of a memo entry, and returns it or its copy.
func (p *Parser) moveValue(v *value.Node, shift int, base uint32) *value.Node {
	if v == nil {
		return nil
	}
	if p.Moved == nil {
		p.Moved = map[*value.Node]*value.Node{}
	}
	p.moveBase, p.baseShift = base, shift
	if value.D(v).Start == value.D(v).End {
		v = p.MoveEmpty(v, shift)
	} else if value.D(v).Gen < p.Gen {
		p.MoveTree(v, p.ShiftOf(v))
	}
	p.ClearMovementSpend()
	return v
}
