package core

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Profile accumulates the cost of each rule over the parses it is given to. Its zero value is
// ready to use. It is not safe for concurrent use; parses that run one within another on the same
// goroutine (from a trace function) are accounted separately. It keeps no input, nodes or history
// of a parse after the parse returns. Snapshot reads it.
type Profile struct {
	totals ProfileSnapshot
	rows   []*RuleProfile
	index  map[string]int
}

// ProfileSnapshot is a copy of the counts of a Profile, with Rules sorted by rule name. Entered
// counts calls that began; the other counts and the times cover only the calls that completed
// (reported their exit). Positions are in the position unit of each parse, not normalized.
type ProfileSnapshot struct {
	// Parses is the number of parses profiled.
	Parses int `json:"parses"`
	// Entered, Calls, Evals, MemoHits and Skipped total the rules' counts.
	Entered  int `json:"entered"`
	Calls    int `json:"calls"`
	Evals    int `json:"evals"`
	MemoHits int `json:"memoHits"`
	Skipped  int `json:"skipped"`
	// ReusedElements totals the repetition elements reused by completed start rule calls. The
	// rows of nested rules include this work too, so they are not added to it.
	ReusedElements int `json:"reusedElements"`
	// Examined totals, over the completed start rule calls, the input they examined.
	Examined int `json:"examined"`
	// Time totals the time of the completed start rule calls (in nanoseconds in JSON).
	Time time.Duration `json:"time"`
	// Rules holds the rules called or skipped, sorted by name.
	Rules []RuleProfile `json:"rules"`
}

// RuleProfile is the cost of one rule.
type RuleProfile struct {
	Rule string `json:"rule"`
	// Entered is the number of calls that began, Calls the number that completed. Each completed
	// call either evaluated the rule body or took the result from the memo (MemoHits); it matched
	// (Matched) or failed (Failed).
	Entered  int `json:"entered"`
	Calls    int `json:"calls"`
	MemoHits int `json:"memoHits"`
	Matched  int `json:"matched"`
	Failed   int `json:"failed"`
	// Evals is the number of body evaluations: one per call not answered by the memo, and one more
	// for each step a left-recursive match grows.
	Evals int `json:"evals"`
	// ReusedElements counts repetition elements reused during the calls, nested calls included.
	// A nested call can report the same reuse as its caller, so rows are not to be summed.
	ReusedElements int `json:"reusedElements"`
	// Consumed totals the input matched by the calls that matched.
	Consumed int `json:"consumed"`
	// Skipped counts the calls the parser skipped (they are not entered), by reason in SkipReasons.
	Skipped     int               `json:"skipped"`
	SkipReasons []SkipReasonCount `json:"skipReasons"`
	// Examined totals the input the calls examined, from the position of each call. Wasted totals
	// the same for the calls that evaluated the body and failed: the work that led to nothing. Both
	// are inclusive: a call counts what the calls it made examined. LongestFail is the most input
	// a single such failed call examined, at LongestFailAt.
	Examined      int      `json:"examined"`
	Wasted        int      `json:"wasted"`
	LongestFail   int      `json:"longestFail"`
	LongestFailAt CallSite `json:"longestFailAt"`
	// Repeats is the number of calls that evaluated the body at a position and binding level where
	// an earlier call of the same parse had evaluated it already. MaxEvals is the most evaluating
	// calls at one position and level, first reached at MaxEvalsAt.
	Repeats    int      `json:"repeats"`
	MaxEvals   int      `json:"maxEvals"`
	MaxEvalsAt CallSite `json:"maxEvalsAt"`
	// Time is the time spent in the calls, nested calls included; a call within a call of the same
	// rule counts once. SelfTime excludes the time of nested calls. Neither includes the trace
	// functions of the parse; both include the cost of measuring, so they are meaningful relative to
	// each other. JSON gives them in nanoseconds.
	Time     time.Duration `json:"time"`
	SelfTime time.Duration `json:"selfTime"`
}

// SkipReasonCount counts the calls of a rule skipped for one reason.
type SkipReasonCount struct {
	Reason SkipReason `json:"reason"`
	Count  int        `json:"count"`
}

// Snapshot returns a copy of the counts, which later parses do not change.
func (pr *Profile) Snapshot() ProfileSnapshot {
	s := ProfileSnapshot{Rules: []RuleProfile{}}
	if pr == nil {
		return s
	}
	s = pr.totals
	s.Rules = make([]RuleProfile, len(pr.rows))
	for i, r := range pr.rows {
		s.Rules[i] = *r
		s.Rules[i].SkipReasons = slices.Clone(r.SkipReasons)
		if s.Rules[i].SkipReasons == nil {
			s.Rules[i].SkipReasons = []SkipReasonCount{}
		}
		slices.SortFunc(s.Rules[i].SkipReasons, func(a, b SkipReasonCount) int { return cmp.Compare(a.Reason, b.Reason) })
	}
	slices.SortFunc(s.Rules, func(a, b RuleProfile) int { return cmp.Compare(a.Rule, b.Rule) })
	return s
}

// row returns the index of the row of the rule name, adding it if needed.
func (pr *Profile) row(name string) int {
	if i, ok := pr.index[name]; ok {
		return i
	}
	if pr.index == nil {
		pr.index = map[string]int{}
	}
	pr.index[name] = len(pr.rows)
	pr.rows = append(pr.rows, &RuleProfile{Rule: name})
	return len(pr.rows) - 1
}

// ProfileRecorder adds the events of one parse to a Profile. Parsers make one per parse
// (Profile.Recorder), pass it every event of the parse in order (Record), and close it when the
// parse returns (Close).
type ProfileRecorder struct {
	pr    *Profile
	stack []profileFrame
	// seen counts the evaluating calls per rule, position and level; it is bounded in a stream parse
	// by dropping the positions the parser no longer holds once it reaches seenMax entries.
	seen    map[profileKey]int32
	seenMax int
	open    []int32 // per row, the calls in progress (for Time)
	// excluded is the time excluded from the calls so far (the trace functions), and paused the
	// time the parse was paused for one.
	excluded time.Duration
	paused   time.Time
}

type profileFrame struct {
	row      int
	start    time.Time
	excluded time.Duration // excluded at start
	child    time.Duration
}

type profileKey struct {
	row, level int32
	pos        int
}

// profileSeenMin is the smallest seenMax.
const profileSeenMin = 1024

// Recorder returns a recorder of one parse into pr, or nil if pr is nil. The methods of a nil
// recorder do nothing.
func (pr *Profile) Recorder() *ProfileRecorder {
	if pr == nil {
		return nil
	}
	return &ProfileRecorder{pr: pr, seenMax: profileSeenMin}
}

// Record adds the event e. A parser calls it before the trace functions of the parse handle e,
// and Resume after they have, so that their time is not counted.
func (r *ProfileRecorder) Record(e TraceEvent) {
	if r == nil {
		return
	}
	now := time.Now()
	pr := r.pr
	switch e.Kind {
	case TraceEnter:
		if len(r.stack) == 0 {
			pr.totals.Parses++
		}
		i := pr.row(e.Rule)
		for len(r.open) <= i {
			r.open = append(r.open, 0)
		}
		r.open[i]++
		pr.rows[i].Entered++
		pr.totals.Entered++
		r.stack = append(r.stack, profileFrame{row: i, excluded: r.excluded})
		r.stack[len(r.stack)-1].start = time.Now()
	case TraceExit:
		r.exit(e, now)
	case TraceSkipped:
		row := pr.rows[pr.row(e.Rule)]
		row.Skipped++
		pr.totals.Skipped++
		for i := range row.SkipReasons {
			if row.SkipReasons[i].Reason == e.SkipReason {
				row.SkipReasons[i].Count++
				r.paused = time.Now()
				return
			}
		}
		row.SkipReasons = append(row.SkipReasons, SkipReasonCount{Reason: e.SkipReason, Count: 1})
	}
	r.paused = time.Now()
}

// Resume tells r that the trace functions have handled the last event recorded.
func (r *ProfileRecorder) Resume() {
	if r != nil && !r.paused.IsZero() {
		r.excluded += time.Since(r.paused)
		r.paused = time.Time{}
	}
}

func (r *ProfileRecorder) exit(e TraceEvent, now time.Time) {
	if len(r.stack) == 0 {
		return
	}
	pr := r.pr
	c := r.stack[len(r.stack)-1]
	r.stack = r.stack[:len(r.stack)-1]
	d := now.Sub(c.start) - (r.excluded - c.excluded)
	row := pr.rows[c.row]
	row.SelfTime += d - c.child
	r.open[c.row]--
	if r.open[c.row] == 0 {
		row.Time += d
	}
	examined := max(e.Examined-e.Pos, 0)
	if n := len(r.stack); n > 0 {
		r.stack[n-1].child += d
	} else {
		pr.totals.Time += d
		pr.totals.Examined += examined
		pr.totals.ReusedElements += e.ReusedElements
	}
	row.Calls++
	pr.totals.Calls++
	row.Evals += e.Evals
	pr.totals.Evals += e.Evals
	row.ReusedElements += e.ReusedElements
	row.Examined += examined
	if e.Memo {
		row.MemoHits++
		pr.totals.MemoHits++
	}
	if e.Matched {
		row.Matched++
		row.Consumed += e.End - e.Pos
	} else {
		row.Failed++
		if !e.Memo {
			row.Wasted += examined
			if examined > row.LongestFail {
				row.LongestFail = examined
				row.LongestFailAt = callSite(e)
			}
		}
	}
	if !e.Memo && e.Evals > 0 {
		if r.seen == nil {
			r.seen = map[profileKey]int32{}
		}
		k := profileKey{row: int32(c.row), level: int32(e.Level), pos: e.Pos}
		n := r.seen[k] + 1
		r.seen[k] = n
		if len(r.seen) >= r.seenMax {
			if e.src != nil {
				r.Discard(e.src.Retained())
			}
			r.seenMax = max(profileSeenMin, 2*len(r.seen))
		}
		if n > 1 {
			row.Repeats++
		}
		if int(n) > row.MaxEvals {
			row.MaxEvals = int(n)
			row.MaxEvalsAt = callSite(e)
		}
	}
}

func callSite(e TraceEvent) CallSite {
	line, col := e.LineCol(e.Pos)
	return CallSite{Location: Location{Pos: e.Pos, Line: line, Col: col}, Level: e.Level}
}

// Discard tells r that the parse no longer holds the input before pos (a stream discarded it), so
// that no rule is evaluated there again.
func (r *ProfileRecorder) Discard(pos int) {
	if r == nil {
		return
	}
	for k := range r.seen {
		if k.pos < pos {
			delete(r.seen, k)
		}
	}
}

// Close ends the recording of the parse. Calls that began and did not complete stay counted in
// Entered only.
func (r *ProfileRecorder) Close() {
	if r != nil {
		*r = ProfileRecorder{}
	}
}

// Thresholds of Hints
const (
	// hintLongFail is the input a failed call must have examined to count as deep backtracking.
	hintLongFail = 16
	// hintRules is the most rules a hint names.
	hintRules = 3
)

// Hints returns observations about the profile that point at where a grammar does more work than
// it needs to, most important first. They are heuristics: each names what to look at, not what is
// wrong.
func (s ProfileSnapshot) Hints() []string {
	var hints []string
	list := func(rs []*RuleProfile, item func(*RuleProfile) string) string {
		parts := make([]string, len(rs))
		for i, r := range rs {
			parts[i] = item(r)
		}
		return strings.Join(parts, ", ")
	}
	if s.Time > 0 {
		if top := s.top(func(r *RuleProfile) int64 { return int64(r.SelfTime) }, hintRules); len(top) > 0 {
			hints = append(hints, fmt.Sprintf("Most time is spent in %s, not counting the rules they call.",
				list(top, func(r *RuleProfile) string {
					return fmt.Sprintf("%s (%.0f%%)", r.Rule, percent(int64(r.SelfTime), int64(s.Time)))
				})))
		}
	}
	timeHints := len(hints)
	if top := s.top(func(r *RuleProfile) int64 {
		if r.MaxEvals <= 2 {
			return 0
		}
		return int64(r.Repeats)
	}, hintRules); len(top) > 0 {
		hints = append(hints, fmt.Sprintf("Evaluated again where they had been evaluated before: %s. "+
			"A memoized rule is usually evaluated at most twice at a position; a rule that is not is evaluated "+
			"whenever its caller is retried there. Look for choices whose alternatives begin with the same "+
			"calls, and factor the common part out.",
			list(top, func(r *RuleProfile) string {
				return fmt.Sprintf("%s (%d of %d evaluations, up to %d times at %s)", r.Rule, r.Repeats, r.Evals, r.MaxEvals, r.MaxEvalsAt.Location)
			})))
	}
	if top := s.top(func(r *RuleProfile) int64 {
		if r.LongestFail < hintLongFail {
			return 0
		}
		return int64(r.LongestFail)
	}, hintRules); len(top) > 0 {
		hints = append(hints, fmt.Sprintf("Failed after examining much input: %s. "+
			"A call that fails late throws its work away, and what is tried next often matches the same input "+
			"again. Check whether an alternative shares a long prefix with a later one, and whether a cut (#cut) "+
			"after the part that identifies an alternative can stop the parser from trying the others.",
			list(top, func(r *RuleProfile) string {
				return fmt.Sprintf("%s (%d positions from %s)", r.Rule, r.LongestFail, r.LongestFailAt.Location)
			})))
	}
	if top := s.top(func(r *RuleProfile) int64 {
		if s.Examined == 0 || r.Wasted < s.Examined {
			return 0
		}
		return int64(r.Wasted)
	}, hintRules); len(top) > 0 {
		hints = append(hints, fmt.Sprintf("Failed calls examined more input in all than the parse did (%d positions): %s. "+
			"The rule is tried at many positions where it cannot match, or fails late; nested failures count in their "+
			"callers too, so look at the innermost of these first.",
			s.Examined, list(top, func(r *RuleProfile) string {
				return fmt.Sprintf("%s (%d positions in %d failed calls)", r.Rule, r.Wasted, r.Failed)
			})))
	}
	if len(hints) == timeHints && s.Calls > 0 {
		hints = append(hints, "Nothing else stands out: no rule is evaluated more than twice at a position, and no failed call examines much input.")
	}
	return hints
}

// top returns up to n rules with the highest positive key, highest first.
func (s ProfileSnapshot) top(key func(*RuleProfile) int64, n int) []*RuleProfile {
	var rs []*RuleProfile
	for i := range s.Rules {
		if r := &s.Rules[i]; key(r) > 0 {
			rs = append(rs, r)
		}
	}
	slices.SortStableFunc(rs, func(a, b *RuleProfile) int { return cmp.Compare(key(b), key(a)) })
	return rs[:min(n, len(rs))]
}

func percent(a, b int64) float64 {
	if b == 0 {
		return 0
	}
	return 100 * float64(a) / float64(b)
}
