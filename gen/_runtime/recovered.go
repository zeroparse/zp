package runtime

import "unsafe"

// The record of recovered errors (Parser.Recovered) is a sequence of entries: errors that recoveries
// made, and immutable lists that the results of calls, growths and Operators Rule attempts share. A
// result keeps its errors as one entry (recEntry), which the record takes again when the result is
// reused, so that a reuse costs the same however many errors it holds. See addRecovery in
// internal/ir/exec/ops/ops.go and Lemma 14 of spec/proofs/linear-cost.md.

// recEntry is an entry of the record of recovered errors, or the errors that a result holds: an
// error, a shared list, or, as the zero value, no errors.
type recEntry struct {
	err  *SyntaxError
	list *errList
}

// errList is a list of recovered errors: the entries that one recSince took. It never changes once
// made, so results and the record share it.
type errList struct {
	entries []recEntry
	n       int // the errors of the entries, an error counted where it occurs
}

// recEntryBytes is the size of an entry, for the memory limit.
const recEntryBytes = uint64(unsafe.Sizeof(recEntry{}))

// errListBytes is the size of a list without its entries, for the memory limit.
const errListBytes = uint64(unsafe.Sizeof(errList{}))

// none reports whether the entry holds no errors.
func (e recEntry) none() bool { return e.err == nil && e.list == nil }

// len returns the number of errors of the entry.
func (e recEntry) len() int {
	switch {
	case e.list != nil:
		return e.list.n
	case e.err != nil:
		return 1
	}
	return 0
}

// appendRecErrors appends the errors of the entries to dst, in order. It unrolls the lists without
// recursion, since they nest as deep as the steps of a growth go; listing k errors visits at most 2k
// entries (Lemma 14).
func appendRecErrors(dst []*SyntaxError, entries []recEntry) []*SyntaxError {
	type frame struct {
		entries []recEntry
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

// recoveredErrors returns the recovered errors of the record, in order.
func (p *Parser) recoveredErrors() []*SyntaxError {
	if len(p.Recovered) == 0 {
		return nil
	}
	return appendRecErrors(make([]*SyntaxError, 0, len(p.Recovered)), p.Recovered)
}

// recSince returns the errors recovered after the mark m (rec.since), charging one unit for each
// entry after it: none, the one entry after it, an error or a list, or a new list of the entries
// after it, which then takes their place in the record. A mark after m is no longer valid.
func (p *Parser) recSince(m int) recEntry {
	if len(p.Recovered) == m {
		return recEntry{}
	}
	return p.recSinceSlow(m)
}

func (p *Parser) recSinceSlow(m int) recEntry {
	if len(p.Recovered)-m == 1 {
		p.spendValue(1)
		return p.Recovered[m]
	}
	after := p.Recovered[m:]
	l := &errList{entries: appendSpend(p, make([]recEntry, 0, len(after)), after)}
	for _, e := range after {
		l.n += e.len()
	}
	p.chargeValueBytes(int(errListBytes + uint64(cap(l.entries))*recEntryBytes))
	if p.Stream != nil {
		p.Stream.compactDiagnosed(p, m, l)
	}
	p.Recovered = append(p.Recovered[:m], recEntry{list: l})
	return recEntry{list: l}
}

// recAdd records the errors of a result again, as one entry, at no cost (rec.add).
func (p *Parser) recAdd(e recEntry) {
	if !e.none() {
		p.Recovered = append(p.Recovered, e)
	}
}

// PushRecovered records the error e that a recovery made (rec.push). Generated parsers call it.
func (p *Parser) PushRecovered(e *SyntaxError) {
	p.Recovered = append(p.Recovered, recEntry{err: e})
}

// compactDiagnosed keeps the progress of the stream's reports (reportRecovered) when the entries of
// the record from m become the list l. The reports deliver whole entries, and diagnosedSkip errors
// of the entry at diagnosed when it is diagnosedList, so a list that takes the place of reported
// entries counts their errors there.
func (s *streamInput) compactDiagnosed(p *Parser, m int, l *errList) {
	switch {
	case s.diagnosed > m:
		skip := s.partialDiagnosed(p)
		for _, e := range p.Recovered[m:min(s.diagnosed, len(p.Recovered))] {
			skip += e.len()
		}
		s.diagnosed, s.diagnosedSkip, s.diagnosedList = m, skip, l
	case s.diagnosed == m && s.partialDiagnosed(p) > 0:
		s.diagnosedList = l
	}
}

// partialDiagnosed returns the errors of the entry at diagnosed that the stream has already
// reported.
func (s *streamInput) partialDiagnosed(p *Parser) int {
	if s.diagnosedList == nil || s.diagnosed >= len(p.Recovered) || p.Recovered[s.diagnosed].list != s.diagnosedList {
		return 0
	}
	return s.diagnosedSkip
}
