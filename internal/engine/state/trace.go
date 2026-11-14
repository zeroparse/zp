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

// TraceKind and TraceEvent are the shared types of package core. The engine reports TraceEnter and
// TraceExit; it does not identify skipped calls.
type (
	TraceKind  = core.TraceKind
	TraceEvent = core.TraceEvent
)

const (
	TraceEnter = core.TraceEnter
	TraceExit  = core.TraceExit
)

// TraceParser returns the parser that reported e, or nil for an event that no parse reported.
// It gives code outside the public API the parse state behind an event.
func TraceParser(e TraceEvent) *Parser {
	if t, ok := e.Source().(*Tracer); ok {
		return t.p
	}
	return nil
}

// TraceExpectations returns the expectations of the farthest failure of the exit event that the
// trace function handles.
func TraceExpectations(e TraceEvent) []ExpID {
	if t, ok := e.Source().(*Tracer); ok && e.Kind == TraceExit {
		return t.exp
	}
	return nil
}

// Tracer is the tracing state of a parse. It is the core.TraceSource of the parse's events: the
// details of the exit event being handled (exp, far, rec, tail) are kept here, not in the event,
// and read only while the trace function runs.
type Tracer struct {
	p    *Parser
	fn   func(TraceEvent)
	recs []*core.ProfileRecorder
	// direct is set by tracedCall just before it calls call for the call it has reported, so that
	// call runs it instead of reporting it again.
	direct bool
	stack  []tracedCallState
	// The exit event being handled: the farthest failure of the call (far, exp), the first entry of
	// the record of recovered errors it left (rec, -1 if none), and whether the errors are the tail of
	// a bounded stream (tail).
	far, rec int
	exp      []ExpID
	tail     bool
}

// emit passes ev to the profiles and to the trace function. A panic there is carried through
// recoverParse as a TracePanic, so that it is not taken for a fault of the parse (a runtime error on
// a bytecode backend is reported as invalid bytecode), and is raised again unchanged.
func (t *Tracer) emit(ev TraceEvent) {
	ev = ev.WithSource(t)
	for _, r := range t.recs {
		r.Record(ev)
	}
	if t.fn != nil {
		t.p.Control.Check()
		t.call(ev)
		t.p.Control.Check()
	}
	for _, r := range t.recs {
		r.Resume()
	}
}

func (t *Tracer) call(ev TraceEvent) {
	defer func() {
		if x := recover(); x != nil {
			if _, ok := x.(Fatal); ok {
				panic(x) // a charged TraceEvent accessor interrupted engine execution
			}
			panic(TracePanic{x})
		}
	}()
	t.fn(ev)
}

// LineCol returns the 1-based line and column of the position pos. In a stream parse, it returns
// 0, 0 for a position (other than 0) in input that has been discarded or not read yet: keeping
// the line starts of discarded input would make the memory of a stream grow with its length.
func (t *Tracer) LineCol(pos int) (line, col int) {
	switch p := t.p; {
	case pos == 0:
		return 1, 1
	case pos < p.Base || pos > p.Loaded():
		return 0, 0
	}
	return t.p.lineCol(pos)
}

// Text returns the input [start, end).
func (t *Tracer) Text(start, end int) string {
	return t.p.Text(start, end)
}

// Failure returns the farthest failure of the exit event being handled, or nil.
func (t *Tracer) Failure() *core.SyntaxError {
	if len(t.exp) == 0 {
		return nil
	}
	return t.p.MakeError(t.far, t.exp)
}

// Recovered returns the errors recovered during the call of the exit event being handled.
func (t *Tracer) Recovered() []*core.SyntaxError {
	p := t.p
	switch {
	case t.tail:
		// A bounded stream has released earlier committed recoveries.
		// Report the same ordered tail, including a successful suffix.
		return p.DiagnosticTail.Preview(p, p.RecoveredErrors())
	case t.rec >= 0 && len(p.Recovered) > t.rec:
		return appendRecErrors(nil, p.Recovered[t.rec:])
	}
	return nil
}

// Retained returns the first position of the input the parse still holds.
func (t *Tracer) Retained() int {
	return t.p.Base
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
}

// SetTrace makes the parse report its rule calls to fn and to profiles (nothing if fn is nil and
// there are no profiles).
func (p *Parser) SetTrace(fn func(TraceEvent), profiles []*core.Profile) {
	if fn == nil && len(profiles) == 0 {
		return
	}
	t := &Tracer{p: p, fn: fn}
	for _, pr := range profiles {
		t.recs = append(t.recs, pr.Recorder())
	}
	p.Tr = t
	p.NoPlain = true
}

// closeTrace ends the recording of the parse's profiles and drops what the tracer refers to, except
// the parser (which its owner resets), since events that callers keep refer to the tracer.
func (t *Tracer) closeTrace() {
	for _, r := range t.recs {
		r.Close()
	}
	*t = Tracer{p: t.p}
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
	t.stack = append(t.stack, tracedCallState{r: r, min: min, start: p.Pos, evals: p.Stats.Evaluated, resumed: p.Resumed, hw: p.Hw, rec: len(p.Recovered)})
	p.Hw = p.Pos
	t.stack[len(t.stack)-1].exp = p.Isolate(p.Pos)
	t.emit(TraceEvent{Kind: TraceEnter, Rule: r.Name, Level: min, Depth: len(t.stack), Pos: p.Pos, Lookahead: p.Silent > 0})
}

// TraceExit reports the end of the innermost call in progress, whose result is ok.
func (p *Parser) TraceExit(ok bool) {
	t := p.Tr
	c := t.stack[len(t.stack)-1]
	all := p.Stats.Evaluated - c.evals
	far, inner := p.Unisolate(c.exp)
	ev := TraceEvent{Kind: TraceExit, Rule: c.r.Name, Level: c.min, Depth: len(t.stack), Pos: c.start,
		Lookahead: p.Silent > 0, Matched: ok, End: c.start, Memo: all == 0, Evals: all - c.nested,
		Examined: p.Hw, ReusedElements: p.Resumed - c.resumed}
	t.far, t.exp, t.rec, t.tail = far, inner, -1, false
	if ok {
		ev.End = p.Pos
		if len(t.stack) == 1 && p.DiagnosticTail.Limit > 0 {
			t.tail = true
		} else {
			t.rec = c.rec
		}
	}
	t.emit(ev)
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
			if p := TraceParser(ev); p != nil {
				p.Control.Check()
			}
		}
		next(ev)
	}
}
