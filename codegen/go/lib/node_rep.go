package libgo

import (
	"unsafe"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/core/impl"
)

// Data is the representation of a core.Node, which the library builds and reads nodes through.
//
// Data is for generated parsers; not a stable API.
type Data = coreimpl.Data[core.Node]

// D returns the representation of the node n. A core.Node holds a Data as its only field, so
// the conversion is free.
//
// D is for generated parsers; not a stable API.
func D(n *core.Node) *Data { return (*Data)(unsafe.Pointer(n)) }

// nodeOf returns the node whose representation is d.
func nodeOfData(d *Data) *core.Node { return (*core.Node)(unsafe.Pointer(d)) }

// The kinds of the reserved types without a rule that generated parsers name.
//
// MatchKind, SeqKind and ListKind are for generated parsers; not a stable API.
var (
	MatchKind = coreimpl.MatchKind
	SeqKind   = coreimpl.SeqKind
	ListKind  = coreimpl.ListKind
)

// TypeTupleValue and TypeListValue are the type names of tuple and list values.
//
// TypeTupleValue and TypeListValue are for generated parsers; not a stable API.
const (
	TypeTupleValue = coreimpl.TypeTupleValue
	TypeListValue  = coreimpl.TypeListValue
)

// BaseString is the base (coreimpl.Kind.Base) of string nodes.
//
// BaseString is for generated parsers; not a stable API.
const BaseString = coreimpl.BaseString

// baseKinds are the kinds without a rule of the bases a named node type converts.
var baseKinds = [...]*coreimpl.Kind{coreimpl.BaseString: coreimpl.MatchKind, coreimpl.BaseTuple: coreimpl.SeqKind, coreimpl.BaseList: coreimpl.ListKind}

// shape returns what the node n stands for in the value model.
func shape(n *core.Node) coreimpl.Shape { return coreimpl.ShapeOfKind(D(n).Kind, D(n).Terminal) }
