package runtime

import "unsafe"

// Memory is counted in bytes of logical capacity: the element count of a buffer times the size of
// its element type, for the storage the parser takes into use for a parse. It costs nothing until
// a finite Memory limit asks for a check, and is counted in three ways:
//
//   - Chunk refills add to a plain counter, Parser.memUsed, in the branch that takes a chunk. They
//     are additions to a field, so the inlined allocators stay inlined.
//   - Storage whose growth has a controlled path, the memo slots and first-call sets, the typed
//     memo slots, the strings that actions concatenate and the text copied out of a stream, is
//     charged on that path only: a finite Memory makes the parse controlled.
//   - Storage that already records its use, the memo entry chunks, the capture scratch, the typed
//     arenas and value chunks, the decoded input and the working stacks, is measured when the
//     limit is checked, from its state, lengths and capacities. A parse with a finite limit starts
//     the stacks and chunks it would take from a pooled parser empty, so the count does not depend
//     on what the parser was used for before.
//
// The check runs at the compute polls of the control and at its explicit checkpoints, so the count
// can pass the limit by what one poll interval allocates. Storage dropped by backtracking is never
// returned; a stream returns what its commits let go of.

// Sizes of the elements that the counters charge.
const (
	nodeBytes      = uint64(unsafe.Sizeof(Node{}))
	ptrBytes       = uint64(unsafe.Sizeof((*Node)(nil)))
	fieldBytes     = uint64(unsafe.Sizeof(NodeField{}))
	frameBytes     = uint64(unsafe.Sizeof(CaptureFrame{}))
	memoEntryBytes = uint64(unsafe.Sizeof(memoEntry{}))
	anyBytes       = uint64(unsafe.Sizeof(any(nil)))
	matchBytes     = uint64(unsafe.Sizeof(TMatch{}))
	intBytes       = uint64(unsafe.Sizeof(0))
	stringBytes    = uint64(unsafe.Sizeof(""))
	// syntaxErrorBytes is a syntax error; the lists that hold its pointer count the pointer.
	syntaxErrorBytes = uint64(unsafe.Sizeof(SyntaxError{}))
	expIDBytes       = uint64(unsafe.Sizeof(ExpID(0)))
	sdepBytes        = uint64(unsafe.Sizeof(sdep{}))
	growKeyBytes     = uint64(unsafe.Sizeof(growKey{}))
	runStepBytes     = uint64(unsafe.Sizeof(runStep{}))
	// The storage of a context table: the table and its map of heads, a head (its key, the
	// pointer in the map and the slice it points at), a set, a result slot with its key in the
	// set's map, a canonical seed with its key, and a jump.
	ctxTableBytes = uint64(unsafe.Sizeof(ctxTable{})) + mapBytes
	ctxHeadBytes  = uint64(unsafe.Sizeof(ctxHeadKey{})) + 2*ptrBytes + uint64(unsafe.Sizeof([]*ctxSet{}))
	ctxSetBytes   = ptrBytes + uint64(unsafe.Sizeof(ctxSet{})) + mapBytes
	ctxSlotBytes  = uint64(unsafe.Sizeof(valsKey{})) + ptrBytes + uint64(unsafe.Sizeof(ctxSlot{}))
	canonBytes    = uint64(unsafe.Sizeof(canonKey{})) + uint64(unsafe.Sizeof(canonRec{}))
	// wideKeyBytes is what a key of a set of more than maxCtxDeps dependencies takes beyond a
	// valsKey, besides the bytes of its values.
	wideKeyBytes = uint64(unsafe.Sizeof(wideKey{})) - uint64(unsafe.Sizeof(valsKey{}))
	// internBytes is a number in the map of intern, besides the bytes of its key.
	internBytes = stringBytes + 8
	jumpBytes   = uint64(unsafe.Sizeof(jumpKey{})) + uint64(unsafe.Sizeof(jumpRec{}))
	// mapBytes is the header of a map; an element of a map is counted as its key and value.
	mapBytes = 48
	// memoIndexEntryBytes is an element of a hash index of a memo chain, and of the map of indexes.
	memoIndexEntryBytes = 16
	// envNodeBytes bounds a node of a variable environment that a rebind copies: the node and its
	// values.
	envNodeBytes = uint64(unsafe.Sizeof(Env{})) + envWidth*anyBytes
)

// memoryCounter is implemented by the allocation chunks of the typed values of a generated parser
// (the generated tslabs), whose storage the memory check reads.
type memoryCounter interface{ MemoryBytes() uint64 }

// attachMemory makes the memory check of c read the counters and buffers of p. With a finite
// limit, p takes no partially used chunk or stack from an earlier parse, so the count does not
// depend on what a pooled parser was used for. The memo table is read whole: a pooled table
// starts at zero (release), and a Document's table carries the bytes it retains from earlier
// parses. It is safe on a nil receiver.
func (c *BudgetControl) attachMemory(p *Parser) {
	if c == nil || c.limits.Memory == 0 {
		return
	}
	c.memP = p
	p.memUsed, p.memKept = 0, 0
	p.nodeSlab, p.ptrSlab, p.fieldSlab, p.frameSlab = nil, nil, nil, nil
	p.KidStack, p.Trail, p.saved, p.exp, p.arena = nil, nil, nil, nil, nil
	p.captureScratch.large = 0
	// The first-call tables are charged when the parse makes them; a pooled parser keeps them.
	p.memo.calls, p.memo.pages = nil, nil
}

// attachTypedMemory is attachMemory for a typed parse, whose values come from the arenas of p and
// the chunks of ext.
func (c *BudgetControl) attachTypedMemory(p *TParser, ext any) {
	if c == nil || c.limits.Memory == 0 {
		return
	}
	c.attachMemory(p.Parser)
	c.memT = p
	c.memExt, _ = ext.(memoryCounter)
	p.matches, p.Trail, p.KidStack, p.saved = nil, nil, nil, nil
}

// checkMemory fails the parse when the memory count exceeds the limit.
func (c *BudgetControl) checkMemory() {
	if used := c.memoryUsed(); used > c.limits.Memory {
		c.interrupted = true
		panic(fatal{err: &LimitError{Kind: LimitMemory, Limit: c.limits.Memory, Used: used}})
	}
}

// memoryUsed returns the memory count of the parse, in bytes.
func (c *BudgetControl) memoryUsed() uint64 {
	used := saturatingAdd(saturatingAdd(c.mem, c.values), c.errs)
	if p := c.memP; p != nil {
		used = saturatingAdd(used, saturatingAdd(p.memUsed, p.memKept))
		used = saturatingAdd(used, p.memo.chargedBytes())
		used = saturatingAdd(used, p.captureScratch.bytes())
		used = saturatingAdd(used, p.heldBytes())
	}
	if t := c.memT; t != nil {
		used = saturatingAdd(used, t.heldBytes())
	}
	if c.memExt != nil {
		used = saturatingAdd(used, c.memExt.MemoryBytes())
	}
	return used
}

// chargedBytes returns the memory count of the memo table: the entry chunks in use, whether kept
// from an earlier parse or new, counted from used, which alloc already keeps, and the slots and
// first-call tables it has taken (memBytes), and the hash indexes of long chains (indexBytes).
func (t *memoTable) chargedBytes() uint64 {
	return t.memBytes + t.indexBytes + uint64(t.used)*256*memoEntryBytes
}

// bytes returns the bytes of the capture scratch in use: the chunks of frames and slots the
// allocation state has reached, and the large frames allocated on their own.
func (a *frameScratch) bytes() uint64 {
	if a.frameN == 0 {
		return a.large
	}
	frames := uint64((a.frameN+nodeChunk-1)/nodeChunk) * nodeChunk * frameBytes
	return a.large + frames + uint64(a.slotCI+1)*ptrChunk*ptrBytes
}

// heldBytes returns the bytes of the storage measured from its state: the decoded input, a
// stream's buffers and the tables of line starts by their length, the working stacks by their
// capacity, and the context tables and skip-run records by the counts they keep.
func (p *Parser) heldBytes() uint64 {
	n := uint64(len(p.In))*4 + uint64(len(p.offs))*4 + uint64(len(p.wide))*8 + uint64(cap(p.lines))*intBytes
	if s := p.Stream; s != nil {
		n += uint64(len(s.in))*4 + uint64(len(s.bs)) + uint64(s.reader.Size()) + uint64(cap(s.lines))*8
		n += uint64(cap(s.nlStarts))*intBytes + uint64(cap(s.tail.errors))*ptrBytes + s.tail.bytes
	}
	if p.Env != nil && (p.g == nil || p.Env != p.g.initEnv) {
		// The environment in use, which a stream keeps after it returns the copies made before.
		n += uint64(p.envDepth+1) * envNodeBytes
	}
	n += uint64(cap(p.KidStack)+cap(p.saved)+cap(p.created))*ptrBytes + uint64(cap(p.Recovered))*recEntryBytes
	n += uint64(cap(p.Trail)) * uint64(unsafe.Sizeof(undo{}))
	n += uint64(cap(p.exp)) * expIDBytes
	n += uint64(cap(p.gamma)) * uint64(unsafe.Sizeof(growRec{}))
	n += uint64(cap(p.frames)) * uint64(unsafe.Sizeof(depFrame{}))
	n += uint64(cap(p.deps)+cap(p.popped)) * uint64(unsafe.Sizeof(dep{}))
	n += uint64(cap(p.looks)) * uint64(unsafe.Sizeof(lookRec{}))
	n += uint64(cap(p.obsGuard))*4 + uint64(cap(p.ctxs))*ptrBytes
	for _, t := range p.ctxs {
		n += t.bytes
	}
	n += uint64(cap(p.skipTables))*ptrBytes + uint64(cap(p.skip.visits))*uint64(unsafe.Sizeof(skipVisit{}))
	for _, s := range p.skipTables {
		if s != nil {
			n += uint64(unsafe.Sizeof(skipSlots{})) + uint64(cap(s.marks))*8 + s.expBytes
			n += uint64(len(s.recs)) * (intBytes + uint64(unsafe.Sizeof(skipRecord{})))
		}
	}
	return n
}

// heldBytes returns the bytes of the typed parser's arenas in use and of its stacks.
func (p *TParser) heldBytes() uint64 {
	n := p.nodes.bytes() + p.vals.bytes() + p.frames.bytes() + p.fields.bytes() + p.memoSlab.bytes() + p.memoIndexBytes
	n += uint64(cap(p.KidStack)+cap(p.saved)) * anyBytes
	n += uint64(cap(p.Created)) * uint64(unsafe.Sizeof(TVal(nil)))
	n += uint64(cap(p.Trail)) * uint64(unsafe.Sizeof(tundo{}))
	return n
}

// bytes returns the bytes of the chunks of a in use and of its large allocations.
func (a *tarena[T]) bytes() uint64 {
	chunks := a.ci
	if a.off > 0 {
		chunks++
	}
	return uint64(a.large+chunks*tchunk) * uint64(unsafe.Sizeof(*new(T)))
}

// retainedBytes estimates the bytes a typed Document keeps for later parses: its memo slots and
// the entries their metadata records.
func (d *typedDocumentState) retainedBytes() uint64 {
	return uint64(len(d.slots))*ptrBytes + uint64(len(d.meta))*(uint64(unsafe.Sizeof(tmemoEntry{}))+ptrBytes+uint64(unsafe.Sizeof(typedDocumentMeta{})))
}

// chargeValueBytes charges n bytes of a value that only the controlled path builds
// (BudgetControl.values).
func (p *Parser) chargeValueBytes(n int) {
	if c := p.Control; c != nil {
		c.values = saturatingAdd(c.values, uint64(n))
	}
}

// chargeError charges the syntax error e (BudgetControl.errs).
func (p *Parser) chargeError(e *SyntaxError) {
	if c := p.Control; c != nil {
		c.errs = saturatingAdd(c.errs, syntaxErrorSize(e))
	}
}

// syntaxErrorSize returns the bytes of the syntax error e and its lists.
func syntaxErrorSize(e *SyntaxError) uint64 {
	return syntaxErrorBytes + uint64(cap(e.Expected)+cap(e.Messages))*stringBytes
}

// MemoryBytes returns the bytes of the values s has allocated.
//
// MemoryBytes is for generated parsers; not a stable API.
func (s *Chunk[T]) MemoryBytes() uint64 {
	return uint64(s.allocated) * uint64(unsafe.Sizeof(*new(T)))
}
