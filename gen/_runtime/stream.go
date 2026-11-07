package runtime

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"iter"
	"math"
	"slices"
	"unicode/utf8"
)

// streamInput is the input of a stream parse. Positions are absolute; the
// buffer holds the input from base on, read from reader as matching needs it.
// The generated input fields (in, bs and n) stay empty, so inlined fast paths
// fall back to the stream-aware peek and literal matching.
//
// Emit is for generated parsers; not a stable API.
type streamInput struct {
	reader *bufio.Reader
	in     []rune // CodePoints
	bs     []byte // Bytes
	base   int
	// baseLine and baseCol are the line and column (1-based, in the unit) of base.
	baseLine, baseCol int
	eof               bool
	// maxInput, when positive, is the longest stream the parse accepts, in units (streamFill).
	maxInput int
	// start is the Id of the start rule of the stream parse, the one rule whose #stream repetition
	// streams.
	start int
	// Emit receives the committed elements of the start rule's #stream
	// repetition with the parse's budget control. Nil collects the repetition
	// as a batch parse does.
	Emit func(*Node, *BudgetControl) error
	// memoBase is the position of memo.slots[0], the floor of the memo: the
	// entries before it were discarded.
	memoBase int
	// scratch is the capture scratch in use when the #stream repetition
	// started. Frames allocated after it belong to committed elements and
	// are reused.
	scratch frameScratchMark
	// nodeChunks counts the node chunks started since the last split.
	nodeChunks int
	// diagnostic receives the committed recovered errors; diagnosed is the
	// number of entries of p.Recovered already delivered to it or to tail,
	// and diagnosedSkip errors of the entry after them were delivered too
	// when it is the list diagnosedList, which took the place of delivered
	// entries (compactDiagnosed).
	diagnostic    func(*SyntaxError) error
	diagnosed     int
	diagnosedSkip int
	diagnosedList *errList
	tail          streamDiagnosticTail
	text          streamTextSnapshot
	// lines holds the line starts once the whole input is loaded and
	// nothing was discarded.
	lines []int
	// nlStarts holds the positions after each newline in [base, nlScanned) of the buffer, in
	// ascending order; nlScanned is the position up to which streamLineCol has scanned.
	nlStarts  []int
	nlScanned int
}

const streamTextChunk = 1024

// streamTextSnapshot owns a small immutable copy of already loaded input, so
// that the texts of nearby nodes share one allocation. Returned substrings
// retain at most one chunk; offsets (CodePoints) remain parser-owned.
type streamTextSnapshot struct {
	base, end int
	source    string
	offsets   []int32
}

// frameScratchMark is the allocation state of a frameScratch.
type frameScratchMark struct{ frameN, slotCI, slotOff int }

// rewind clears and reuses the frames and slots allocated after m.
func (a *frameScratch) rewind(m frameScratchMark) {
	for i := m.frameN; i < a.frameN; i++ {
		a.frames[i/nodeChunk][i%nodeChunk] = CaptureFrame{}
	}
	for ci := m.slotCI; ci <= a.slotCI && ci < len(a.slots); ci++ {
		lo, hi := 0, ptrChunk
		if ci == m.slotCI {
			lo = m.slotOff
		}
		if ci == a.slotCI {
			hi = a.slotOff
		}
		if lo < hi {
			clear(a.slots[ci][lo:hi])
		}
	}
	a.frameN, a.slotCI, a.slotOff = m.frameN, m.slotCI, m.slotOff
}

// streamReader charges one step per caller Read and reports a non-EOF error
// from that Read ahead of cancellation observed after it.
type streamReader struct {
	reader  io.Reader
	control *BudgetControl
}

func (r *streamReader) Read(dst []byte) (int, error) {
	r.control.Check()
	r.control.Charge(1)
	n, err := r.reader.Read(dst)
	if r.control.ctx != nil {
		if cerr := contextErr(r.control.ctx); cerr != nil {
			if err != nil && err != io.EOF {
				cerr = err
			}
			r.control.interrupted = true
			panic(fatal{err: cerr})
		}
	}
	return n, err
}

// RunParseStream parses the input read from reader with rule r, passing each
// committed element of the rule's #stream repetition to emit.
//
// RunParseStream is for generated parsers; not a stable API.
func RunParseStream(r *Rule, reader io.Reader, emit func(*Node) error, opts []ParseOption) error {
	var e func(*Node, *BudgetControl) error
	if emit != nil {
		e = func(n *Node, _ *BudgetControl) error { return emit(n) }
	}
	return RunParseStreamWithBudget(r, reader, e, opts)
}

// RunParseStreamWithBudget is RunParseStream with emit also given the parse's budget
// control, so that converting elements to typed values is charged to it.
//
// RunParseStreamWithBudget is for generated parsers; not a stable API.
func RunParseStreamWithBudget(r *Rule, reader io.Reader, emit func(*Node, *BudgetControl) error, opts []ParseOption) (err error) {
	o, err := ResolveOptions(r.g, append(opts[:len(opts):len(opts)], streaming))
	if err != nil {
		return err
	}
	if emit == nil && (o.diagnostic != nil || o.diagnosticLimit > 0) {
		// A diagnostic-only stream still needs element commit boundaries.
		emit = func(*Node, *BudgetControl) error { return nil }
	}
	var n *Node
	defer recoverParse(&n, &err)
	o = prepareOptions(o, 0, true)
	if o.control != nil {
		reader = &streamReader{reader: reader, control: o.control}
	}
	p := &Parser{unit: o.unit, MaxDepth: o.maxDepth}
	p.setGrammar(r.g)
	p.Expectations = o.expectations
	p.Control = o.control
	p.Observation = newObservation(&o.observe)
	p.Stream = &streamInput{reader: bufio.NewReader(reader), baseLine: 1, baseCol: 1, Emit: emit, diagnostic: o.diagnostic, start: r.Id}
	p.Stream.tail.limit = o.diagnosticLimit
	p.Stream.maxInput = o.maxInput
	o.control.attachMemory(p)
	p.Control.Check()
	_, ok := p.Call(r, 0)
	p.Control.Check()
	if ok && p.streamAtEnd() {
		// Recoveries in the successful suffix are reported before the result.
		if err := p.reportRecovered(); err != nil {
			return err
		}
		if p.Stream.tail.limit > 0 {
			return p.Stream.tail.result(p)
		}
		if len(p.Recovered) > 0 {
			return SyntaxErrors(p.recoveredErrors())
		}
		return nil
	}
	if ok {
		p.Expect(p.Pos, IdEndInput)
	}
	return p.MakeError(p.farthest, p.exp[p.expBase:])
}

// StreamStart is called when the start rule's #stream repetition starts.
// The capture frames allocated so far, including the start rule's, stay in
// use; elements allocate their nodes from chunks of their own.
func (p *Parser) StreamStart() {
	if nodeFrameScratch {
		a := &p.captureScratch
		p.Stream.scratch = frameScratchMark{a.frameN, a.slotCI, a.slotOff}
	} else {
		p.frameSlab = nil
	}
	p.nodeSlab, p.ptrSlab, p.fieldSlab = nil, nil, nil
	p.Stream.nodeChunks = 0
	// The chunks of the prefix stay in use for the whole stream; commits return only those taken
	// since.
	p.memKept += p.memUsed
	p.memUsed = 0
}

// streamReport delivers the recovered errors committed since the last report; an error of the
// diagnostic callback ends the parse with it, before the element is delivered.
func (p *Parser) streamReport() {
	if err := p.reportRecovered(); err != nil {
		panic(fatal{err})
	}
}

// streamDeliver passes the element v to the callback of the stream parse; v is final from then on,
// and a callback error ends the parse unchanged.
func (p *Parser) streamDeliver(v *Node) {
	if v != nil {
		v.RawFresh = false
	}
	p.streamFail(p.Stream.Emit(v, p.Control))
}

// streamFail ends the parse with the error err of the stream callback, if there is one.
func (p *Parser) streamFail(err error) {
	if err != nil {
		panic(fatal{err})
	}
}

// streamRelease releases what the committed elements held, for streamCommit.
func (p *Parser) streamRelease() {
	s := p.Stream
	p.Trail = p.Trail[:0]
	// Capture frames are referenced only by active calls, and the elements'
	// calls have returned, so the frames allocated since the repetition
	// started can be reused.
	if nodeFrameScratch {
		p.captureScratch.rewind(s.scratch)
	}
	// A chunk stays alive while anything in it is referenced and keeps alive
	// what its objects point to, so chunks shared by consecutive elements
	// would chain every emitted element to the parser. New chunks start once
	// the node chunk is nearly used up or more than one chunk was started.
	if len(p.nodeSlab) < nodeChunk/8 || s.nodeChunks >= 2 {
		p.nodeSlab, p.ptrSlab, p.fieldSlab = nil, nil, nil
		if !nodeFrameScratch {
			p.frameSlab = nil
		}
		s.nodeChunks = 0
		// The chunks let go of hold the elements delivered so far, and so does the text copied
		// for them: their bytes are returned, except those of the expectation chunk in use and
		// the frame chunk kept.
		p.memUsed = uint64(cap(p.arena))*expIDBytes + uint64(cap(p.frameSlab))*frameBytes
		if p.Control != nil {
			// The text snapshot in use stays held; the values before it were delivered.
			p.Control.values = 0
			if s.text.source != "" {
				p.Control.values = uint64(len(s.text.source)) + uint64(cap(s.text.offsets))*4
			}
		}
	}
}

// streamDropRecords discards what the parse keeps about the positions before pos besides the memo,
// which no later call reads, for streamCommit.
func (p *Parser) streamDropRecords(pos int) {
	// Completed calls may have shortened the trail before the commit;
	// its inactive capacity can still refer to committed elements.
	clear(p.Trail[:cap(p.Trail)])
	p.pruneSkipRecords(pos)
	p.Observation.dropDiscarded(pos)
}

// streamSettleErrors forgets the recovered errors before the boundary, for streamCommit.
func (p *Parser) streamSettleErrors() {
	s := p.Stream
	// The recovered errors before the boundary were delivered and
	// collected, and no active call restores a longer prefix of them:
	// the start rule's call saved an empty one, and the element marks
	// are no longer active. Memo entries own copies of their lists.
	if cap(p.Recovered) <= streamRecoveryScratchMax {
		clearSpend(p, p.Recovered[:cap(p.Recovered)])
		p.Recovered = p.Recovered[:0]
	} else {
		p.Recovered = nil
	}
	s.diagnosed, s.diagnosedSkip, s.diagnosedList = 0, 0, nil
	if p.Control != nil {
		// With a diagnostic limit, the errors are now held by the tail, which counts
		// those it keeps; without one, Recovered keeps them and errs keeps counting them.
		p.Control.errs = 0
	}
}

// reportRecovered delivers the recovered errors committed since the last
// call to the diagnostic callback and the retained tail. It runs only at
// stream boundaries, which recoveries in abandoned alternatives never reach.
func (p *Parser) reportRecovered() error {
	s := p.Stream
	if s.diagnostic == nil && s.tail.limit == 0 {
		return nil
	}
	if s.diagnosed > len(p.Recovered) {
		s.diagnosed = len(p.Recovered)
	}
	var errs []*SyntaxError
	for s.diagnosed < len(p.Recovered) {
		if e := p.Recovered[s.diagnosed]; e.list == nil {
			// An error, delivered without listing it.
			if e.err != nil {
				if err := p.reportError(e.err); err != nil {
					// The error ends the parse.
					return err
				}
			}
			s.diagnosed++
			continue
		}
		errs = appendRecErrors(errs[:0], p.Recovered[s.diagnosed:s.diagnosed+1])
		for _, original := range errs[s.partialDiagnosed(p):] {
			if err := p.reportError(original); err != nil {
				return err
			}
		}
		s.diagnosed, s.diagnosedSkip, s.diagnosedList = s.diagnosed+1, 0, nil
	}
	return nil
}

// reportError delivers the recovered error original to the diagnostic callback, as a copy that the
// callback owns, and to the retained tail.
func (p *Parser) reportError(original *SyntaxError) error {
	s := p.Stream
	p.Control.Check()
	if s.diagnostic != nil {
		e := *original
		e.Expected = append([]string(nil), e.Expected...)
		e.Messages = append([]string(nil), e.Messages...)
		if err := s.diagnostic(&e); err != nil {
			return err
		}
	}
	if s.tail.limit > 0 {
		s.tail.add(original)
	}
	p.Control.Check()
	return nil
}

// streaming marks the options of a stream entry point.
func streaming(o *parseOptions) { o.streaming = true }

// WithDiagnosticCallback reports each committed #recover error during ParseStream or Iterate,
// immediately before the corresponding stream element. Recoveries in abandoned alternatives are not
// reported. The callback receives an independent copy; returning an error stops parsing, suppresses
// that element and is returned unchanged. A nil callback disables delivery. The option is invalid
// with other entry points. Recoveries after the #stream repetition are reported before the result.
// With or without this option, recovered errors remain available at the end of the stream.
func WithDiagnosticCallback(f func(*SyntaxError) error) ParseOption {
	return func(o *parseOptions) { o.diagnostic = f }
}

// WithDiagnosticLimit retains at most n committed stream recoveries, keeping the latest ones.
// Zero retains every error, as without the option. Negative values are invalid. The option,
// including zero, is valid only with stream entry points. When errors are dropped, the result is
// a *TruncatedSyntaxErrors.
func WithDiagnosticLimit(n int) ParseOption {
	return func(o *parseOptions) { o.diagnosticLimit, o.diagnosticLimitSet = n, true }
}

// Keep only small recovery scratch slices across stream elements. The
// diagnostic tail, not this scratch, owns committed errors.
const streamRecoveryScratchMax = 16

// TruncatedSyntaxErrors is the incomplete result of a stream parsed with a
// positive diagnostic limit. Errors is the retained tail in occurrence order.
// Saturated counts are lower bounds; truncation and the tail remain exact.
type TruncatedSyntaxErrors struct {
	Errors          SyntaxErrors
	Total           uint64
	Dropped         uint64
	CountsSaturated bool
}

func (e *TruncatedSyntaxErrors) Error() string {
	return fmt.Sprintf("recovered syntax errors truncated (total %d, dropped %d):\n%s", e.Total, e.Dropped, e.Errors)
}

// Unwrap exposes the retained errors to errors.As as SyntaxErrors.
func (e *TruncatedSyntaxErrors) Unwrap() error { return e.Errors }

// streamDiagnosticTail is populated only at stream boundaries. The ring
// avoids shifting a large tail for each committed recovery.
type streamDiagnosticTail struct {
	limit           int
	errors          SyntaxErrors
	head            int
	total           uint64
	dropped         uint64
	truncated       bool
	countsSaturated bool
	// bytes counts the errors in errors (syntaxErrorSize), for the memory limit.
	bytes uint64
}

func (t *streamDiagnosticTail) add(e *SyntaxError) {
	t.bytes += syntaxErrorSize(e)
	if t.total == math.MaxUint64 {
		t.countsSaturated = true
	} else {
		t.total++
	}
	if len(t.errors) < t.limit {
		t.errors = append(t.errors, e)
		return
	}
	t.truncated = true
	if t.dropped == math.MaxUint64 {
		t.countsSaturated = true
	} else {
		t.dropped++
	}
	t.bytes -= syntaxErrorSize(t.errors[t.head])
	t.errors[t.head] = e
	t.head++
	if t.head == t.limit {
		t.head = 0
	}
}

func (t *streamDiagnosticTail) result(p *Parser) error {
	p.Control.Check()
	if len(t.errors) == 0 {
		return nil
	}
	if !t.truncated {
		return t.errors
	}
	ordered := t.errors
	if t.head != 0 {
		p.Control.Charge(uint64(len(t.errors)))
		ordered = append(append(make(SyntaxErrors, 0, len(t.errors)), t.errors[t.head:]...), t.errors[:t.head]...)
	}
	p.Control.Check()
	return &TruncatedSyntaxErrors{Errors: ordered, Total: t.total, Dropped: t.dropped, CountsSaturated: t.countsSaturated}
}

// ErrStreamClosed is returned by Next after an iterator is closed before the end of its stream.
var ErrStreamClosed = errors.New("stream iterator is closed")
var errIteratorStop = errors.New("stream iterator stopped")

// StreamPull advances a stream parse synchronously to one committed element
// at a time. The standard library pull coroutine has no independently
// scheduled producer.
//
// StreamPull is for generated parsers; not a stable API.
type StreamPull[T any] struct {
	next     func() (T, bool)
	stop     func()
	terminal bool
	err      error
}

// IterateStream validates the options of a stream entry point of grammar g and returns a
// lazy iterator over the elements that parse passes to emit.
//
// IterateStream is for generated parsers; not a stable API.
func IterateStream[T any](g *grammar, opts []ParseOption, parse func(emit func(T) error) error) (*StreamPull[T], error) {
	if _, err := ResolveOptions(g, append(opts[:len(opts):len(opts)], streaming)); err != nil {
		return nil, err
	}
	it := &StreamPull[T]{}
	it.next, it.stop = iter.Pull(func(yield func(T) bool) {
		it.err = parse(func(v T) error {
			if !yield(v) {
				return errIteratorStop
			}
			return nil
		})
	})
	return it, nil
}

func (it *StreamPull[T]) Next() (v T, ok bool, err error) {
	if it.terminal {
		return v, false, it.err
	}
	defer func() {
		if x := recover(); x != nil {
			// Pull propagates the original caller panic after the parser unwinds.
			it.terminal = true
			it.next, it.stop = nil, nil
			panic(x)
		}
	}()
	v, ok = it.next()
	if ok {
		return v, true, nil
	}
	it.terminal = true
	it.next, it.stop = nil, nil
	return v, false, it.err
}

func (it *StreamPull[T]) Close() {
	if it.terminal {
		return
	}
	it.terminal = true
	stop := it.stop
	it.next, it.stop = nil, nil
	defer func() { it.err = ErrStreamClosed }()
	stop()
}

// StreamIterator returns the committed elements of a stream one at a time. It is single-owner: Next
// and Close must not overlap, including from callbacks. Close releases the parser state but never
// closes or interrupts the reader.
type StreamIterator struct{ pull *StreamPull[*Node] }

// Next returns (node, true, nil) for each committed element, with a nil node for an element without a
// value. At the end of a clean stream it returns (nil, false, nil), and after a failure (nil, false,
// err); later calls return the same result.
func (it *StreamIterator) Next() (*Node, bool, error) { return it.pull.Next() }

// Close abandons unfinished parsing, after which Next returns ErrStreamClosed. It is idempotent and does
// not change the result of a finished stream. It cannot interrupt a Read already blocked in caller
// code, and must not overlap a call to Next.
func (it *StreamIterator) Close() { it.pull.Close() }

// streamDiscard discards the input before keep. The buffer holds read-ahead
// input, so it is compacted only once at least half of it can go.
func (p *Parser) streamDiscard(keep int) {
	s := p.Stream
	n := keep - s.base
	if n <= 0 || 2*n < len(s.in)+len(s.bs) {
		return
	}
	s.baseLine, s.baseCol = p.streamLineCol(keep)
	// Drop the line starts up to the new base; streamLineCol has scanned up to keep.
	k, _ := slices.BinarySearch(s.nlStarts, keep+1)
	s.nlStarts = s.nlStarts[:copy(s.nlStarts, s.nlStarts[k:])]
	if p.unit == Bytes {
		p.SpendRead(uint64(len(s.bs) - n))
		s.bs = s.bs[:copy(s.bs, s.bs[n:])]
	} else {
		p.SpendRead(uint64(len(s.in) - n))
		s.in = s.in[:copy(s.in, s.in[n:])]
	}
	s.base = keep
}

// pruneStreamMemo discards the memo entries before pos and makes pos the floor of the memo, below
// which lookups find nothing and stores keep nothing (memo.prune).
func (p *Parser) pruneStreamMemo(pos int) {
	s, t := p.Stream, &p.memo
	k := pos - s.memoBase
	if k <= 0 {
		return
	}
	for _, head := range t.slots[:min(k, len(t.slots))] {
		for e := head; e != nil; {
			p.Spend()
			next := e.next
			*e = memoEntry{next: t.free}
			t.free = e
			e = next
		}
	}
	// The slots of the pruned positions are returned; the entries go to the free list, whose
	// reuse is not charged again.
	t.memBytes -= min(t.memBytes, uint64(min(k, len(t.slots)))*ptrBytes)
	if k >= len(t.slots) {
		clear(t.slots)
		t.slots = t.slots[:0]
	} else {
		n := copy(t.slots, t.slots[k:])
		clear(t.slots[n:])
		t.slots = t.slots[:n]
	}
	if t.index != nil {
		moved := make(map[int]memoIndex[memoEntry], len(t.index))
		t.indexBytes = 0
		for at, ix := range t.index {
			if at >= k {
				moved[at-k] = ix
				t.indexBytes += uint64(len(ix)+1) * memoIndexEntryBytes
			}
		}
		t.index = moved
	}
	s.memoBase = pos
}

// streamLoaded returns the position of the end of the input read so far.
func (s *streamInput) loaded() int {
	return s.base + len(s.in) + len(s.bs)
}

// streamFill reads the input up to just before position n and reports whether
// it is available. Once the required amount has been read, it reads only what
// is already buffered and does not wait for more.
func (p *Parser) streamFill(n int) bool {
	p.Control.demand(n)
	ok := p.streamFillTo(n)
	if max := p.Stream.maxInput; max > 0 && n > max && p.Stream.loaded() > max {
		// The outcome does not depend on how the reader chunks the stream: the parser needed the
		// unit at MaxInput and the stream holds it. loaded is at least n unless the stream ended.
		panic(fatal{err: &LimitError{Kind: LimitInput, Limit: uint64(max), Requested: uint64(min(n, p.Stream.loaded()))}})
	}
	return ok
}

func (p *Parser) streamFillTo(n int) bool {
	s := p.Stream
	for s.loaded() < n && !s.eof {
		p.streamRead()
		for s.reader.Buffered() > 0 {
			if p.unit == CodePoints {
				if b, _ := s.reader.Peek(s.reader.Buffered()); !utf8.FullRune(b) {
					break
				}
			}
			p.streamRead()
		}
	}
	return s.loaded() >= n
}

func (p *Parser) streamRead() {
	p.Control.Check()
	p.Spend()
	s := p.Stream
	var err error
	if p.unit == Bytes {
		var b byte
		if b, err = s.reader.ReadByte(); err == nil {
			s.bs = append(s.bs, b)
		}
	} else {
		var r rune
		if r, _, err = s.reader.ReadRune(); err == nil {
			s.in = append(s.in, r)
		}
	}
	if err == io.EOF {
		s.eof = true
	} else if err != nil {
		panic(fatal{err})
	}
}

// streamPeek is peek on a stream. A complete character never waits for more
// input; an incomplete UTF-8 prefix waits for more bytes or the end of input.
func (p *Parser) streamPeek() (rune, int, bool) {
	p.SpendRead(1)
	s := p.Stream
	if p.Pos < s.base || !p.streamFill(p.Pos+1) {
		return 0, 0, false
	}
	if p.unit == CodePoints {
		return s.in[p.Pos-s.base], 1, true
	}
	b := s.bs[p.Pos-s.base:]
	if b[0] < utf8.RuneSelf {
		return rune(b[0]), 1, true
	}
	for !s.eof && !utf8.FullRune(b) {
		p.streamFillTo(s.loaded() + 1)
		b = s.bs[p.Pos-s.base:]
	}
	p.SpendRead(uint64(min(utf8.UTFMax, len(b))))
	r, size := utf8.DecodeRune(b)
	return r, size, true
}

// streamByte returns the byte at position i (Bytes).
func (p *Parser) streamByte(i int) (byte, bool) {
	p.SpendRead(1)
	s := p.Stream
	if i < s.base || !p.streamFill(i+1) {
		return 0, false
	}
	return s.bs[i-s.base], true
}

// streamMatchLiteral is MatchLiteral on a stream. It reads only as far as the
// literal matches.
func (p *Parser) streamMatchLiteral(rs []rune, text string, desc ExpID, build bool) (*Node, bool) {
	start := p.Pos
	s := p.Stream
	if p.unit == Bytes {
		for i := 0; i < len(text); i++ {
			if c, ok := p.streamByte(p.Pos); !ok || c != text[i] {
				p.Pos = start
				return p.matchReplacementLiteral(rs, text, desc, build)
			}
			p.Pos++
		}
	} else {
		for _, r := range rs {
			p.SpendRead(1)
			if p.Pos < s.base || !p.streamFill(p.Pos+1) || s.in[p.Pos-s.base] != r {
				p.Pos = start
				p.Expect(start, desc)
				return nil, false
			}
			p.Pos++
		}
	}
	if !build {
		return nil, true
	}
	return p.NewNode(Node{RawKind: RawKindMatch, RawStart: int32(start), RawEnd: int32(p.Pos), RawText: p.LiteralText(start, text), RawTerminal: true, RawFresh: true}), true
}

func (p *Parser) streamAtLineStart() bool {
	if p.Pos == 0 {
		return true
	}
	if p.unit == Bytes {
		c, ok := p.streamByte(p.Pos - 1)
		return ok && c == '\n'
	}
	s := p.Stream
	i := p.Pos - 1
	return i >= s.base && p.streamFill(i+1) && s.in[i-s.base] == '\n'
}

func (p *Parser) streamAtEnd() bool {
	p.SpendRead(1)
	return !p.streamFill(p.Pos + 1)
}

// streamText returns owned text between start and end. The part already
// discarded is cut off.
func (p *Parser) streamText(start, end int) string {
	s := p.Stream
	start, end = max(start, s.base), max(end, s.base)
	if start == end {
		return ""
	}
	c := &s.text
	if start >= c.base && end <= c.end && c.source != "" {
		if p.unit == Bytes {
			return c.source[start-c.base : end-c.base]
		}
		return c.source[c.offsets[start-c.base]:c.offsets[end-c.base]]
	}
	// Do not read ahead for text or repeatedly copy tiny reader fragments.
	// Oversized ranges are copied on their own.
	if end-start > streamTextChunk || s.loaded()-start < 64 {
		return p.streamCopy(start, end)
	}
	snap := streamTextSnapshot{base: start, end: start + min(streamTextChunk, s.loaded()-start)}
	snap.source = p.streamCopy(snap.base, snap.end)
	if p.unit == Bytes {
		*c = snap
		return snap.source[:end-start]
	}
	if p.Control != nil {
		// An interruption while filling must not leave partial offsets in
		// the snapshot in use, so a controlled parse fills separate storage.
		snap.offsets = make([]int32, snap.end-snap.base+1)
		p.chargeValueBytes(4 * len(snap.offsets))
	} else if n := snap.end - snap.base + 1; cap(c.offsets) >= n {
		snap.offsets = c.offsets[:n]
	} else {
		snap.offsets = make([]int32, n)
	}
	i := 0
	for off := range snap.source {
		if i&127 == 0 {
			p.SpendRead(uint64(min(128, snap.end-snap.base-i)))
		}
		snap.offsets[i] = int32(off)
		i++
	}
	snap.offsets[i] = int32(len(snap.source))
	*c = snap
	return snap.source[:snap.offsets[end-start]]
}

// streamCopy copies the loaded text between start and end.
func (p *Parser) streamCopy(start, end int) string {
	s := p.Stream
	start, end = start-s.base, end-s.base
	if p.unit == Bytes {
		if p.Control != nil {
			p.Control.Charge(uint64(end - start))
		}
		p.chargeValueBytes(end - start)
		return string(s.bs[start:end])
	}
	if p.Control != nil {
		p.Control.Charge(2 * uint64(end-start))
	}
	text := string(s.in[start:end])
	p.chargeValueBytes(len(text))
	return text
}

// streamLineCol returns the line and column of pos, counted from base.
func (p *Parser) streamLineCol(pos int) (int, int) {
	s := p.Stream
	if s.eof && s.base == 0 {
		// The whole input is loaded: look the line up in a table of line
		// starts, built on first use, instead of scanning for each error.
		if s.lines == nil {
			s.lines = []int{0}
			for i := range s.loaded() {
				p.SpendRead(1)
				if p.unit == Bytes && s.bs[i] == '\n' || p.unit == CodePoints && s.in[i] == '\n' {
					s.lines = append(s.lines, i+1)
				}
			}
		}
		pos = min(pos, s.loaded())
		lo, hi := 0, len(s.lines) // the number of line starts at or before pos
		for lo < hi {
			if m := int(uint(lo+hi) >> 1); s.lines[m] <= pos {
				lo = m + 1
			} else {
				hi = m
			}
		}
		return lo, pos - s.lines[lo-1] + 1
	}
	// A stream buffer: scan each unit once, whatever the order of the queries, and look the
	// position up in the line starts found so far.
	pos = max(min(pos, s.loaded()), s.base)
	s.nlScanned = max(s.nlScanned, s.base)
	for ; s.nlScanned < pos; s.nlScanned++ {
		p.SpendRead(1)
		var nl bool
		if p.unit == Bytes {
			nl = s.bs[s.nlScanned-s.base] == '\n'
		} else {
			nl = s.in[s.nlScanned-s.base] == '\n'
		}
		if nl {
			s.nlStarts = append(s.nlStarts, s.nlScanned+1)
		}
	}
	// k is the number of line starts at or before pos.
	k, _ := slices.BinarySearch(s.nlStarts, pos+1)
	if k == 0 {
		return s.baseLine, s.baseCol + pos - s.base
	}
	return s.baseLine + k, pos - s.nlStarts[k-1] + 1
}

// NewStreamIterator returns the iterator over the elements pull yields.
//
// NewStreamIterator is for generated parsers; not a stable API.
func NewStreamIterator(pull *StreamPull[*Node]) *StreamIterator { return &StreamIterator{pull} }
