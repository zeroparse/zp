package runtime

import (
	"slices"
	"testing"
)

// TestReportRecoveredOfLists delivers each recovered error to a stream's callback once, in order:
// the errors of a list in the record, and those of a list that took the place of entries already
// delivered (compactDiagnosed), of which only the rest are new.
func TestReportRecoveredOfLists(t *testing.T) {
	var got []int
	p := &Parser{Stream: &streamInput{diagnostic: func(e *SyntaxError) error {
		got = append(got, e.Pos)
		return nil
	}}}
	push := func(pos ...int) {
		for _, n := range pos {
			p.PushRecovered(&SyntaxError{Pos: n})
		}
	}
	report := func(want ...int) {
		t.Helper()
		got = got[:0]
		if err := p.reportRecovered(); err != nil || !slices.Equal(got, want) {
			t.Fatalf("delivered %v (err %v), want %v", got, err, want)
		}
	}
	push(1, 2)
	p.recSince(0) // a call's list of the errors before the first report
	push(3)
	report(1, 2, 3)
	push(4, 5)
	report(4, 5)
	// A call that began before every report ends: its list holds the five errors delivered.
	l := p.recSince(0)
	if l.len() != 5 || len(p.Recovered) != 1 {
		t.Fatalf("list of %d errors, record of %d entries", l.len(), len(p.Recovered))
	}
	report()
	push(6)
	report(6)
	// A list over a partly delivered list and new errors.
	push(7)
	p.recSince(0)
	report(7)
	// Two lists that a call shares, the second recorded again (rec.add).
	push(8, 9)
	k := p.recSince(len(p.Recovered) - 2)
	p.recAdd(k)
	report(8, 9, 8, 9)
	var all []int
	for _, e := range p.recoveredErrors() {
		all = append(all, e.Pos)
	}
	if want := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 8, 9}; !slices.Equal(all, want) {
		t.Fatalf("recovered %v, want %v", all, want)
	}
}
