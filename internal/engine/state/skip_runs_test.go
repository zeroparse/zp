package state

import (
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/core/noderep"
	"zeroparse.dev/zp/internal/value"
)

// walk runs a walk of site 1 from pos that visits n positions one apart and fails at the next,
// and stops before its end if abort.
func walkSkip(p *Parser, pos, n int, abort bool) {
	p.Pos = pos
	p.SkipBegin(1, false)
	for k := 0; ; k++ {
		if k >= 1 && p.SkipHit() {
			break
		}
		p.SkipVisit()
		if abort && k == n/2 {
			return // the budget stops the parse in the iteration
		}
		if k == n {
			break // the iteration fails at its start
		}
		p.Pos++
	}
	p.SkipEnd()
}

// TestSkipRunsAbortStoresNothing checks that records are stored only when a walk ends: a recording
// walk stopped in an iteration leaves its positions as an earlier walk left them, and a plain walk
// only marks those of its matching iterations.
func TestSkipRunsAbortStoresNothing(t *testing.T) {
	p := &Parser{}
	walkSkip(p, 0, 0, false) // plain, fails at once: touches no slot
	if len(p.skipTables) != 0 {
		t.Fatalf("a walk that fails at once makes slots: %v", p.skipTables)
	}
	plain := func(pos int) uint8 {
		if pos < 10 {
			return skipVisited
		}
		return skipFresh // the iteration at 10 failed
	}
	walkSkip(p, 0, 10, false) // plain: marks 0..9
	s := p.skipTables[2]
	for pos := 0; pos <= 10; pos++ {
		if got := s.get(pos, nil).state; got != plain(pos) {
			t.Fatalf("after a plain walk, position %d has state %d", pos, got)
		}
	}
	walkSkip(p, 2, 8, true) // recording from 2, stopped at 6
	for pos := 0; pos <= 10; pos++ {
		if got := s.get(pos, nil).state; got != plain(pos) {
			t.Fatalf("after a stopped recording walk, position %d has state %d", pos, got)
		}
	}
	p.skips, p.skipN = nil, 0 // the stopped parse is gone; a later walk starts clean
	walkSkip(p, 3, 7, false)
	for pos := 0; pos <= 10; pos++ {
		want := uint8(skipVisited)
		if pos >= 3 {
			want = skipRecorded
		}
		if got := s.get(pos, nil).state; got != want {
			t.Fatalf("after a recording walk from 3, position %d has state %d, want %d", pos, got, want)
		}
		if want == skipRecorded && s.get(pos, nil).end != 10 {
			t.Fatalf("position %d records end %d, want 10", pos, s.get(pos, nil).end)
		}
	}
}

// TestSkipRunsAbortControl checks that the test above detects records stored when their iteration
// ends: a stopped walk then leaves records behind.
func TestSkipRunsAbortControl(t *testing.T) {
	SkipRecordMutation = SkipEarlyStore
	defer func() { SkipRecordMutation = 0 }()
	p := &Parser{}
	walkSkip(p, 0, 10, false)
	walkSkip(p, 2, 8, true)
	s := p.skipTables[2]
	recorded := 0
	for pos := 0; pos <= 10; pos++ {
		if s.get(pos, nil).state == skipRecorded {
			recorded++
		}
	}
	if recorded == 0 {
		t.Fatal("a stopped walk that stores records early leaves none")
	}
}

// texts returns the texts of the nodes of the elements es.
func texts(es []*core.Node) string {
	out := ""
	for _, e := range es {
		out += value.D(e).Text
	}
	return out
}

// TestSkipRunsShareElements checks that placing the elements of a walk before a record's suffix
// leaves what every record holds as it was: in place only before the front of a buffer, and
// otherwise in a copy.
func TestSkipRunsShareElements(t *testing.T) {
	p := &Parser{}
	el := func(s string) []*core.Node {
		var ns []*core.Node
		for _, c := range s {
			ns = append(ns, p.NewNode(value.Data{Kind: noderep.MatchKind, Text: string(c)}))
		}
		return ns
	}
	b0, lo0 := p.placeVals(el("abc"), valSuffix{})
	whole, mid := valSuffix{b0, lo0}, valSuffix{b0, lo0 + 1}
	if got := texts(whole.slice()); got != "abc" {
		t.Fatalf("a walk without a record holds %q", got)
	}
	// Elements before a suffix in the middle of a buffer are copied: the buffer holds the rest of
	// another walk.
	b1, lo1 := p.placeVals(el("x"), mid)
	if b1 == b0 {
		t.Fatalf("elements are written before a suffix that is not at the front of its buffer")
	}
	if got := texts(valSuffix{b1, lo1}.slice()); got != "xbc" {
		t.Fatalf("the copy holds %q, want xbc", got)
	}
	if got := texts(whole.slice()) + texts(mid.slice()); got != "abc"+"bc" {
		t.Fatalf("a copy changed the records of the buffer: %q", got)
	}
	// The copy has room before its front, which a walk that reaches the front fills in place
	// once; a second walk to the same suffix copies.
	front := valSuffix{b1, lo1}
	b2, lo2 := p.placeVals(el("y"), front)
	if b2 != b1 || texts(valSuffix{b2, lo2}.slice()) != "yxbc" {
		t.Fatalf("a walk reaching the front of a buffer does not use its room: %q", texts(valSuffix{b2, lo2}.slice()))
	}
	b3, lo3 := p.placeVals(el("z"), front)
	if b3 == b1 || texts(valSuffix{b3, lo3}.slice()) != "zxbc" {
		t.Fatalf("a second walk to the same suffix shares the room: %q", texts(valSuffix{b3, lo3}.slice()))
	}
	if got := texts(front.slice()) + texts(valSuffix{b2, lo2}.slice()); got != "xbc"+"yxbc" {
		t.Fatalf("placing elements changed an earlier suffix: %q", got)
	}
	// No elements of its own: the walk takes the suffix as it is.
	if b, lo := p.placeVals(nil, mid); b != b0 || lo != lo0+1 {
		t.Fatalf("a walk without elements of its own copies its tail")
	}
}

// TestRunSiteTurnsHotOnRestart checks that a run site keeps only the furthest position of its walks
// until a walk begins below it and walks RunHotSpan of the input below it, and then turns hot, for
// the lookahead state of the site separately. Walks that follow each other, walks apart and walks
// that go back less than RunHotSpan do not turn it hot, however they alternate.
func TestRunSiteTurnsHotOnRestart(t *testing.T) {
	walked := func(p *Parser, site, start, end int) {
		t.Helper()
		if p.RunHot(site) {
			t.Fatalf("site %d is hot before the walk from %d", site, start)
		}
		p.Pos = end
		p.RunWalked(site, start)
	}
	p := &Parser{}
	walked(p, 1, 10, 12) // an inner walk
	walked(p, 1, 4, 20)  // goes back 8 below 12
	walked(p, 1, 20, 30) // begins where the last ended
	walked(p, 1, 40, 40) // empty, apart
	walked(p, 1, 50, 60)
	if p.RunHot(1) || len(p.skipTables) != 0 || len(p.skips) != 0 {
		t.Fatalf("walks that do not go back keep state: hot %v, tables %d", p.RunHot(1), len(p.skipTables))
	}
	p.Silent++
	walked(p, 1, 51, 55) // another state of the site has its own position
	p.Silent--
	walked(p, 1, 55, 70) // goes back 5 below 60
	walked(p, 1, 10, 69) // goes back 59 below 70: less than RunHotSpan
	// Alternating walks, each of which ends where the next begins: [0,e) then [e,E), then [1,e) and
	// so on. A walk that begins below the furthest end and walks RunHotSpan below it is hot.
	q := &Parser{}
	e, far := 100, 200
	walked(q, 3, 0, e)
	walked(q, 3, e, far)
	q.Pos = e
	q.RunWalked(3, 1) // walks e-1 below far
	if !q.RunHot(3) {
		t.Fatal("a walk back over a span of an earlier walk did not turn the site hot")
	}
	if q.Silent = 1; q.RunHot(3) {
		t.Fatal("the site turned hot in the lookahead state too")
	}
	p.Silent = 0
	p.Missing = &MissingTokenState{}
	walked(p, 2, 0, 200)
	p.Pos = 300
	p.RunWalked(2, 100)
	if p.RunHot(2) {
		t.Fatal("a site turned hot with missing-token insertion")
	}
}
