package instrument

import (
	"encoding/json"
	"flag"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
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

// TestProfileRecorderReuse checks that a recorder takes over the buffers of the last closed one
// without its data: the counts of a parse do not depend on the parses before it, including one
// interrupted with calls in progress and one nested within another, and a parse after the first
// allocates only its recorder.
func TestProfileRecorderReuse(t *testing.T) {
	var pr Profile
	parse := func(r *ProfileRecorder) {
		r.Record(enter("s", 1, 0))
		for range 2 {
			r.Record(enter("e", 2, 4))
			r.Record(exit("e", 2, 4, 4, false, 1, 5))
		}
		r.Record(exit("s", 1, 0, 0, false, 1, 5))
	}
	repeats := func() int {
		for _, row := range pr.Snapshot().Rules {
			if row.Rule == "e" {
				return row.Repeats
			}
		}
		return -1
	}
	// An interrupted parse leaves calls in progress and an evaluation of e at 4.
	r := pr.Recorder()
	r.Record(enter("s", 1, 0))
	r.Record(enter("e", 2, 4))
	r.Record(exit("e", 2, 4, 4, false, 1, 5))
	r.Record(enter("e", 2, 4))
	r.Close()
	before := repeats()
	r = pr.Recorder()
	parse(r)
	r.Close()
	if got := repeats() - before; got != 1 {
		t.Errorf("repeats of a parse after an interrupted one = %d, want 1", got)
	}
	if s := pr.Snapshot(); s.Rules[0].Rule != "e" || s.Rules[0].Time == 0 {
		t.Errorf("time of e after an interrupted parse: %+v", s.Rules[0])
	}
	// A nested recorder neither shares the buffers of the enclosing one nor leaves it data.
	before = repeats()
	outer := pr.Recorder()
	outer.Record(enter("s", 1, 0))
	outer.Record(enter("e", 2, 4))
	outer.Record(exit("e", 2, 4, 4, false, 1, 5))
	inner := pr.Recorder()
	parse(inner)
	inner.Close()
	outer.Record(exit("s", 1, 0, 0, false, 1, 5))
	outer.Close()
	if got := repeats() - before; got != 1 {
		t.Errorf("repeats of a nested parse and its enclosing one = %d, want 1 (the nested one's)", got)
	}
	if n := testing.AllocsPerRun(10, func() { r := pr.Recorder(); parse(r); r.Close() }); n > 1 {
		t.Errorf("a parse after the first allocates %v times, want only its recorder", n)
	}
}

// TestProfileRecorderKeepsSeenRelative checks when a profile keeps the seen map of a closed recorder:
// unless the map once held more than spareSeenRatio times the entries of the parse that closes it,
// and more than spareSeenFloor. A parse of n calls of x has n+1 entries (with s).
func TestProfileRecorderKeepsSeenRelative(t *testing.T) {
	var pr Profile
	parse := func(n int) {
		r := pr.Recorder()
		r.Record(enter("s", 1, 0))
		for pos := range n {
			r.Record(enter("x", 2, pos))
			r.Record(exit("x", 2, pos, pos+1, true, 1, pos+1))
		}
		r.Record(exit("s", 1, 0, n, true, 1, n))
		r.Close()
	}
	for _, c := range []struct {
		n    int
		kept bool
	}{
		{4096, true},               // the first parse keeps its map: 4097 entries
		{2048, true},               // within the ratio
		{1024, true},               // 1025 entries: 4 times them reach the map's peak
		{1023, false},              // 1024 entries: past the ratio, so dropped
		{10, true},                 // a new map, small
		{spareSeenFloor * 2, true}, // grows the map
		{10, false},                // past the ratio and the floor
		{spareSeenFloor - 1, true}, // a new map, at the floor
		{spareSeenFloor / 8, true}, // past the ratio but within the floor
	} {
		parse(c.n)
		if kept := pr.spare.seen != nil; kept != c.kept {
			t.Fatalf("after a parse of %d entries kept = %v, want %v (peak %d)", c.n, kept, c.kept, pr.spare.seenPeak)
		}
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

// TestProfileRecorderStreamBound checks that the evaluation counts of a long stream stay bounded:
// the positions a stream discards are dropped, and only those.
func TestProfileRecorderStreamBound(t *testing.T) {
	var pr Profile
	r := pr.Recorder()
	r.Record(enter("s", 1, 0))
	const n, keep = 100_000, 10
	for pos := range n {
		r.Record(enter("line", 2, pos))
		r.Record(exit("line", 2, pos, pos+1, true, 1, pos+1))
		if pos%100 == 0 {
			r.Discard(pos - keep)
		}
	}
	if len(r.seen) > 100+keep {
		t.Fatalf("%d positions kept", len(r.seen))
	}
	for k := range r.seen {
		if k.pos < n-100-keep {
			t.Fatalf("position %d kept after it was discarded", k.pos)
		}
	}
	r.Record(exit("s", 1, 0, n, true, 1, n))
	if row := pr.Snapshot().Rules[0]; row.Calls != n || row.Repeats != 0 {
		t.Fatalf("line %+v", row)
	}
}

// TestProfileRecorderExcludesTraceFunctions checks that the time between Record and Resume, where the
// trace functions run, is not counted, and is counted without Resume. Both bounds hold at any machine
// load: with Resume, the times lie within the recording less the measured pauses; without it, the time
// of a call includes every pause measured between its enter and exit events.
func TestProfileRecorderExcludesTraceFunctions(t *testing.T) {
	const pause = 2 * time.Millisecond
	events := []TraceEvent{enter("s", 1, 0), enter("a", 2, 0), exit("a", 2, 0, 1, true, 1, 1), exit("s", 1, 0, 1, true, 1, 1)}
	for _, resume := range []bool{true, false} {
		var pr Profile
		var paused [4]time.Duration // after each event
		start := time.Now()
		r := pr.Recorder()
		for i, e := range events {
			r.Record(e)
			p := time.Now()
			time.Sleep(pause)
			paused[i] = time.Since(p)
			if resume {
				r.Resume()
			}
		}
		r.Close()
		wall := time.Since(start)
		s := pr.Snapshot()
		a, root := s.Rules[0], s.Rules[1]
		if resume {
			limit := wall - (paused[0] + paused[1] + paused[2] + paused[3])
			if s.Time > limit || a.Time > limit || root.Time > limit || root.SelfTime > limit {
				t.Errorf("trace function time counted: total %v, a %v, s %v self %v, limit %v", s.Time, a.Time, root.Time, root.SelfTime, limit)
			}
		} else if s.Time < paused[0]+paused[1]+paused[2] || a.Time < paused[1] || root.SelfTime < paused[0]+paused[2] {
			t.Errorf("time without Resume: total %v, a %v, s self %v, pauses %v", s.Time, a.Time, root.SelfTime, paused)
		}
	}
}

// TestProfileRecorderNesting checks the time of a rule called within a call of itself (counted once),
// self times, the counts of a left-recursive growth, and Lookahead not changing the counts.
func TestProfileRecorderNesting(t *testing.T) {
	var pr Profile
	r := pr.Recorder()
	la := enter("e", 3, 0)
	la.Lookahead = true
	lax := exit("e", 3, 0, 0, false, 1, 1)
	lax.Lookahead = true
	for _, e := range []TraceEvent{
		enter("s", 1, 0),
		enter("e", 2, 0),
		la, lax,
		enter("e", 3, 0), exit("e", 3, 0, 1, true, 0, 1),
		exit("e", 2, 0, 3, true, 3, 4), // two growth steps
		exit("s", 1, 0, 3, true, 1, 4),
	} {
		r.Record(e)
		time.Sleep(time.Millisecond)
	}
	r.Close()
	s := pr.Snapshot()
	e, root := s.Rules[0], s.Rules[1]
	if e.Entered != 3 || e.Calls != 3 || e.Evals != 4 || e.MemoHits != 1 || e.Failed != 1 || e.Wasted != 1 || e.Examined != 6 || e.Consumed != 4 {
		t.Errorf("e %+v", e)
	}
	if e.Time > root.Time || e.SelfTime > e.Time || root.SelfTime+e.Time > root.Time+time.Millisecond/2 {
		t.Errorf("times: e %v self %v, s %v self %v", e.Time, e.SelfTime, root.Time, root.SelfTime)
	}
	if e.Time < 5*time.Millisecond {
		t.Errorf("outer e call time %v, want the nested calls counted once within it", e.Time)
	}
	if s.Evals != 5 || s.Examined != 4 || s.Time != root.Time {
		t.Errorf("totals %+v", s)
	}
}

// TestProfileSnapshotJSON pins the JSON of a snapshot and of events of every kind
// (testdata/profile.golden.json; go test -run TestProfileSnapshotJSON -update rewrites it).
func TestProfileSnapshotJSON(t *testing.T) {
	var pr Profile
	r := pr.Recorder()
	events := []TraceEvent{
		enter("s", 1, 0),
		{Kind: TraceSkipped, Rule: "b", Depth: 2, Pos: 0, SkipReason: SkipFirstCharacter},
		{Kind: TraceEnter, Rule: "a", Level: 2, Depth: 2, Pos: 0, Lookahead: true},
		{Kind: TraceExit, Rule: "a", Level: 2, Depth: 2, Pos: 0, Lookahead: true, Matched: false, End: 0, Evals: 1, Examined: 3},
		enter("a", 2, 0), exit("a", 2, 0, 2, true, 1, 2),
		enter("a", 2, 2), exit("a", 2, 2, 2, false, 0, 3),
		{Kind: TraceExit, Rule: "s", Depth: 1, Pos: 0, Matched: true, End: 2, Evals: 1, ReusedElements: 4, Examined: 3},
	}
	for _, e := range events {
		r.Record(e)
	}
	r.Close()
	s := pr.Snapshot()
	s.Time = 0
	for i := range s.Rules {
		s.Rules[i].Time, s.Rules[i].SelfTime = 0, 0
	}
	got, err := json.MarshalIndent(struct {
		Events   []TraceEvent    `json:"events"`
		Snapshot ProfileSnapshot `json:"snapshot"`
	}{events, s}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	const golden = "testdata/profile.golden.json"
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("JSON differs from %s (-update rewrites it):\n%s", golden, got)
	}
	var back struct {
		Events   []TraceEvent    `json:"events"`
		Snapshot ProfileSnapshot `json:"snapshot"`
	}
	if err := json.Unmarshal(want, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back.Events, events) || !reflect.DeepEqual(back.Snapshot, s) {
		t.Fatalf("round trip: %+v", back)
	}
}

var update = flag.Bool("update", false, "rewrite the golden files")
