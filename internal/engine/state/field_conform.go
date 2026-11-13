package state

import (
	"strconv"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/value"
)

// Convert evaluates a node type constructor T{v}: it converts v to the named node type of ks, as the
// composition convert (term_exec.go) does.
func (p *Parser) Convert(ks *value.NodeTypeKinds, v any) (any, error) {
	return p.termConvert(ks, v)
}

// FieldConforms reports whether v is a value of type t in the value model, as the composition conforms
// (term_exec.go) does. It charges a step for each list or tuple element it examines. node[T] is a node whose
// value is of type T (node[_] any node with a value), and error an error value (an error or a missing token);
// a list or tuple type takes a list or tuple value, not the node of a repetition or sequence.
func (p *Parser) FieldConforms(t *compile.FieldType, v any) bool {
	return p.termConforms(t, v)
}

// CastError reports a cast value that is not of the type it is cast to. pos is the input position
// where the evaluation started, and gline:gcol the grammar position of the cast (0 if unknown).
func (p *Parser) CastError(v any, t *compile.FieldType, pos, gline, gcol int) error {
	line, col := p.Input.lineCol(pos)
	parts := []string{"cannot cast ", TypeName(v), " to ", t.Text, " at input position ", strconv.Itoa(pos),
		" (line ", strconv.Itoa(line), ", column ", strconv.Itoa(col), ")"}
	if gline > 0 {
		parts = append(parts, "; cast at grammar position ", strconv.Itoa(gline), ":", strconv.Itoa(gcol))
	}
	return p.ActionError(parts...)
}
