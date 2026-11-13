package value

import (
	"zeroparse.dev/zp/runtime/noderep"
	"zeroparse.dev/zp/runtime/parse"
)

// The functions in this file build and change nodes for the tests of packages outside the engine,
// which cannot reach the fields of a Node. They are functions rather than methods so that they do
// not become part of the public Node API.

// NewNode returns a node of kind k with the range [start, end). text is the string of a
// KindString node, elems the elements of a tuple or list, typ the type name of a KindStruct node,
// fields its fields (or captures), and value the value of a KindValue node.
func NewNode(k parse.NodeKind, start, end int, text string, elems []*parse.Node, typ string, fields noderep.Fields, value any) *parse.Node {
	n := Ptr(Data{Start: int32(start), End: int32(end), Text: text, Children: elems, Fields: fields, Box: value})
	d := D(n)
	switch k {
	case parse.KindString:
		d.Kind, d.Terminal = noderep.MatchKind, true
	case parse.KindTuple:
		d.Kind = noderep.SeqKind
	case parse.KindList:
		d.Kind = noderep.ListKind
	case parse.KindError:
		d.Kind = noderep.ErrorKind
	case parse.KindStruct:
		d.Kind = noderep.KindOf(typ, "")
	case parse.KindTupleValue:
		d.Kind = noderep.TupleValueKind
	case parse.KindListValue:
		d.Kind = noderep.ListValueKind
	case parse.KindValue:
		d.Kind = noderep.BoxKind
	}
	return n
}

// SetNodeElems replaces the elements of the tuple or list n, which can make a cycle.
func SetNodeElems(n *parse.Node, elems []*parse.Node) { D(n).Children = elems }

// SetNodeText replaces the string of the KindString node n.
func SetNodeText(n *parse.Node, text string) {
	D(n).Text = text
}

// SetNodeStart replaces the start of the range of n.
func SetNodeStart(n *parse.Node, start int) { D(n).Start = int32(start) }

// SetNodeField sets the field name of n to v, adding it if absent.
func SetNodeField(n *parse.Node, name string, v any) {
	noderep.SetField(&D(n).Fields, name, v)
}
