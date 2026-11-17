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
