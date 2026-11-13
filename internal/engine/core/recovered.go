package core

import (
	"unsafe"

	"zeroparse.dev/zp/runtime/parse"
)

// The record of recovered errors (Parser.Recovered) is a sequence of entries: errors that recoveries
// made, and immutable lists that the results of calls, growths and Operators Rule attempts share. A
// result keeps its errors as one entry (RecEntry), which the record takes again when the result is
// reused, so that a reuse costs the same however many errors it holds. See addRecovery in
// internal/ir/exec/ops/ops.go and Lemma 14 of spec/proofs/linear-cost.md.

// RecEntry is an entry of the record of recovered errors, or the errors that a result holds: an
// error, a shared list, or, as the zero value, no errors.
type RecEntry struct {
	err  *parse.SyntaxError
	list *ErrList
}

// ErrList is a list of recovered errors: the entries that one recSince took. It never changes once
// made, so results and the record share it.
type ErrList struct {
	entries []RecEntry
	n       int // the errors of the entries, an error counted where it occurs
}

// recEntryBytes is the size of an entry, for the memory limit.
const recEntryBytes = uint64(unsafe.Sizeof(RecEntry{}))

// errListBytes is the size of a list without its entries, for the memory limit.
const errListBytes = uint64(unsafe.Sizeof(ErrList{}))

// ErrEntry returns the entry of the error e.
func ErrEntry(e *parse.SyntaxError) RecEntry { return RecEntry{err: e} }

// None reports whether the entry holds no errors.
func (e RecEntry) None() bool { return e.err == nil && e.list == nil }

// Len returns the number of errors of the entry.
func (e RecEntry) Len() int {
	switch {
	case e.list != nil:
		return e.list.n
	case e.err != nil:
		return 1
	}
	return 0
}

// AppendErrors appends the errors of the entry to dst, in order.
func (e RecEntry) AppendErrors(dst []*parse.SyntaxError) []*parse.SyntaxError {
	return appendRecErrors(dst, []RecEntry{e})
}

// appendRecErrors appends the errors of the entries to dst, in order. It unrolls the lists without
// recursion, since they nest as deep as the steps of a growth go; listing k errors visits at most 2k
// entries (Lemma 14).
func appendRecErrors(dst []*parse.SyntaxError, entries []RecEntry) []*parse.SyntaxError {
	type frame struct {
		entries []RecEntry
		i       int
	}
	var stack []frame
	cur := frame{entries: entries}
	for {
		if cur.i == len(cur.entries) {
			if len(stack) == 0 {
				return dst
			}
			cur, stack = stack[len(stack)-1], stack[:len(stack)-1]
			continue
		}
		x := cur.entries[cur.i]
		cur.i++
		switch {
		case x.list != nil:
			stack = append(stack, cur)
			cur = frame{entries: x.list.entries}
		case x.err != nil:
			dst = append(dst, x.err)
		}
	}
}

// RecoveredErrors returns the recovered errors of the record, in order.
func (p *Parser) RecoveredErrors() []*parse.SyntaxError {
	if len(p.Recovered) == 0 {
		return nil
	}
	return appendRecErrors(make([]*parse.SyntaxError, 0, len(p.Recovered)), p.Recovered)
}

// recSince returns the errors recovered after the mark m (rec.since), charging one unit for each
// entry after it: none, the one entry after it, an error or a list, or a new list of the entries
// after it, which then takes their place in the record. A mark after m is no longer valid.
func (p *Parser) recSince(m int) RecEntry {
	if len(p.Recovered) == m {
		return RecEntry{}
	}
	return p.recSinceSlow(m)
}

func (p *Parser) recSinceSlow(m int) RecEntry {
	if len(p.Recovered)-m == 1 {
		p.SpendValue(1)
		return p.Recovered[m]
	}
	after := p.Recovered[m:]
	l := &ErrList{entries: AppendSpend(p, make([]RecEntry, 0, len(after)), after)}
	for _, e := range after {
		l.n += e.Len()
	}
	p.ChargeValueBytes(int(errListBytes + uint64(cap(l.entries))*recEntryBytes))
	p.compactDiagnosed(m, l)
	p.Recovered = append(p.Recovered[:m], RecEntry{list: l})
	return RecEntry{list: l}
}

// compactDiagnosed keeps the progress of a stream's reports (ReportRecovered) when the entries
// from m become the list l. The reports deliver whole entries, and diagnosedSkip errors of the entry
// at Diagnosed when it is diagnosedList, so a list that takes the place of reported entries counts
// their errors there.
func (p *Parser) compactDiagnosed(m int, l *ErrList) {
	switch {
	case p.Diagnosed > m:
		skip := p.partialDiagnosed()
		skip += countRecErrors(p.Recovered[m:min(p.Diagnosed, len(p.Recovered))])
		p.Diagnosed, p.diagnosedSkip, p.diagnosedList = m, skip, l
	case p.Diagnosed == m && p.partialDiagnosed() > 0:
		p.diagnosedList = l
	}
}

// partialDiagnosed returns the errors of the entry at Diagnosed that a stream has already reported.
func (p *Parser) partialDiagnosed() int {
	if p.diagnosedList == nil || p.Diagnosed >= len(p.Recovered) || p.Recovered[p.Diagnosed].list != p.diagnosedList {
		return 0
	}
	return p.diagnosedSkip
}

// countRecErrors returns the number of errors of the entries.
func countRecErrors(entries []RecEntry) int {
	n := 0
	for _, e := range entries {
		n += e.Len()
	}
	return n
}

// recAdd records the errors of a result again, as one entry, at no cost (rec.add).
func (p *Parser) recAdd(e RecEntry) {
	if !e.None() {
		p.Recovered = append(p.Recovered, e)
	}
}

// PushRecovered records the error e that a recovery made (rec.push).
func (p *Parser) PushRecovered(e *parse.SyntaxError) {
	p.Recovered = append(p.Recovered, RecEntry{err: e})
}

// NewErrList returns a list of the errors errs, each an entry of its own, as rec.since makes one. It
// is exported only as a hook for tests in other packages.
func NewErrList(errs []*parse.SyntaxError) RecEntry {
	l := &ErrList{entries: make([]RecEntry, len(errs)), n: len(errs)}
	for i, e := range errs {
		l.entries[i] = RecEntry{err: e}
	}
	return RecEntry{list: l}
}

// Errors returns the errors of the entry, in order.
func (e RecEntry) Errors() []*parse.SyntaxError { return e.AppendErrors(nil) }

// List returns the list that the entry is, or nil for an error or no errors.
func (e RecEntry) List() *ErrList { return e.list }
