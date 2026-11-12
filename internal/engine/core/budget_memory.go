package core

import (
	"unsafe"

	"zeroparse.dev/zp/internal/limits"
	"zeroparse.dev/zp/internal/value"
)

// Memory is counted in bytes of logical capacity: the element count of a buffer times the size of
// its element type, for the storage the parser takes into use for a parse. It is counted in four
// ways, all of which cost nothing until a finite Memory limit asks for a check:
//
//   - Chunk refills and memo slot growth add to plain counters, Parser.MemUsed and
//     MemoTable.MemBytes, in the branch that takes a chunk or new slots. They are additions to
//     fields, so the inlined allocators stay inlined, and they run on every parse, controlled or
//     not.
//   - Tables that grow an element at a time keep a byte counter beside their data: the context
//     tables, the hash indexes of memo chains, and the expectations of skip-run records.
//   - Storage that only the controlled path allocates is added where it is allocated: values
//     (strings that actions concatenate, text copied out of the input, rebound variable
//     environments, memo keys and error lists) to BudgetControl.values, which a stream returns
//     with its node chunks, and syntax errors to BudgetControl.errs, which a stream returns when
//     its diagnostic tail takes the recovered errors.
//   - Buffers that grow by appending, the decoded input, the line table and the working stacks,
//     are measured when the limit is checked (heldBytes). A parse with a finite limit starts them
//     empty, so their capacities do not depend on an earlier parse.
//
// The check runs at the compute polls of the control and at its explicit checkpoints, so the count
// can pass the limit by what one poll interval allocates. Storage dropped by backtracking is never
// returned; a stream returns what its commits let go of.

// Sizes of the elements that the counters charge.
const (
	nodeBytes     = uint64(unsafe.Sizeof(value.Node{}))
	ptrBytes      = uint64(unsafe.Sizeof((*value.Node)(nil)))
	fieldBytes    = uint64(unsafe.Sizeof(value.NodeField{}))
	frameBytes    = uint64(unsafe.Sizeof(Frame{}))
	memoBytes     = uint64(unsafe.Sizeof(MemoEntry{}))
	memoSlotBytes = uint64(unsafe.Sizeof((*MemoEntry)(nil)))
	anyBytes      = uint64(unsafe.Sizeof(any(nil)))
	intBytes      = uint64(unsafe.Sizeof(0))
	stringBytes   = uint64(unsafe.Sizeof(""))
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
	jumpBytes     = uint64(unsafe.Sizeof(jumpKey{})) + uint64(unsafe.Sizeof(jumpRec{}))
	// mapBytes is the header of a map; an element of a map is counted as its key and value.
	mapBytes = 48
	// memoIndexEntryBytes is an element of a hash index of a memo chain, and of the map of indexes.
	memoIndexEntryBytes = 16
	// envNodeBytes bounds a node of a variable environment that a rebind copies: the node and its
	// values.
	envNodeBytes = uint64(unsafe.Sizeof(Env{})) + envWidth*anyBytes
)

// AttachMemory makes the memory check of c read the counters and buffers of p. The memo table's
// counter is read whole: a pooled table starts at zero (MemoTable.Reset), and a Document's table
// carries the bytes it retains from earlier parses. With a finite limit, p takes no partially used
// chunk or stack from an earlier parse, so the count does not depend on what a pooled parser was
// used for. It is safe on a nil receiver.
func (c *BudgetControl) AttachMemory(p *Parser) {
	if c == nil || c.Limits.Memory == 0 {
		return
	}
	c.memP = p
	p.MemUsed, p.MemKept = 0, 0
	p.NodeSlab, p.PtrSlab, p.fieldSlab, p.FrameSlab = nil, nil, nil, nil
	if p.Scratch != nil {
		p.KidStack = nil
		// The sparse first-call directory keeps its length across pooled parses; start it
		// empty so that its pages are charged as they are taken.
		p.Memo.Pages = nil
	}
}

// checkMemory fails the parse when the memory count exceeds the limit.
func (c *BudgetControl) checkMemory() {
	if used := c.MemoryUsed(); used > c.Limits.Memory {
		panic(Fatal{Err: &limits.LimitError{Kind: limits.LimitMemory, Limit: c.Limits.Memory, Used: used}})
	}
}

// MemoryUsed returns the memory count of the parse, in bytes.
func (c *BudgetControl) MemoryUsed() uint64 {
	used := limits.SaturatingAdd(limits.SaturatingAdd(c.Mem, c.values), c.errs)
	if p := c.memP; p != nil {
		used = limits.SaturatingAdd(used, p.MemUsed+p.MemKept)
		used = limits.SaturatingAdd(used, uint64(max(p.NodeChunks, 0))*NodeChunk*nodeBytes)
		if p.Memo != nil {
			used = limits.SaturatingAdd(used, p.Memo.ChargedBytes())
		}
		used = limits.SaturatingAdd(used, p.heldBytes())
	}
	return used
}

// heldBytes returns the bytes of the storage measured from its state: the decoded or buffered input
// and the table of line starts by their lengths, a stream's read buffer, the working stacks by
// their capacity, the context tables and skip-run records by the counts they keep, and the state
// of missing-token insertion, and the maps of moved nodes and shifts of a Document parse.
func (p *Parser) heldBytes() uint64 {
	n := uint64(len(p.In))*4 + uint64(len(p.Offs))*4 + uint64(len(p.Bs)) + uint64(cap(p.Lines))*intBytes
	if p.Reader != nil {
		n += uint64(p.Reader.Size())
	}
	n += uint64(cap(p.KidStack))*ptrBytes + p.terms.bytes()
	n += uint64(cap(p.Trail))*uint64(unsafe.Sizeof(Undo{})) + uint64(cap(p.Recovered))*recEntryBytes
	n += uint64(cap(p.Created))*ptrBytes + uint64(cap(p.Exp))*expIDBytes
	n += uint64(cap(p.Gamma))*uint64(unsafe.Sizeof(growRec{})) + uint64(cap(p.seedReadLog))*intBytes
	n += uint64(cap(p.frames))*uint64(unsafe.Sizeof(depFrame{})) + uint64(cap(p.deps))*uint64(unsafe.Sizeof(dep{}))
	n += uint64(cap(p.looks))*uint64(unsafe.Sizeof(lookRec{})) + uint64(cap(p.obsGuard))*4
	n += uint64(cap(p.keptSeedResults)) * uint64(unsafe.Sizeof(keptSeedResult{}))
	n += uint64(cap(p.ctxs)) * ptrBytes
	n += uint64(len(p.Moved))*2*ptrBytes + uint64(len(p.Shifts))*(4+intBytes)
	if p.Env != p.initEnv {
		// The environment in use, which a stream keeps after it returns the copies made before.
		n += uint64(p.envLevels+1) * envNodeBytes
	}
	if sl := p.streamLines; sl != nil {
		n += uint64(unsafe.Sizeof(streamLines{})) + uint64(cap(sl.starts))*intBytes
	}
	n += uint64(cap(p.DiagnosticTail.Errors))*ptrBytes + p.DiagnosticTail.bytes
	for _, t := range p.ctxs {
		n += t.bytes
	}
	n += uint64(cap(p.skipTables))*ptrBytes + uint64(cap(p.skips))*uint64(unsafe.Sizeof(skipWalk{}))
	n += p.skipBytes
	for i := range p.skips {
		n += uint64(cap(p.skips[i].visits))*uint64(unsafe.Sizeof(skipVisit{})) + uint64(cap(p.skips[i].vals))*ptrBytes
	}
	for _, s := range p.skipTables {
		if s != nil {
			n += uint64(unsafe.Sizeof(skipSlots{})) + uint64(cap(s.marks))*8 + s.expBytes
			n += uint64(len(s.recs)) * (intBytes + uint64(unsafe.Sizeof(skipRecord{})))
		}
	}
	if m := p.Missing; m != nil {
		n += uint64(cap(m.Path))*uint64(unsafe.Sizeof(MissingTokenEvent{})) + m.seedBytes
		n += uint64(len(m.installed)) * uint64(unsafe.Sizeof(MissingTokenSite{})+1)
		n += uint64(len(m.Seeds)) * (ptrBytes + uint64(unsafe.Sizeof([]MissingTokenEvent{})))
	}
	if p.Backend != nil {
		n += p.Backend.StackBytes()
	}
	return n
}

// ChargeValueBytes charges n bytes of a value that only the controlled path builds (BudgetControl.values).
func (p *Parser) ChargeValueBytes(n int) {
	if BudgetControlsEnabled && p.Control != nil {
		p.Control.chargeValueBytes(n)
	}
}

func (c *BudgetControl) chargeValueBytes(n int) {
	c.values = limits.SaturatingAdd(c.values, uint64(n))
}

// chargeError charges the syntax error e (BudgetControl.errs).
func (p *Parser) chargeError(e *SyntaxError) {
	if BudgetControlsEnabled && p.Control != nil {
		p.Control.errs = limits.SaturatingAdd(p.Control.errs, syntaxErrorSize(e))
	}
}

// syntaxErrorSize returns the bytes of the syntax error e and its lists.
func syntaxErrorSize(e *SyntaxError) uint64 {
	return syntaxErrorBytes + uint64(cap(e.Expected)+cap(e.Messages))*stringBytes
}
