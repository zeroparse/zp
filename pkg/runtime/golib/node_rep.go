package golib

import (
	"unsafe"

	zpvalue "zeroparse.dev/zp/pkg/runtime/value"
	"zeroparse.dev/zp/pkg/runtime/value/impl"
)

// Data is the representation of a value.Node, which the library builds and reads nodes through.
//
// Data is for generated parsers; not a stable API.
type Data = valueimpl.Data[zpvalue.Node]

// D returns the representation of the node n. A value.Node holds a Data as its only field, so
// the conversion is free.
//
// D is for generated parsers; not a stable API.
func D(n *zpvalue.Node) *Data { return (*Data)(unsafe.Pointer(n)) }

// nodeOf returns the node whose representation is d.
func nodeOfData(d *Data) *zpvalue.Node { return (*zpvalue.Node)(unsafe.Pointer(d)) }

// The kinds of the reserved types without a rule that generated parsers name.
//
// MatchKind, SeqKind and ListKind are for generated parsers; not a stable API.
var (
	MatchKind = valueimpl.MatchKind
	SeqKind   = valueimpl.SeqKind
	ListKind  = valueimpl.ListKind
)

// TypeTupleValue and TypeListValue are the type names of tuple and list values.
//
// TypeTupleValue and TypeListValue are for generated parsers; not a stable API.
const (
	TypeTupleValue = valueimpl.TypeTupleValue
	TypeListValue  = valueimpl.TypeListValue
)

// BaseString is the base (coreimpl.Kind.Base) of string nodes.
//
// BaseString is for generated parsers; not a stable API.
const BaseString = valueimpl.BaseString

// baseKinds are the kinds without a rule of the bases a named node type converts.
var baseKinds = [...]*valueimpl.Kind{valueimpl.BaseString: valueimpl.MatchKind, valueimpl.BaseTuple: valueimpl.SeqKind, valueimpl.BaseList: valueimpl.ListKind}

// shape returns what the node n stands for in the value model.
func shape(n *zpvalue.Node) valueimpl.Shape { return valueimpl.ShapeOfKind(D(n).Kind, D(n).Terminal) }
