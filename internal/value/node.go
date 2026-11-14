// Package value gives the engine access to the representation of core.Node: the parsers, the
// Document, the compiler and the structural query build and read nodes through D. It also holds
// the node helpers that only the engine needs.
package value

import (
	"slices"
	"strings"
	"unsafe"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/core/impl"
)

// Data is the representation of a core.Node.
type Data = coreimpl.Data[core.Node]

// D returns the representation of n. A core.Node holds its representation as its only field.
func D(n *core.Node) *Data { return (*Data)(unsafe.Pointer(n)) }

// Of returns the node with the representation d.
func Of(d Data) core.Node { return *(*core.Node)(unsafe.Pointer(&d)) }

// Ptr returns a new node with the representation d.
func Ptr(d Data) *core.Node {
	n := new(core.Node)
	*D(n) = d
	return n
}

// IsNode reports whether n is a node of the value model (a string node, a tuple or list node, a
// node of a named node type, or an error value) rather than a struct, a box or a list or tuple
// value.
func IsNode(n *core.Node) bool {
	if k := D(n).Kind; k != nil && k.Base >= 0 && k.Base <= coreimpl.BaseMissing {
		return true
	}
	return D(n).Terminal
}

// UnboxNode returns the value that the carrier n stands for: the box's value for a box, nil for
// nil, and n otherwise.
func UnboxNode(n *core.Node) any {
	if n == nil {
		return nil
	}
	if D(n).Kind == coreimpl.BoxKind {
		return D(n).Box
	}
	return n
}

// UnboxValue is UnboxNode for a value that may be a carrier.
func UnboxValue(v any) any {
	if n, ok := v.(*core.Node); ok {
		return UnboxNode(n)
	}
	return v
}

// NodeTypeKinds are the kinds of the nodes of a named node type, by the base (BaseString,
// BaseTuple, BaseList) of the node converted to it. They have no rule.
type NodeTypeKinds [coreimpl.BaseList + 1]*coreimpl.Kind

// NewNodeTypeKinds returns the interned kinds of the named node type typ.
func NewNodeTypeKinds(typ string) *NodeTypeKinds {
	var ks NodeTypeKinds
	for base := range ks {
		ks[base] = coreimpl.NodeTypeKind(typ, base)
	}
	return &ks
}

// Convert returns the node of the named node type of ks that the value v converts to: a copy of
// v with the same range and content, or v itself if it already has the type. It reports false if
// v is not a string, tuple or list node.
func (ks *NodeTypeKinds) Convert(v any) (*core.Node, bool) {
	n, ok := v.(*core.Node)
	if !ok || n == nil || !HasBase(n) {
		return nil, false
	}
	return ks.Retype(n), true
}

// HasBase reports whether n is a string, tuple or list node, of a named node type or not: a node that
// a conversion converts.
func HasBase(n *core.Node) bool {
	k := D(n).Kind
	return k != nil && k.Base >= 0 && k.Base <= coreimpl.BaseList
}

// Retype returns n, a string, tuple or list node (HasBase), as a node of the named node type of ks: a
// copy of n with the same range and content, or n itself if it already has the type.
func (ks *NodeTypeKinds) Retype(n *core.Node) *core.Node {
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
func (ks *NodeTypeKinds) ConvertOwned(n *core.Node) bool {
	if n == nil || !HasBase(n) {
		return false
	}
	D(n).Kind = ks[D(n).Kind.Base]
	return true
}

// FieldOf returns the value of the field name of n.
func FieldOf(n *core.Node, name string) (any, bool) {
	return D(n).Fields.Get(name)
}

// ShapeOf returns what n stands for in the value model.
func ShapeOf(n *core.Node) coreimpl.Shape { return coreimpl.ShapeOfKind(D(n).Kind, D(n).Terminal) }

// KindOfShape returns the core.NodeKind of a node that has the shape. It agrees with
// core.Node.Kind, which TestKindOfShape checks.
func KindOfShape(shape coreimpl.Shape) core.NodeKind {
	switch shape {
	case coreimpl.ShapeString:
		return core.KindString
	case coreimpl.ShapeTuple:
		return core.KindTuple
	case coreimpl.ShapeList:
		return core.KindList
	case coreimpl.ShapeError:
		return core.KindError
	case coreimpl.ShapeBox:
		return core.KindValue
	case coreimpl.ShapeTupleValue:
		return core.KindTupleValue
	case coreimpl.ShapeListValue:
		return core.KindListValue
	}
	return core.KindStruct
}

// ValueTypeName names the type of a value of the given shape in messages, in the terms of the
// value model: node[string] for a string node, node[(_, _)] and node[[]_] for tuple and list
// nodes (a tuple has elems elements), error for an error value, (_, _) and []_ for tuple and list
// values, and name for a struct.
func ValueTypeName(shape coreimpl.Shape, name string, elems int) string {
	switch shape {
	case coreimpl.ShapeString:
		return "node[string]"
	case coreimpl.ShapeTuple:
		return "node[" + tupleTerm(elems) + "]"
	case coreimpl.ShapeList:
		return "node[[]_]"
	case coreimpl.ShapeError:
		return "error"
	case coreimpl.ShapeTupleValue:
		return tupleTerm(elems)
	case coreimpl.ShapeListValue:
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
func CaptureLocations(n *core.Node) []coreimpl.CaptureLocation {
	return coreimpl.CaptureLocations(D(n), D)
}
