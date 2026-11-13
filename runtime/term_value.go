package runtime

import (
	"strings"

	"zeroparse.dev/zp/runtime/noderep"
	"zeroparse.dev/zp/runtime/parse"
)

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
func valItems(x any) ([]*parse.Node, bool) {
	switch x := x.(type) {
	case nil:
		return nil, true
	case *parse.Node:
		if x == nil {
			return nil, true
		}
		return D(x).Children, true
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
func nodeOf(x any) (*parse.Node, bool) {
	n, ok := x.(*parse.Node)
	return n, ok && n != nil
}

// seqKind is the kind of a tuple value or a list value.
func seqKind(tuple bool) *noderep.Kind {
	if tuple {
		return noderep.TupleValueKind
	}
	return noderep.ListValueKind
}

// elemSpan is the operation seq.span: an element of a list or a tuple is a node, boxed if it was a scalar,
// and nil adds no range.
func elemSpan(it *parse.Node) (int, int, bool) {
	if it == nil {
		return 0, 0, false
	}
	return int(D(it).Start), int(D(it).End), true
}

// elemSpan is the operation seq.span: a node adds its range, and an int, a string or a bool adds the range
// of the rule, which the node runtimes give the box they hold it in.
func (c *TActCtx) elemSpan(it any) (int, int, bool) {
	switch t := it.(type) {
	case TVal:
		s, e := t.TSpan()
		return s, e, true
	case int, string, bool:
		return c.Start, c.End, true
	}
	return 0, 0, false
}

// setSpan is the operation node.setSpan.
func setSpan(n *parse.Node, start, end int) { D(n).Start, D(n).End = int32(start), int32(end) }

// made is the operation act.made.
func (c *ActCtx) made(n *parse.Node) { c.P.created = append(c.P.created, n) }

// made is the operation act.made in the typed runtime.
func (c *TActCtx) made(n TVal) { c.P.Created = append(c.P.Created, n) }

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

// tnodeHasText is the operation node.hasText in the typed runtime.
func tnodeHasText(n TVal) bool {
	_, ok := n.TText()
	return ok
}

// tnodeWidth is the operation node.width in the typed runtime.
func tnodeWidth(n TVal) any {
	start, end := n.TSpan()
	return end - start
}
