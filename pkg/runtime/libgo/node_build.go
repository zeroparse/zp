package libgo

import (
	zpvalue "zeroparse.dev/zp/pkg/runtime/value"
	"zeroparse.dev/zp/pkg/runtime/value/impl"
)

// The functions in this file build and change nodes for the tests of packages outside runtime, which
// cannot reach the fields of a Node. They are for generated parsers and tests; not a stable API.
// They are functions rather than methods so that they are not part of the Node API. Generated
// parsers re-export none of them.

// NewNode returns a node of kind k with the range [start, end). text is the string of a
// KindString node, elems the elements of a tuple or list, typ the type name of a KindStruct node,
// fields its fields (or captures), and value the value of a KindValue node.
//
// NewNode is for generated parsers and tests; not a stable API.
func NewNode(k zpvalue.NodeKind, start, end int, text string, elems []*zpvalue.Node, typ string, fields valueimpl.Fields, value any) *zpvalue.Node {
	d := &Data{Start: int32(start), End: int32(end), Text: text, Children: elems, Fields: fields, Box: value}
	switch k {
	case zpvalue.KindString:
		d.Kind, d.Terminal = valueimpl.MatchKind, true
	case zpvalue.KindTuple:
		d.Kind = valueimpl.SeqKind
	case zpvalue.KindList:
		d.Kind = valueimpl.ListKind
	case zpvalue.KindError:
		d.Kind = valueimpl.ErrorKind
	case zpvalue.KindStruct:
		d.Kind = KindOf(typ, "")
	case zpvalue.KindTupleValue:
		d.Kind = valueimpl.TupleValueKind
	case zpvalue.KindListValue:
		d.Kind = valueimpl.ListValueKind
	case zpvalue.KindValue:
		d.Kind = valueimpl.BoxKind
	}
	return nodeOfData(d)
}

// SetNodeElems replaces the elements of the tuple or list n, which can make a cycle.
//
// SetNodeElems is for generated parsers and tests; not a stable API.
func SetNodeElems(n *zpvalue.Node, elems []*zpvalue.Node) { D(n).Children = elems }

// SetNodeText replaces the string of the KindString node n.
//
// SetNodeText is for generated parsers and tests; not a stable API.
func SetNodeText(n *zpvalue.Node, text string) {
	D(n).Text = text
}

// SetNodeStart replaces the start of the range of n.
//
// SetNodeStart is for generated parsers and tests; not a stable API.
func SetNodeStart(n *zpvalue.Node, start int) { D(n).Start = int32(start) }

// SetNodeField sets the field name of n to v, adding it if absent.
//
// SetNodeField is for generated parsers and tests; not a stable API.
func SetNodeField(n *zpvalue.Node, name string, v any) {
	for i := range D(n).Fields {
		if D(n).Fields[i].Name == name {
			D(n).Fields[i].Value = v
			return
		}
	}
	D(n).Fields = append(D(n).Fields, valueimpl.NodeField{Name: name, Value: v})
}
