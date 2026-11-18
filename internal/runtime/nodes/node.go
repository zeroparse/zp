// Package nodes gives the engine access to the representation of core.Node: the parsers, the
// Document, the compiler and the structural query build and read nodes through D. It also holds
// the node helpers that only the engine needs.
package nodes

import (
	"slices"
	"strings"
	"unsafe"

	"zeroparse.dev/zp/pkg/runtime/value"
	"zeroparse.dev/zp/pkg/runtime/value/impl"
)

// Data is the representation of a core.Node.
type Data = valueimpl.Data[value.Node]

// D returns the representation of n. A core.Node holds its representation as its only field.
func D(n *value.Node) *Data { return (*Data)(unsafe.Pointer(n)) }

// Of returns the node with the representation d.
func Of(d Data) value.Node { return *(*value.Node)(unsafe.Pointer(&d)) }

// Ptr returns a new node with the representation d.
func Ptr(d Data) *value.Node {
	n := new(value.Node)
	*D(n) = d
	return n
}

// IsNode reports whether n is a node of the value model (a string node, a tuple or list node, a
// node of a named node type, or an error value) rather than a struct, a box or a list or tuple
// value.
func IsNode(n *value.Node) bool {
	if k := D(n).Kind; k != nil && k.Base >= 0 && k.Base <= valueimpl.BaseMissing {
		return true
	}
	return D(n).Terminal
}

// UnboxNode returns the value that the carrier n stands for: the box's value for a box, nil for
// nil, and n otherwise.
func UnboxNode(n *value.Node) any {
	if n == nil {
		return nil
	}
	if D(n).Kind == valueimpl.BoxKind {
		return D(n).Box
	}
	return n
}

// UnboxValue is UnboxNode for a value that may be a carrier.
func UnboxValue(v any) any {
	if n, ok := v.(*value.Node); ok {
		return UnboxNode(n)
	}
	return v
}

// NodeTypeKinds are the kinds of the nodes of a named node type, by the base (BaseString,
// BaseTuple, BaseList) of the node converted to it. They have no rule.
type NodeTypeKinds [valueimpl.BaseList + 1]*valueimpl.Kind

// NewNodeTypeKinds returns the interned kinds of the named node type typ.
func NewNodeTypeKinds(typ string) *NodeTypeKinds {
	var ks NodeTypeKinds
	for base := range ks {
		ks[base] = valueimpl.NodeTypeKind(typ, base)
	}
	return &ks
}

// Convert returns the node of the named node type of ks that the value v converts to: a copy of
// v with the same range and content, or v itself if it already has the type. It reports false if
// v is not a string, tuple or list node.
func (ks *NodeTypeKinds) Convert(v any) (*value.Node, bool) {
	n, ok := v.(*value.Node)
	if !ok || n == nil || !HasBase(n) {
		return nil, false
	}
	return ks.Retype(n), true
}

// HasBase reports whether n is a string, tuple or list node, of a named node type or not: a node that
// a conversion converts.
func HasBase(n *value.Node) bool {
	k := D(n).Kind
	return k != nil && k.Base >= 0 && k.Base <= valueimpl.BaseList
}

// Retype returns n, a string, tuple or list node (HasBase), as a node of the named node type of ks: a
// copy of n with the same range and content, or n itself if it already has the type.
func (ks *NodeTypeKinds) Retype(n *value.Node) *value.Node {
	d := D(n)
	k := ks[d.Kind.Base]
	if d.Kind == k {
		return n
	}
	c := Ptr(*d)
	cd := D(c)
	cd.Kind, cd.Fresh = k, false
	if d.Fresh {
		// The rule that made n may still change it; the copy keeps what it holds now.
		cd.Children = slices.Clone(d.Children)
	}
	cd.Fields = slices.Clone(d.Fields)
	return c
}

// ConvertOwned gives n, a node that nothing else holds, the named node type of ks in place. It
// reports false if n is not a string, tuple or list node.
func (ks *NodeTypeKinds) ConvertOwned(n *value.Node) bool {
	if n == nil || !HasBase(n) {
		return false
	}
	D(n).Kind = ks[D(n).Kind.Base]
	return true
}

// FieldOf returns the value of the field name of n.
func FieldOf(n *value.Node, name string) (any, bool) {
	return D(n).Fields.Get(name)
}

// ShapeOf returns what n stands for in the value model.
func ShapeOf(n *value.Node) valueimpl.Shape { return valueimpl.ShapeOfKind(D(n).Kind, D(n).Terminal) }

// KindOfShape returns the core.NodeKind of a node that has the shape. It agrees with
// core.Node.Kind, which TestKindOfShape checks.
func KindOfShape(shape valueimpl.Shape) value.NodeKind {
	switch shape {
	case valueimpl.ShapeString:
		return value.KindString
	case valueimpl.ShapeTuple:
		return value.KindTuple
	case valueimpl.ShapeList:
		return value.KindList
	case valueimpl.ShapeError:
		return value.KindError
	case valueimpl.ShapeBox:
		return value.KindValue
	case valueimpl.ShapeTupleValue:
		return value.KindTupleValue
	case valueimpl.ShapeListValue:
		return value.KindListValue
	}
	return value.KindStruct
}

// ValueTypeName names the type of a value of the given shape in messages, in the terms of the
// value model: node[string] for a string node, node[(_, _)] and node[[]_] for tuple and list
// nodes (a tuple has elems elements), error for an error value, (_, _) and []_ for tuple and list
// values, and name for a struct.
func ValueTypeName(shape valueimpl.Shape, name string, elems int) string {
	switch shape {
	case valueimpl.ShapeString:
		return "node[string]"
	case valueimpl.ShapeTuple:
		return "node[" + tupleTerm(elems) + "]"
	case valueimpl.ShapeList:
		return "node[[]_]"
	case valueimpl.ShapeError:
		return "error"
	case valueimpl.ShapeTupleValue:
		return tupleTerm(elems)
	case valueimpl.ShapeListValue:
		return "[]_"
	}
	return name
}

// tupleTerm is the type term of a tuple of n elements of unknown types.
func tupleTerm(n int) string {
	if n == 1 {
		return "(_,)"
	}
	return "(" + strings.TrimSuffix(strings.Repeat("_, ", n), ", ") + ")"
}

// CaptureLocations returns the locations of the captures of the tuple node n, in name order (see
// coreimpl.CaptureLocations).
func CaptureLocations(n *value.Node) []valueimpl.CaptureLocation {
	return valueimpl.CaptureLocations(D(n), D)
}
