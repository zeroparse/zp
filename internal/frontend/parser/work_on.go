//go:build zp_workcount

package parser

import "sync/atomic"

// work holds the units of each kind of work charged since the last ResetWork.
var work [workKinds]atomic.Uint64

// charge counts n units of the work k.
func charge(k workKind, n int) { work[k].Add(uint64(n)) }

// Work returns the units of work that the lexer and the builder of the grammar have read since the
// last ResetWork: the code points of the source that the lexer reads, and the code points, comments,
// comment gaps and tokens that the builder, its layout replay and the error messages of a source read.
// The counters are shared by every goroutine, so a measure must not run while another parse does.
func Work() (lexer, builder uint64) { return work[lexerWork].Load(), work[builderWork].Load() }

// ResetWork sets the counters of Work to zero.
func ResetWork() {
	for k := range work {
		work[k].Store(0)
	}
}

// The accessors of work.go, which count what they read.

func (l *lexer) at(i int) rune {
	charge(lexerWork, 1)
	return l.src[i]
}

func (l *lexer) runes(start, end int) []rune {
	charge(lexerWork, end-start)
	return l.src[start:end]
}

func (c *canonical) at(i int) rune {
	charge(builderWork, 1)
	return c.src[i]
}

func (c *canonical) runes(start, end int) []rune {
	charge(builderWork, end-start)
	return c.src[start:end]
}

func (p *parser) at(i int) *token {
	charge(builderWork, 1)
	return &p.toks[i]
}

func (p *parser) tok() token {
	charge(builderWork, 1)
	return p.toks[p.i]
}

func (m *messages) at(i int) *token {
	charge(builderWork, 1)
	return &m.toks[i]
}
