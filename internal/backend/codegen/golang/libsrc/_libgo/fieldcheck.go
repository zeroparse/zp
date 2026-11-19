package libgo

// FieldType is a type that a cast checks a dynamically typed value
// against. Generated code declares one for each cast.
//
// FieldType is for generated parsers; not a stable API.
type FieldType struct {
	Kind byte // i int, s string, b bool, z nil, n node[_], S node[string], v node[elem], E error, N struct or named node type, l list, t tuple (alts), o optional, u union, _ top
	Name string
	Elem *FieldType
	Alts []*FieldType
	Text string
}

// TermCastFailed returns the failure of a cast of v to t, a type that v does not fit. The failure names
// the input position where the action started and, when line is positive, the grammar position of the
// cast.
//
// TermCastFailed is for generated parsers; not a stable API.
func (c *ActCtx) TermCastFailed(v any, t *FieldType, line, col int) any {
	return c.P.castFailure(typeName(v), t.Text, c.start, line, col)
}

// TermCastFailed is ActCtx.TermCastFailed for the values of the typed library.
//
// TermCastFailed is for generated parsers; not a stable API.
func (c *TActCtx) TermCastFailed(v any, t *FieldType, line, col int) any {
	return c.P.Parser.castFailure(ttypeName(v), t.Text, c.Start, line, col)
}

// castFailure returns the evaluation error of a failed cast of a value of type from to type to, at
// input position pos and, when gline is positive, at grammar position gline:gcol.
func (p *Parser) castFailure(from, to string, pos, gline, gcol int) evalError {
	e := p.MakeError(pos, nil)
	if gline > 0 {
		return p.evalErrorValue("cannot cast %s to %s at input position %d (line %d, column %d); cast at grammar position %d:%d", from, to, pos, e.Line, e.Col, gline, gcol)
	}
	return p.evalErrorValue("cannot cast %s to %s at input position %d (line %d, column %d)", from, to, pos, e.Line, e.Col)
}
