//go:build !zp_workcount

package parser

// The accessors of work.go, without a count.

func charge(k workKind, n int) {}

func (l *lexer) at(i int) rune                   { return l.src[i] }
func (l *lexer) runes(start, end int) []rune     { return l.src[start:end] }
func (c *canonical) at(i int) rune               { return c.src[i] }
func (c *canonical) runes(start, end int) []rune { return c.src[start:end] }
func (p *parser) at(i int) *token                { return &p.toks[i] }
func (p *parser) tok() token                     { return p.toks[p.i] }
func (m *messages) at(i int) *token              { return &m.toks[i] }
