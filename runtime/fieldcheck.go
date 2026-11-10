package runtime

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

// Cast returns v if it fits t, the type of a cast, and raises an evaluation error otherwise. The
// error names the input position where the action started and, when line is positive, the grammar
// position of the cast.
func (c *ActCtx) Cast(v any, t *FieldType, line, col int) any {
	if !c.P.termConforms(t, v) {
		c.P.castFailed(typeName(v), t.Text, c.start, line, col)
	}
	return v
}

// castFailed raises the evaluation error of a failed cast of a value of type from to type to, at
// input position pos and, when gline is positive, at grammar position gline:gcol.
func (p *Parser) castFailed(from, to string, pos, gline, gcol int) {
	e := p.MakeError(pos, nil)
	if gline > 0 {
		p.evalErrorf("cannot cast %s to %s at input position %d (line %d, column %d); cast at grammar position %d:%d", from, to, pos, e.Line, e.Col, gline, gcol)
	}
	p.evalErrorf("cannot cast %s to %s at input position %d (line %d, column %d)", from, to, pos, e.Line, e.Col)
}

// Cast is ActCtx.Cast for the values of the typed runtime.
func (c *TActCtx) Cast(v any, t *FieldType, line, col int) any {
	if !c.P.termConforms(t, v) {
		c.P.Parser.castFailed(ttypeName(v), t.Text, c.Start, line, col)
	}
	return v
}
