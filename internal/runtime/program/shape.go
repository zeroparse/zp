package program

import (
	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/runtime/nodes"
)

// SingleMode is how a parser takes the value of an expression where it takes one element: an
// alternative, the operand of ?, a capture or a recovered expression.
type SingleMode uint8

const (
	SingleNone  SingleMode = iota // no value
	SinglePass                    // the one element
	SingleTuple                   // the node of the tuple of the elements
)

// Shaper converts the result of an expression's matcher to the value where it takes one element
// or is a scope.
type Shaper struct {
	Elems  bool // the matcher returns the tuple node of its elements
	Shared bool // the tuple node is the result of a splice rule: a memo entry shares it
	K      int  // the number of valued elements
}

// One returns the one element of the result v.
func (s Shaper) One(v *core.Node) *core.Node {
	if s.Elems {
		if v == nil || len(nodes.D(v).Children) == 0 {
			return nil
		}
		return nodes.D(v).Children[0]
	}
	return v
}
