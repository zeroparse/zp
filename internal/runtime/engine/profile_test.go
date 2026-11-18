package engine

import (
	"runtime"
	"strings"
	"testing"

	zpinput "zeroparse.dev/zp/pkg/runtime/input"
	"zeroparse.dev/zp/pkg/runtime/instrument"
	"zeroparse.dev/zp/pkg/runtime/value"
)

func TestProfile(t *testing.T) {
	t.Parallel()
	// x calls no other rule, so it is not memoized (transient) and each alternative of main
	// evaluates it again at 0; y fails after examining the whole input.
	prog := mustCompile(t, `rule main = y / x "a" / x "b" / x "c"
rule y = "x"+ "!"
rule x = "x"+`)
	for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
		var prof instrument.Profile
		for range 2 {
			if _, err := prog.ParseWith("main", "xxc", ParseOptions{Backend: b, Profiles: []*instrument.Profile{&prof}}); err != nil {
				t.Fatal(err)
			}
		}
		pr := prof.Snapshot()
		if pr.Parses != 2 || pr.Calls != 10 || pr.Evals != 10 || pr.MemoHits != 0 || pr.Examined != 6 || pr.Time < 0 {
			t.Errorf("%v: totals %+v", b, pr)
		}
		got := map[string]instrument.RuleProfile{}
		for _, r := range pr.Rules {
			got[r.Rule] = r
			if r.SelfTime < 0 || r.SelfTime > r.Time || r.Time > pr.Time {
				t.Errorf("%v: %s: time %v, self %v", b, r.Rule, r.Time, r.SelfTime)
			}
		}
		if names := len(pr.Rules); names != 3 || pr.Rules[0].Rule != "main" || pr.Rules[1].Rule != "x" || pr.Rules[2].Rule != "y" {
			t.Errorf("%v: rules in order of name: %+v", b, pr.Rules)
		}
		x, y, m := got["x"], got["y"], got["main"]
		if x.Calls != 6 || x.Evals != 6 || x.Matched != 6 || x.Failed != 0 || x.Consumed != 12 || x.Wasted != 0 ||
			x.Repeats != 4 || x.MaxEvals != 3 || x.MaxEvalsAt != (instrument.CallSite{Location: zpinput.Location{Pos: 0, Line: 1, Col: 1}}) {
			t.Errorf("%v: x %+v", b, x)
		}
		if y.Calls != 2 || y.Failed != 2 || y.Wasted != 6 || y.LongestFail != 3 || y.LongestFailAt != (instrument.CallSite{Location: zpinput.Location{Pos: 0, Line: 1, Col: 1}}) || y.Repeats != 0 {
			t.Errorf("%v: y %+v", b, y)
		}
		if m.Calls != 2 || m.Matched != 2 || m.Consumed != 6 || m.Time != pr.Time {
			t.Errorf("%v: main %+v", b, m)
		}
		hints := strings.Join(pr.Hints(), "\n")
		for _, want := range []string{
			"Evaluated again where they had been evaluated before: x (4 of 6 evaluations, up to 3 times at 1:1).",
			"Failed calls examined more input in all than the parse did (6 positions): y (6 positions in 2 failed calls).",
		} {
			if !strings.Contains(hints, want) {
				t.Errorf("%v: hints lack %q:\n%s", b, want, hints)
			}
		}
	}
}

// TestProfileMemoAndRecursion checks memo hits, left-recursive growth and the time of recursive
// calls.
func TestProfileMemoAndRecursion(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, `rule main = a "!" / a "?"
rule a = a "x" / "y"`)
	var prof instrument.Profile
	if _, err := prog.ParseWith("main", "yxx?", ParseOptions{Profiles: []*instrument.Profile{&prof}}); err != nil {
		t.Fatal(err)
	}
	pr := prof.Snapshot()
	var a instrument.RuleProfile
	for _, r := range pr.Rules {
		if r.Rule == "a" {
			a = r
		}
	}
	// The first call of a grows the match in 4 evaluations, during which 4 recursive calls take
	// the result so far from the memo; the second call from main takes the result from the memo.
	if a.Calls != 6 || a.Evals != 4 || a.MemoHits != 5 || a.Time > pr.Time || a.Repeats != 0 {
		t.Errorf("a %+v", a)
	}
	if strings.Contains(strings.Join(pr.Hints(), "\n"), "Evaluated again") {
		t.Errorf("hints %q", pr.Hints())
	}
}

// TestStreamLocations checks that, in a stream parse, events never give a wrong line and column
// for a position whose input has been discarded (they give 0, 0), and that the profile's
// locations are right or unknown.
func TestStreamLocations(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, `rule main = line* #stream $$
rule line = [a-z]* #atomic "\n"`)
	var b strings.Builder
	for range 3000 {
		b.WriteString("abcdefgh\n")
	}
	input := b.String()
	want := func(pos int) (int, int) { return pos/9 + 1, pos%9 + 1 }
	for _, back := range []Backend{Closure, Bytecode, BytecodeIterative} {
		var prof instrument.Profile
		bad, unknown := 0, 0
		trace := func(e TraceEvent) {
			// Only the start of the start rule's call precedes discarded input: check one more
			// position, at the start of line 2, which is not read yet at first and then discarded.
			for _, pos := range []int{e.Pos, e.End, 9} {
				line, col := e.LineCol(pos)
				if line == 0 && col == 0 {
					unknown++
				} else if l, c := want(pos); line != l || col != c {
					bad++
				}
			}
		}
		err := prog.ParseStreamWith("main", strings.NewReader(input), func(*value.Node) error { return nil }, ParseOptions{Backend: back, Trace: trace, Profiles: []*instrument.Profile{&prof}})
		if err != nil {
			t.Fatal(err)
		}
		if bad > 0 || unknown == 0 {
			t.Errorf("%v: %d wrong locations, %d unknown", back, bad, unknown)
		}
		for _, r := range prof.Snapshot().Rules {
			for _, l := range []zpinput.Location{r.MaxEvalsAt.Location, r.LongestFailAt.Location} {
				if wl, wc := want(l.Pos); l.Line != 0 && (l.Line != wl || l.Col != wc) {
					t.Errorf("%v: %s: location %+v", back, r.Rule, l)
				}
			}
		}
	}
	if got := (zpinput.Location{Pos: 12}).String(); got != "position 12" {
		t.Errorf("unknown location prints as %q", got)
	}
}

// TestHintsWithoutTime checks that, when no time was measured (so there is no time hint), a
// profile with one finding does not also say that nothing stands out.
func TestHintsWithoutTime(t *testing.T) {
	t.Parallel()
	pr := instrument.ProfileSnapshot{Calls: 10, Examined: 100, Rules: []instrument.RuleProfile{
		{Rule: "x", Calls: 10, Evals: 10, Repeats: 5, MaxEvals: 3, MaxEvalsAt: instrument.CallSite{Location: zpinput.Location{Pos: 0, Line: 1, Col: 1}}},
	}}
	hints := pr.Hints()
	if len(hints) != 1 || !strings.HasPrefix(hints[0], "Evaluated again") {
		t.Errorf("hints %q", hints)
	}
	pr.Rules[0].MaxEvals = 2
	if hints := pr.Hints(); len(hints) != 1 || !strings.HasPrefix(hints[0], "Nothing else stands out") {
		t.Errorf("hints %q", hints)
	}
}

// TestProfileStreamMemory checks that profiling a stream keeps its memory bounded: the profile
// forgets the positions evaluated once the stream has discarded them. It measures the live heap
// early in the stream and near its end, so it does not run in parallel with other tests.
func TestProfileStreamMemory(t *testing.T) {
	prog := mustCompile(t, `rule main = line* #stream $$
rule line = [a-z]* #atomic "\n"`)
	const lines = 200_000
	heap := func() uint64 {
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return m.HeapAlloc
	}
	for _, back := range []Backend{Closure, Bytecode, BytecodeIterative} {
		var prof instrument.Profile
		var early, late uint64
		trace := func(e TraceEvent) {
			if e.Kind != TraceExit || e.Rule != "line" {
				return
			}
			switch e.Pos {
			case 9 * (lines / 10):
				early = heap()
			case 9 * (lines - 10):
				late = heap()
			}
		}
		r := strings.NewReader(strings.Repeat("abcdefgh\n", lines))
		opts := ParseOptions{Backend: back, Trace: trace, Profiles: []*instrument.Profile{&prof}}
		if err := prog.ParseStreamWith("main", r, func(*value.Node) error { return nil }, opts); err != nil {
			t.Fatal(err)
		}
		var line *instrument.RuleProfile
		ps := prof.Snapshot()
		for i := range ps.Rules {
			if ps.Rules[i].Rule == "line" {
				line = &ps.Rules[i]
			}
		}
		// Without discarding, the profile would keep one more position per line: some megabytes
		// over the lines between the two measurements.
		if early == 0 || late == 0 || late > early+1<<20 || line == nil || line.Calls != lines+1 || line.Repeats != 0 {
			t.Errorf("%v: heap %d then %d, line %+v", back, early, late, line)
		}
	}
}
