//go:build !zp_exec

package runtime

// The hand-written stream delivery and commitment; the build tag zp_exec selects the compositions
// printed from Exec IR instead (stream_exec.go).

// EmitStream passes a committed element of the start rule's #stream
// repetition to the caller and commits the input before end. Elements are
// final once emitted: a caller error ends the parse unchanged.
func (p *Parser) EmitStream(v *Node, end int) {
	p.Control.Check()
	p.streamReport() // the element is not emitted if it fails
	p.streamDeliver(v)
	p.Control.Check()
	p.streamCommit(end)
}

// streamCommit makes it final that the parse never backtracks before pos. The
// preceding unit is kept for detecting line starts.
func (p *Parser) streamCommit(pos int) {
	p.streamDiscard(pos - 1)
	p.streamRelease()
	if pos-p.Stream.pruned >= 1024 {
		p.streamPrune(pos)
	}
	if p.Stream.tail.limit > 0 {
		p.streamSettleErrors()
	}
}

// StreamsIn reports whether the #stream repetition of the rule id streams: the parse is a stream
// parse with a callback, the rule is its start rule, and the rule runs at the top (a recursive
// rule counts its entries, so its outermost entry is the first; any other rule is entered once).
//
// StreamsIn is for generated parsers; not a stable API.
func (p *Parser) StreamsIn(id int, rec bool) bool {
	return p.Stream != nil && p.Stream.Emit != nil && p.Stream.start == id && (!rec || p.Depth == 1)
}
