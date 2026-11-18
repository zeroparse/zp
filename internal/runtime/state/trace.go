package state

import (
	"zeroparse.dev/zp/core"
)

// Tracing reports every rule call of a parse to a function (ParseOptions.Trace). It is built
// around the one function all backends call for a rule (Parser.Call), so that an ordinary parse
// pays for it with a single nil check per call of a rule that is not plain; plain calls, which
// skip call, take that path only when the parser allows it (Parser.NoPlain), which a traced
// parse does not: it runs the call-preserving family of its backend (FamilyOf). See
// docs/design/records/014-tracing-and-profiling.md.

// TraceKind is core.TraceKind. A parse reports TraceEnter and TraceExit; the engine reports no
// TraceSkipped events (the calls that its backends skip are not reported).
type TraceKind = core.TraceKind

const (
	TraceEnter = core.TraceEnter
	TraceExit  = core.TraceExit
)

// TraceEvent is core.TraceEvent. Its accessors read the parse's tracing state (Tracer, the event's
// TraceSource): call them only from the trace function, while it handles the event.
type TraceEvent = core.TraceEvent

// TraceParser returns the parser that reported e, or nil for an event that no parse reported.
// It gives code outside the public API the parse state behind an event.
func TraceParser(e TraceEvent) *Parser {
	if t, ok := e.Source().(*Tracer); ok {
		return t.p
	}
	return nil
}

// TraceExpectations returns the expectations of the farthest failure that the exit event e, the one
// the trace function is handling, records.
func TraceExpectations(e TraceEvent) []ExpID {
	if t, ok := e.Source().(*Tracer); ok && e.Kind == TraceExit {
		return t.exp
	}
	return nil
}

// Tracer is the tracing state of a parse. It is the TraceSource of the parse's events: the state
// of the exit event being reported (far, exp and the recovered errors) is valid while the trace
// function and the profile recorders handle it.
type Tracer struct {
	p  *Parser
	fn func(TraceEvent)
	// rules, if not nil, selects the calls fn receives: those of the rules it holds and the calls
	// nested in them. maxDepth, if positive, leaves out the calls nested deeper.
	rules    map[string]bool
	maxDepth int
	// profiles record the events of the parse. Each records an event before fn handles it and resumes
	// after, so that the time of fn is not counted.
	profiles []*core.ProfileRecorder
	// direct is set by tracedCall just before it calls call for the call it has reported, so that
	// call runs it instead of reporting it again.
	direct bool
	stack  []tracedCallState
	// The exit being reported: its farthest failure and expectations, and the errors recovered
	// during the call: tail, the errors of a bounded stream's tail, or the entries of the record of
	// recovered errors from rec on.
	far  int
	exp  []ExpID
	tail bool
	rec  int
}

// LineCol returns the 1-based line and column of the position pos. In a stream parse, it returns
// 0, 0 for a position (other than 0) in input that has been discarded or not read yet: keeping
// the line starts of discarded input would make the memory of a stream grow with its length.
func (t *Tracer) LineCol(pos int) (line, col int) {
	switch {
	case pos == 0:
		return 1, 1
	case pos < t.p.Base || pos > t.p.Loaded():
		return 0, 0
	}
	return t.p.lineCol(pos)
}

// Text returns the input [start, end). Input that a stream parse has already discarded is not
// available.
func (t *Tracer) Text(start, end int) string {
	return t.p.Text(start, end)
}

// Failure returns the farthest failure of the exit being reported.
func (t *Tracer) Failure() *core.SyntaxError {
	if len(t.exp) == 0 {
		return nil
	}
	return t.p.MakeError(t.far, t.exp)
}

// Recovered returns the errors recovered during the call of the exit being reported.
func (t *Tracer) Recovered() []*core.SyntaxError {
	p := t.p
	if t.tail {
		// A bounded stream has released earlier committed recoveries: report the same ordered
		// tail, including a successful suffix.
		return p.DiagnosticTail.Preview(p, p.RecoveredErrors())
	}
	if len(p.Recovered) > t.rec {
		return appendRecErrors(nil, p.Recovered[t.rec:])
	}
	return nil
}

// discard tells the profile recorders that a stream discarded the input before pos: no rule is
// evaluated there again.
func (t *Tracer) discard(pos int) {
	for _, r := range t.profiles {
		r.Discard(pos)
	}
}

// emit passes ev to the profile recorders and, if the call is selected and no deeper than the
// depth limit, to the trace function. A panic there is carried through recoverParse as a
// TracePanic, so that it is not taken for a fault of the parse (a runtime error on a bytecode
// backend is reported as invalid bytecode), and is raised again unchanged.
func (t *Tracer) emit(ev TraceEvent, selected bool) {
	p := t.p
	p.Control.Check()
	deliver := t.fn != nil && selected && (t.maxDepth <= 0 || ev.Depth <= t.maxDepth)
	for _, r := range t.profiles {
		r.Record(ev)
	}
	if !deliver {
		return
	}
	if t.profiles != nil {
		defer func() {
			for _, r := range t.profiles {
				r.Resume()
			}
		}()
	}
	defer func() {
		if x := recover(); x != nil {
			if _, ok := x.(Fatal); ok {
				panic(x) // a charged TraceEvent accessor interrupted engine execution
			}
			panic(TracePanic{x})
		}
	}()
	t.fn(ev)
	p.Control.Check()
}

// TracePanic is a panic of the trace function, with the value it panicked with.
type TracePanic struct{ Value any }

// tracedCallState is the state of a traced call in progress.
type tracedCallState struct {
	r     *Rule
	min   int
	start int
	// evals is the number of body evaluations of the parse when the call started, and nested
	// those made by nested calls since.
	evals, nested int
	// resumed is the parse's repetition-element reuse count when the call started.
	resumed int
	// hw is the end of the examined input when the call started, and rec the number of
	// entries of the record of recovered errors.
	hw, rec int
	exp     ExpMark
	// selected is set when the trace function receives the call's events (Tracer.rules).
	selected bool
}

// SetTrace makes the parse report its rule calls to fn and record them in profiles (nothing if fn is
// nil and there are no profiles); SelectTrace then limits what fn receives. CloseTrace ends the
// recording when the parse returns.
func (p *Parser) SetTrace(fn func(TraceEvent), profiles []*core.Profile) {
	if fn == nil && len(profiles) == 0 {
		return
	}
	t := &Tracer{p: p, fn: fn}
	for _, pr := range profiles {
		t.profiles = append(t.profiles, pr.Recorder())
	}
	p.Tr = t
	p.NoPlain = true
}

// SelectTrace limits the events that the trace function given to SetTrace receives to those that s
// selects (all if s is nil). It is apart from SetTrace so that SetTrace stays small enough to
// inline into the parse entry points.
func (p *Parser) SelectTrace(s *TraceSelection) {
	if t := p.Tr; s != nil && t != nil {
		t.maxDepth = s.MaxDepth
		if len(s.Rules) != 0 {
			t.rules = make(map[string]bool, len(s.Rules))
			for _, name := range s.Rules {
				t.rules[name] = true
			}
		}
	}
}

// CloseTrace ends the recording of the parse's profiles (calls that began and did not complete
// stay counted as entered only) and drops what the tracer refers to besides the parser, which the
// events kept by the trace function still refer to.
func (p *Parser) CloseTrace() {
	if t := p.Tr; t != nil {
		for _, r := range t.profiles {
			r.Close()
		}
		*t = Tracer{p: p}
	}
}

// tracedCall reports a call of r, runs it with call, and reports its result.
func (p *Parser) tracedCall(r *Rule, min int) (*core.Node, bool) {
	p.TraceEnter(r, min)
	p.Tr.direct = true
	v, ok := p.Call(r, min)
	p.TraceExit(ok)
	return v, ok
}

// TraceEnter reports the start of a call of r and starts measuring it. The call's examined range
// and expectations are recorded separately, as CallBegin does for a memoized call, and merged
// back by TraceExit; doing so for any call leaves the parse's own records unchanged (both only
// keep a maximum).
func (p *Parser) TraceEnter(r *Rule, min int) {
	t := p.Tr
	selected := t.rules == nil || t.rules[r.Name] || len(t.stack) > 0 && t.stack[len(t.stack)-1].selected
	t.stack = append(t.stack, tracedCallState{r: r, min: min, start: p.Pos, evals: p.Stats.Evaluated, resumed: p.Resumed, hw: p.Hw, rec: len(p.Recovered), selected: selected})
	p.Hw = p.Pos
	t.stack[len(t.stack)-1].exp = p.Isolate(p.Pos)
	t.emit(TraceEvent{Kind: TraceEnter, Rule: r.Name, Level: min, Depth: len(t.stack), Pos: p.Pos, Lookahead: p.Silent > 0}.WithSource(t), selected)
}

// TraceExit reports the end of the innermost call in progress, whose result is ok.
func (p *Parser) TraceExit(ok bool) {
	t := p.Tr
	c := t.stack[len(t.stack)-1]
	all := p.Stats.Evaluated - c.evals
	far, inner := p.Unisolate(c.exp)
	ev := TraceEvent{Kind: TraceExit, Rule: c.r.Name, Level: c.min, Depth: len(t.stack), Pos: c.start,
		Lookahead: p.Silent > 0, Matched: ok, End: c.start, Memo: all == 0, Evals: all - c.nested,
		Examined: p.Hw, ReusedElements: p.Resumed - c.resumed}.WithSource(t)
	t.far, t.exp, t.tail, t.rec = far, inner, false, len(p.Recovered)
	if ok {
		ev.End = p.Pos
		t.tail = len(t.stack) == 1 && p.DiagnosticTail.Limit > 0
		t.rec = c.rec
	}
	t.emit(ev, c.selected)
	t.exp = nil
	p.MergeExpected(far, inner)
	p.Hw = max(c.hw, p.Hw)
	t.stack = t.stack[:len(t.stack)-1]
	if n := len(t.stack); n > 0 {
		t.stack[n-1].nested += all
	}
}

// ComposeTrace joins public option callbacks while checking interruption before
// starting the next individual callback. Caller-created composite functions are
// still arbitrary caller code and cannot be preempted internally.
func ComposeTrace(prev, next func(TraceEvent)) func(TraceEvent) {
	return func(ev TraceEvent) {
		prev(ev)
		if BudgetControlsEnabled {
			TraceParser(ev).Control.Check()
		}
		next(ev)
	}
}
