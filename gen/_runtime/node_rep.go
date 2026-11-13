package runtime

import (
	"unsafe"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/core/noderep"
)

// Data is the representation of a core.Node, which the runtime builds and reads nodes through.
//
// Data is for generated parsers; not a stable API.
type Data = noderep.Data[core.Node]

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
	MatchKind = noderep.MatchKind
	SeqKind   = noderep.SeqKind
	ListKind  = noderep.ListKind
)

// TypeTupleValue and TypeListValue are the type names of tuple and list values.
//
// TypeTupleValue and TypeListValue are for generated parsers; not a stable API.
const (
	TypeTupleValue = noderep.TypeTupleValue
	TypeListValue  = noderep.TypeListValue
)

// BaseString is the base (noderep.Kind.Base) of string nodes.
//
// BaseString is for generated parsers; not a stable API.
const BaseString = noderep.BaseString

// baseKinds are the kinds without a rule of the bases a named node type converts.
var baseKinds = [...]*noderep.Kind{noderep.BaseString: noderep.MatchKind, noderep.BaseTuple: noderep.SeqKind, noderep.BaseList: noderep.ListKind}

// shape returns what the node n stands for in the value model.
func shape(n *core.Node) noderep.Shape { return noderep.ShapeOfKind(D(n).Kind, D(n).Terminal) }
