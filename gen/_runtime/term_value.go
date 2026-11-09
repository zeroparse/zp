package runtime

import "strings"

// The value primitives of terms (ops.AddValue) of the node and the typed runtime. The compositions
// of the operators in term_exec.go decide which of them applies; each acts on one kind of value.

// valKind is the operation val.kind: the kind of x. A node, a list, a tuple, a struct and a box are
// valKindOther; a box is opened by UnboxValue.
func valKind(x any) int {
	switch x.(type) {
	case nil:
		return valKindNil
	case int:
		return valKindInt
	case string:
		return valKindString
	case bool:
		return valKindBool
	}
	return valKindOther
}

// intCmp is the operation int.cmp: -1, 0 or 1 as the int l is less than, equal to or greater than the int r.
func intCmp(l, r any) int {
	a, b := l.(int), r.(int)
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// intIndex is the operation int.index: the int x and true if it is an index of n elements.
func intIndex(x any, n int) (int, bool) {
	k := x.(int)
	if k < 0 || k >= n {
		return 0, false
	}
	return k, true
}

// valItems is the operation seq.items in the node runtime.
func valItems(x any) ([]*Node, bool) {
	switch x := x.(type) {
	case nil:
		return nil, true
	case *Node:
		if x == nil {
			return nil, true
		}
		return x.RawChildren, true
	}
	return nil, false
}

// tvalItems is the operation seq.items in the typed runtime.
func tvalItems(x any) ([]any, bool) {
	if x == nil {
		return nil, true
	}
	if n, ok := x.(TVal); ok {
		return n.TKids(), true
	}
	return nil, false
}

// nodeOf is the operation node.of in the node runtime: x as a node, list, tuple or struct.
func nodeOf(x any) (*Node, bool) {
	n, ok := x.(*Node)
	return n, ok && n != nil
}

// tnodeOf is the operation node.of in the typed runtime.
func tnodeOf(x any) (TVal, bool) {
	n := AsTval(x)
	return n, n != nil
}

// tnodeStart and tnodeEnd are the operations node.start and node.end in the typed runtime.
func tnodeStart(n TVal) any {
	s, _ := n.TSpan()
	return s
}

func tnodeEnd(n TVal) any {
	_, e := n.TSpan()
	return e
}

// strAdd is the operation str.add. Under budget controls it copies the bytes in chunks, charging each.
func (p *Parser) strAdd(l, r any) any {
	ls, rs := l.(string), r.(string)
	if p.Control == nil {
		return ls + rs
	}
	var out strings.Builder
	out.Grow(checkedTextSize(len(ls), len(rs)))
	p.chargeValueBytes(len(ls) + len(rs))
	for _, s := range [...]string{ls, rs} {
		for off := 0; off < len(s); {
			n := min(spendValueChunk, len(s)-off)
			p.spendValue(n)
			out.WriteString(s[off : off+n])
			off += n
		}
	}
	return out.String()
}

// strCmp is the operation str.cmp.
func (p *Parser) strCmp(l, r any) int { return p.compareText(l.(string), r.(string)) }

// strEq is the operation str.eq.
func (p *Parser) strEq(l, r any) bool {
	ls, rs := l.(string), r.(string)
	if p.Control == nil {
		return ls == rs
	}
	return p.TextEqual(ls, rs)
}
