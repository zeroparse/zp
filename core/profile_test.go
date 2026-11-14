package core

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func enter(rule string, depth, pos int) TraceEvent {
	return TraceEvent{Kind: TraceEnter, Rule: rule, Depth: depth, Pos: pos}
}

func exit(rule string, depth, pos, end int, matched bool, evals, examined int) TraceEvent {
	return TraceEvent{Kind: TraceExit, Rule: rule, Depth: depth, Pos: pos, End: end, Matched: matched,
		Memo: evals == 0, Evals: evals, Examined: examined}
}

// TestProfileRecorderCounts replays a parse: s calls a twice at 0 (the second time from the
// memo), b fails after examining to 5, and a skipped call of c.
func TestProfileRecorderCounts(t *testing.T) {
	var pr Profile
	r := pr.Recorder()
	for _, e := range []TraceEvent{
		enter("s", 1, 0),
		enter("b", 2, 0), exit("b", 2, 0, 0, false, 1, 5),
		enter("a", 2, 0), exit("a", 2, 0, 2, true, 1, 3),
		enter("a", 2, 0), exit("a", 2, 0, 2, true, 0, 3),
		{Kind: TraceSkipped, Rule: "c", Depth: 2, Pos: 2, SkipReason: SkipFirstCharacter},
		exit("s", 1, 0, 2, true, 1, 5),
	} {
		r.Record(e)
		r.Resume()
	}
	r.Close()
	s := pr.Snapshot()
	if s.Parses != 1 || s.Entered != 4 || s.Calls != 4 || s.Evals != 3 || s.MemoHits != 1 || s.Skipped != 1 || s.Examined != 5 {
		t.Fatalf("totals %+v", s)
	}
	var names []string
	for _, row := range s.Rules {
		names = append(names, row.Rule)
	}
	if !reflect.DeepEqual(names, []string{"a", "b", "c", "s"}) {
		t.Fatalf("rules %v, want sorted by name", names)
	}
	a, b, c := s.Rules[0], s.Rules[1], s.Rules[2]
	if a.Calls != 2 || a.MemoHits != 1 || a.Consumed != 4 || a.Examined != 6 || a.Repeats != 0 || a.MaxEvals != 1 {
		t.Errorf("a %+v", a)
	}
	if b.Failed != 1 || b.Wasted != 5 || b.LongestFail != 5 || b.LongestFailAt.Pos != 0 {
		t.Errorf("b %+v", b)
	}
	if c.Skipped != 1 || c.Entered != 0 || !reflect.DeepEqual(c.SkipReasons, []SkipReasonCount{{SkipFirstCharacter, 1}}) {
		t.Errorf("c %+v", c)
	}
}

// TestProfileRecorderInterrupted checks that a call without an exit counts in Entered only, and that
// a recorder of a parse within a callback is independent.
func TestProfileRecorderInterrupted(t *testing.T) {
	var pr Profile
	outer := pr.Recorder()
	outer.Record(enter("s", 1, 0))
	inner := pr.Recorder()
	inner.Record(enter("s", 1, 0))
	inner.Record(exit("s", 1, 0, 1, true, 1, 1))
	inner.Close()
	outer.Record(enter("x", 2, 0))
	outer.Close()
	s := pr.Snapshot()
	if s.Parses != 2 || s.Entered != 3 || s.Calls != 1 {
		t.Fatalf("totals %+v", s)
	}
}

// TestProfileRepeats checks Repeats and MaxEvals per position and level, and Discard.
func TestProfileRepeats(t *testing.T) {
	var pr Profile
	r := pr.Recorder()
	r.Record(enter("s", 1, 0))
	for range 3 {
		r.Record(enter("e", 2, 4))
		r.Record(exit("e", 2, 4, 4, false, 1, 5))
	}
	r.Discard(5)
	r.Record(enter("e", 2, 4))
	r.Record(exit("e", 2, 4, 4, false, 1, 5))
	r.Record(exit("s", 1, 0, 0, false, 1, 5))
	e := pr.Snapshot().Rules[0]
	if e.Repeats != 2 || e.MaxEvals != 3 || e.MaxEvalsAt.Pos != 4 {
		t.Fatalf("e %+v", e)
	}
}

func TestTraceEventJSON(t *testing.T) {
	b, err := json.Marshal(TraceEvent{Kind: TraceSkipped, Rule: "r", SkipReason: SkipFirstCharacter})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"kind":"skipped","rule":"r","level":0,"depth":0,"pos":0,"lookahead":false,"matched":false,"end":0,"memo":false,"evals":0,"reusedElements":0,"examined":0,"skipReason":"first-character"}`
	if string(b) != want {
		t.Fatalf("got %s", b)
	}
	var e TraceEvent
	if err := json.Unmarshal(b, &e); err != nil || e.Kind != TraceSkipped {
		t.Fatalf("round trip %+v %v", e, err)
	}
	if e.Text() != "" || e.Failure() != nil || e.Recovered() != nil {
		t.Fatal("accessors of an event without a source")
	}
	if l, c := e.LineCol(3); l != 0 || c != 0 {
		t.Fatal("LineCol without a source")
	}
}

func TestProfileHints(t *testing.T) {
	var pr Profile
	r := pr.Recorder()
	r.Record(enter("s", 1, 0))
	r.Record(enter("x", 2, 0))
	r.Record(exit("x", 2, 0, 0, false, 1, 40))
	r.Record(exit("s", 1, 0, 0, false, 1, 40))
	hints := pr.Snapshot().Hints()
	if !strings.Contains(strings.Join(hints, "\n"), "x (40 positions from position 0)") {
		t.Fatalf("hints %q", hints)
	}
	if (*Profile)(nil).Snapshot().Rules == nil {
		t.Fatal("nil profile snapshot has nil Rules")
	}
}

// streamSource is a TraceSource of a stream that holds the input from base on.
type streamSource struct{ base int }

func (s *streamSource) LineCol(int) (int, int)    { return 0, 0 }
func (s *streamSource) Text(int, int) string      { return "" }
func (s *streamSource) Failure() *SyntaxError     { return nil }
func (s *streamSource) Recovered() []*SyntaxError { return nil }
func (s *streamSource) Retained() int             { return s.base }

// TestProfileRecorderStreamBound checks that the evaluation counts of a long stream stay bounded:
// the positions the source no longer holds are dropped.
func TestProfileRecorderStreamBound(t *testing.T) {
	var pr Profile
	r := pr.Recorder()
	src := &streamSource{}
	r.Record(enter("s", 1, 0).WithSource(src))
	const n = 100 * profileSeenMin
	for pos := range n {
		src.base = max(pos-10, 0)
		r.Record(enter("line", 2, pos).WithSource(src))
		r.Record(exit("line", 2, pos, pos+1, true, 1, pos+1).WithSource(src))
	}
	if len(r.seen) > 4*profileSeenMin {
		t.Fatalf("%d positions kept", len(r.seen))
	}
	r.Record(exit("s", 1, 0, n, true, 1, n).WithSource(src))
	if row := pr.Snapshot().Rules[0]; row.Calls != n || row.Repeats != 0 {
		t.Fatalf("line %+v", row)
	}
}
