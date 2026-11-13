package runtime

import (
	"fmt"
	"unicode/utf8"

	"zeroparse.dev/zp/core"
)

// Document incrementally parses the configured start rule. It belongs to one
// goroutine; returned nodes may be shared with later parses.
//
// Of its fields and methods, BeginParse, EndParse, ResolveParseOptions, State are for generated parsers; not a stable API.
type Document struct {
	State     *documentState
	defaults  parseOptions
	typedMemo interface{ splice(ed documentEdit) }
	// parsing is set while a parse runs, whose observation callbacks must not
	// parse or edit the Document again (BeginParse).
	parsing bool
}

// DocumentBusy returned by a Parse, ParseValue or Edit of a Document that is
// already parsing. Only a trace callback of that parse can make one; parsers
// generated with observability export it as ErrDocumentBusy.
//
// DocumentBusy is for generated parsers; not a stable API.
var DocumentBusy = fmt.Errorf("document is already parsing")

func (d *Document) BeginParse() error {
	if d.parsing {
		return DocumentBusy
	}
	d.parsing = true
	return nil
}

func (d *Document) EndParse() { d.parsing = false }

// MakeDocument is for generated parsers; not a stable API.
func MakeDocument(root *Rule, text string, opts []ParseOption) (d *Document, err error) {
	o, err := ResolveOptions(root.g, opts)
	if err != nil {
		return nil, err
	}
	defer recoverParse(&d, &err)
	o = prepareText(o, text).forDocument()
	// Construction performs the same input decoding and budget checks as a parse,
	// but its counters do not carry into subsequent invocations.
	p := &Parser{unit: o.unit, Control: o.control}
	p.setGrammar(root.g)
	p.Expectations = o.expectations
	o.control.attachMemory(p)
	if o.unit == core.CodePoints {
		p.setSource(text)
	} else if o.control != nil {
		p.SpendRead(uint64(len(text)))
	}
	o.control.Check()
	o.control, o.prepared = nil, false
	state := newDocumentState(root, text, o.unit)
	state.expectations = o.expectations
	return &Document{State: state, defaults: o}, nil
}

// Text returns the current exact input, including invalid UTF-8 bytes.
func (d *Document) Text() string { return d.State.text }

// Edit replaces [start,end) in the Document's position unit.
func (d *Document) Edit(start, end int, text string) error {
	if d.parsing {
		return DocumentBusy
	}
	ed, err := d.State.editReuse(start, end, text)
	if err != nil {
		return err
	}
	if d.typedMemo != nil {
		if d.State.pending > pendingEditLimit {
			d.typedMemo = nil
		} else {
			d.typedMemo.splice(ed)
		}
	}
	return nil
}

// Parse parses the current text, applying invocation options after the
// Document's construction defaults. An invocation cannot change its unit.
func (d *Document) Parse(opts ...ParseOption) (*core.Node, error) {
	if err := d.BeginParse(); err != nil {
		return nil, err
	}
	defer d.EndParse()
	o, err := d.ResolveParseOptions(opts)
	if err != nil {
		return nil, err
	}
	return d.State.parse(o)
}

func (d *Document) ResolveParseOptions(opts []ParseOption) (parseOptions, error) {
	o := d.defaults
	for _, opt := range opts {
		opt(&o)
	}
	depth, err := resolveDepth(o.limits.Depth, d.State.root.g.maxDepth)
	if err != nil {
		return o, err
	}
	o.maxDepth = depth
	if d.State.root.g.NoBudgetControls && (o.context != nil || !limitsDisabled(o.limits)) {
		return o, fmt.Errorf("this parser was generated without limit controls (generate with -limits to use WithContext and a finite Compute or Memory limit)")
	}
	if err := o.validateDiagnostics(); err != nil {
		return o, err
	}
	if err := o.validateObservation(d.State.root.g); err != nil {
		return o, err
	}
	return o, nil
}

// Stats returns the evaluation and reuse counts of the last Parse. A generated Document reuses
// memo results only, never repetition elements, so every reuse is a memo hit.
func (d *Document) Stats() core.ParseStats {
	s := d.State.stats
	return core.ParseStats{Evaluated: s.Evaluated, Reused: s.Reused, MemoHits: s.Reused}
}

// A slice's first element, length and capacity identify an exact slice view.
// The element pointer keeps the backing array visible to the collector while
// cloning. Empty slices without capacity have no elements or mutable backing
// array to share.
//
// DocumentSliceKey is for generated parsers; not a stable API.
type DocumentSliceKey struct {
	First    any
	Length   int
	Capacity int
}

// newDocumentState parses only the configured start rule.
func newDocumentState(root *Rule, text string, unit core.PositionUnit) *documentState {
	return &documentState{root: root, text: text, unit: unit}
}

func (d *documentState) edit(start, end int, replacement string) error {
	_, err := d.editReuse(start, end, replacement)
	return err
}

// documentEdit is an edit as the memo sees it (memo.splice): the input [start, end) is replaced
// with input delta units longer, and the input after end decodes as before.
type documentEdit struct{ start, end, delta int }

// editReuse applies an edit to the text and the memo (memo.splice) and returns it as the memo sees
// it: from before start when the edit can change how the input before start decodes, to after end
// when it can change how the input after end decodes.
func (d *documentState) editReuse(start, end int, replacement string) (ed documentEdit, err error) {
	length := len(d.text)
	if d.unit != core.Bytes {
		length = utf8.RuneCountInString(d.text)
	}
	if start < 0 || end < start || end > length {
		return documentEdit{}, fmt.Errorf("invalid range [%d,%d) for text of length %d", start, end, length)
	}
	startByte, endByte := start, end
	if d.unit == core.Bytes {
		if err := documentByteBoundary(d.text, start); err != nil {
			return documentEdit{}, err
		}
		if err := documentByteBoundary(d.text, end); err != nil {
			return documentEdit{}, err
		}
	} else {
		startByte = documentByteOffset(d.text, start)
		endByte = documentByteOffset(d.text, end)
	}
	// The replacement can complete an incomplete UTF-8 sequence before start,
	// turning invalid bytes, each a separate character, into one character.
	// Such a sequence has at most UTFMax-1 bytes, each one position in either
	// unit, so results that examined them are not retained.
	reuse := start
	for i := 0; i < utf8.UTFMax-1 && i < startByte; i++ {
		if r, size := utf8.DecodeLastRuneInString(d.text[:startByte-i]); r != utf8.RuneError || size != 1 {
			break
		}
		reuse--
	}
	ed = documentEdit{start: reuse, end: end, delta: len(replacement) - (endByte - startByte)}
	if d.unit != core.Bytes {
		// The replacement can also end with the start of a sequence that the continuation bytes after
		// end, each a separate character, complete. Past those, the characters are the same.
		endX := endByte
		for endX < len(d.text) && endX-endByte < utf8.UTFMax-1 && !utf8.RuneStart(d.text[endX]) {
			endX++
		}
		ed.end = end + endX - endByte
		reuseByte := startByte - (start - reuse)
		window := d.text[reuseByte:startByte] + replacement + d.text[endByte:endX]
		ed.delta = utf8.RuneCountInString(window) - (ed.end - reuse)
	}

	// Build the replacement before changing either text or memo. Invalid UTF-8
	// bytes are preserved, and a replacement may complete an existing prefix.
	d.text = d.text[:startByte] + replacement + d.text[endByte:]
	d.pending++
	d.epoch++
	if d.pending > pendingEditLimit {
		// The edits that the memo would keep up with cost more than parsing again: the next parse
		// starts without the memo, as a parse from scratch does.
		d.memo = memoTable{}
	} else {
		d.memo.epoch = d.epoch
		d.memo.splice(ed)
	}
	return ed, nil
}

// examinedFrom is where the input that a Document call examined starts (memoEntry.from), or
// noRecord for an entry without a record of memo.examine. An entry that examined no input records
// the empty range at its position, which an entry at position 0 tells from no record by from.
type examinedFrom uint8

const (
	noRecord examinedFrom = iota
	fromPos
	fromBefore
)

// examinedFromOf returns the start of a record of memo.examine that starts before the position of
// its entry if before.
func examinedFromOf(before bool) examinedFrom {
	if before {
		return fromBefore
	}
	return fromPos
}

// splice applies an edit to the memo (memo.splice), copying the entries it keeps into a new table:
// those that examined only input before the edit stay; those that examined only input after it,
// with no recovered errors and not pinned, move by its delta, with what their values must move
// recorded in shifts (documentMove); the others are removed, with those that more than pendingEditLimit
// edits have passed since a parse stored them or found them to answer a lookup (memoTable.epoch against
// memoEntry.seen).
func (t *memoTable) splice(ed documentEdit) {
	epoch := t.epoch
	var kept memoTable
	for pos := range t.slots {
		for e := t.slots[pos]; e != nil; e = e.next {
			to, from := pos, pos
			if e.from == fromBefore {
				from--
			}
			switch {
			case e.from == noRecord:
				continue
			case epoch-e.seen > pendingEditLimit:
				// More edits behind than a lookup applies to an entry: it is a miss from now on, and
				// the entries of earlier texts that a Document retains stay bounded by that many edits.
				continue
			case e.examined <= ed.start:
			case from >= ed.end && !e.pinned && e.errs.none():
				to = pos + ed.delta
			default:
				continue
			}
			c := kept.alloc()
			*c = *e
			c.next = nil
			shift := t.shifts[e]
			if to != pos {
				c.end += ed.delta
				c.examined += ed.delta
				c.far += ed.delta
				shift += ed.delta
			}
			// A result without a node holds no positions.
			if shift != 0 && c.node != nil {
				if kept.shifts == nil {
					kept.shifts = map[*memoEntry]int{}
				}
				kept.shifts[c] = shift
			}
			kept.put(int(e.rule), to, int(e.min), e.env, c)
		}
	}
	*t = kept
}

// moveMarkLimit is the number of moves after which a parse starts without the memo of the Document,
// and numbers the moves from 1 again. A parse moves each entry of the memo at most once, so it makes
// far fewer moves than the numbers that remain.
const moveMarkLimit = 1 << 31

// documentMove moves the nodes of the value of e by the shift that edits gave e (memo.splice), in
// copies, since the value may be part of a result already returned, and reports whether it could:
// a value that holds a host value other than a node or a basic value is not moved. The check and the
// copy each cost, for every distinct node of the value, a unit and one more for each of its children
// and fields.
func (p *Parser) documentMove(e *memoEntry) bool {
	if len(p.memo.shifts) == 0 {
		return true
	}
	shift, ok := p.memo.shifts[e]
	if !ok {
		return true
	}
	// The check marks the nodes it visits with the number of the move, which no earlier move of the
	// Document had, so that a mark that an earlier move left, in this parse or an earlier one, whether
	// that move went on or refused, never reads as a visit of this one. A node that the check finds
	// marked occurs in the value more than once; such a value is moved through a map from the nodes to
	// their copies, so that each is copied once and the copies are shared as the nodes were.
	// Moving the value is work of the Document that a parse from scratch does not do.
	defer p.Control.FreeEnd(p.Control.FreeBegin(true))
	d := p.Document
	d.moves++
	movable, shared := p.movable(e.node, d.moves)
	if !movable {
		return false
	}
	if shared {
		if p.moved == nil {
			p.moved = map[*core.Node]*core.Node{}
		}
		e.node = p.moveShared(e.node, int32(shift))
		clear(p.moved)
	} else {
		e.node = p.moveNode(e.node, int32(shift))
	}
	delete(p.memo.shifts, e)
	return true
}

// movable reports whether moveNode can move n: its fields and box hold only nodes and basic values;
// and whether a node occurs in n more than once. It marks the nodes it visits with mark.
func (p *Parser) movable(n *core.Node, mark uint32) (movable, shared bool) {
	if n == nil {
		return true, false
	}
	if D(n).Mark == mark {
		return true, true
	}
	D(n).Mark = mark
	p.spendValue(1 + len(D(n).Children) + len(D(n).Fields))
	if !basicValue(D(n).Box) {
		return false, false
	}
	for _, c := range D(n).Children {
		ok, sh := p.movable(c, mark)
		if !ok {
			return false, false
		}
		shared = shared || sh
	}
	for _, f := range D(n).Fields {
		if c, ok := f.Value.(*core.Node); ok {
			ok, sh := p.movable(c, mark)
			if !ok {
				return false, false
			}
			shared = shared || sh
		} else if !basicValue(f.Value) {
			return false, false
		}
	}
	return true, shared
}

// basicValue reports whether v holds no positions: nil, a boolean, a number or a string.
func basicValue(v any) bool {
	switch v.(type) {
	case nil, bool, string, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, uintptr,
		float32, float64, complex64, complex128:
		return true
	}
	return false
}

// moveNode returns a copy of the movable node n, in which no node occurs twice, with its positions
// and those of its descendants moved by shift.
func (p *Parser) moveNode(n *core.Node, shift int32) *core.Node {
	if n == nil {
		return nil
	}
	c := p.NewNode(*D(n))
	p.spendValue(len(D(n).Children) + len(D(n).Fields))
	D(c).Start += shift
	D(c).End += shift
	if D(n).Children != nil {
		D(c).Children = p.Nodes(len(D(n).Children))
		for i, k := range D(n).Children {
			D(c).Children[i] = p.moveNode(k, shift)
		}
	}
	if D(n).Fields != nil {
		D(c).Fields = p.fields(len(D(n).Fields))
		for _, f := range D(n).Fields {
			if k, ok := f.Value.(*core.Node); ok {
				f.Value = p.moveNode(k, shift)
			}
			D(c).Fields = append(D(c).Fields, f)
		}
	}
	return c
}

// moveShared is moveNode for a value that holds a node more than once: moved holds the copies made
// so far, which it shares.
func (p *Parser) moveShared(n *core.Node, shift int32) *core.Node {
	if n == nil {
		return nil
	}
	if c := p.moved[n]; c != nil {
		return c
	}
	c := p.NewNode(*D(n))
	p.moved[n] = c
	p.spendValue(len(D(n).Children) + len(D(n).Fields))
	D(c).Start += shift
	D(c).End += shift
	if D(n).Children != nil {
		D(c).Children = p.Nodes(len(D(n).Children))
		for i, k := range D(n).Children {
			D(c).Children[i] = p.moveShared(k, shift)
		}
	}
	if D(n).Fields != nil {
		D(c).Fields = p.fields(len(D(n).Fields))
		for _, f := range D(n).Fields {
			if k, ok := f.Value.(*core.Node); ok {
				f.Value = p.moveShared(k, shift)
			}
			D(c).Fields = append(D(c).Fields, f)
		}
	}
	return c
}

func documentByteOffset(text string, index int) int {
	if index == 0 {
		return 0
	}
	count := 0
	for off := range text {
		if count == index {
			return off
		}
		count++
	}
	return len(text)
}

func documentByteBoundary(text string, pos int) error {
	if pos < len(text) && !utf8.RuneStart(text[pos]) {
		for start := pos - 1; start >= max(0, pos-utf8.UTFMax+1); start-- {
			if utf8.RuneStart(text[start]) {
				_, size := utf8.DecodeRuneInString(text[start:])
				if start+size > pos {
					return fmt.Errorf("position %d is not at a character boundary", pos)
				}
				break
			}
		}
	}
	return nil
}

// parse retains completed memo entries after ordinary syntax failures. A
// panic or budget interruption discards all entries because they may depend on
// unfinished left-recursion growth or partially evaluated actions.
func (d *documentState) parse(o parseOptions) (*core.Node, error) {
	return DocumentParseConverted(d, o, func(n *core.Node, _ *BudgetControl, _ error) *core.Node { return n })
}

// DocumentParseConverted keeps Node parsing and final typed conversion inside
// the same limit-controlled invocation and memo cleanup boundary.
//
// DocumentParseConverted is for generated parsers; not a stable API.
func DocumentParseConverted[T any](d *documentState, o parseOptions, convert func(*core.Node, *BudgetControl, error) T) (res T, err error) {
	defer recoverParse(&res, &err)
	if o.unit != d.unit {
		return res, fmt.Errorf("document parse cannot change unit")
	}
	if o.expectations != d.expectations {
		return res, fmt.Errorf("document parse cannot change expectations")
	}
	if d.root == nil {
		return res, fmt.Errorf("document start rule is missing")
	}
	o = prepareText(o, d.text).forDocument()
	if o.control != nil && o.control.limits.Memory != 0 && d.memo.chargedBytes() > o.control.limits.Memory {
		// The retained memo alone exceeds the limit: the parse starts without it.
		d.memo = memoTable{}
	}
	p := &Parser{unit: d.unit, MaxDepth: o.maxDepth, Control: o.control}
	p.setGrammar(d.root.g)
	p.Expectations = o.expectations
	p.Observation = newObservation(&o.observe)
	defer p.releaseObservation()
	if d.unit == core.Bytes {
		p.bs, p.n = d.text, len(d.text)
	} else {
		p.setSource(d.text)
	}
	if d.moves > moveMarkLimit {
		// The marks that the nodes of the memo carry would soon repeat: start without it.
		d.memo, d.moves = memoTable{}, 0
	}
	p.Document, p.memo = d, d.memo
	d.pending = 0
	o.control.attachMemory(p)
	d.stats = documentStats{}
	defer func() {
		if x := recover(); x != nil {
			d.memo = memoTable{}
			panic(x)
		}
		d.memo = p.memo
	}()
	p.Control.Check()
	v, ok := p.Call(d.root, 0)
	p.Control.Check()
	var n *core.Node
	if ok && p.Pos == p.n {
		n = v
		if len(p.Recovered) > 0 {
			err = core.SyntaxErrors(p.recoveredErrors())
		}
	} else {
		if ok {
			p.Expect(p.Pos, IdEndInput)
		}
		err = p.MakeError(p.farthest, p.exp[p.expBase:])
	}
	err = publishDocumentError(p, err)
	res = convert(n, o.control, err)
	p.Control.Check()
	return res, err
}

// publishDocumentError separates caller-owned diagnostics from private memo
// entries. Strings are immutable, but the error objects and slice containers
// must not be shared with a later invocation.
func publishDocumentError(p *Parser, err error) error {
	// The copy is work that only a Document parse does.
	defer p.Control.FreeEnd(p.Control.FreeBegin(true))
	switch e := err.(type) {
	case core.SyntaxErrors:
		if e == nil {
			return e
		}
		p.spendValue(len(e))
		out := make(core.SyntaxErrors, len(e))
		for i, item := range e {
			out[i] = cloneDocumentError(p, item)
		}
		return out
	}
	// Ordinary failures are freshly assembled by MakeError, not memoized.
	return err
}

func cloneDocumentError(p *Parser, e *core.SyntaxError) *core.SyntaxError {
	p.Spend()
	if e == nil {
		return nil
	}
	out := new(core.SyntaxError)
	*out = *e
	if e.Expected != nil {
		p.spendValue(len(e.Expected))
		out.Expected = make([]string, len(e.Expected))
		copySpend(p, out.Expected, e.Expected)
	}
	if e.Messages != nil {
		p.spendValue(len(e.Messages))
		out.Messages = make([]string, len(e.Messages))
		copySpend(p, out.Messages, e.Messages)
	}
	return out
}
