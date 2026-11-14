package core

import "fmt"

// TraceKind tells what a TraceEvent reports: the start of a rule call, its end, or a call that the
// parser skipped.
type TraceKind uint8

const (
	// TraceEnter is reported when a rule is called, before the memo is consulted.
	TraceEnter TraceKind = iota
	// TraceExit is reported when the call returns.
	TraceExit
	// TraceSkipped is reported, unpaired, for a call that the parser did not make because it could
	// not match (SkipReason tells why). Producers that do not identify such calls never report it.
	TraceSkipped
)

// String returns "enter", "exit" or "skipped".
func (k TraceKind) String() string {
	switch k {
	case TraceEnter:
		return "enter"
	case TraceExit:
		return "exit"
	case TraceSkipped:
		return "skipped"
	}
	return fmt.Sprintf("TraceKind(%d)", uint8(k))
}

// MarshalText returns the String form of k.
func (k TraceKind) MarshalText() ([]byte, error) {
	if k > TraceSkipped {
		return nil, fmt.Errorf("core: invalid TraceKind %d", uint8(k))
	}
	return []byte(k.String()), nil
}

// UnmarshalText sets k from its String form.
func (k *TraceKind) UnmarshalText(b []byte) error {
	switch string(b) {
	case "enter":
		*k = TraceEnter
	case "exit":
		*k = TraceExit
	case "skipped":
		*k = TraceSkipped
	default:
		return fmt.Errorf("core: invalid TraceKind %q", b)
	}
	return nil
}

// SkipReason tells why a parser skipped a call (TraceSkipped).
type SkipReason string

// SkipFirstCharacter is the reason of a call skipped because the next character cannot start the
// rule.
const SkipFirstCharacter SkipReason = "first-character"

// TraceEvent describes the start or the end of a rule call, or a skipped call. Enter and exit
// events nest: every TraceEnter is followed, after the events of the calls the rule makes, by its
// TraceExit, unless the parse is aborted (by a runtime error, a limit, an error of a stream's
// reader or emit function, or a panic of the trace function, which propagates unchanged).
//
// The fields can be kept. The methods LineCol, Text, Failure and Recovered read the state of the
// parser that reported the event: call them only from the trace function, while it handles the
// event. A producer that does not provide that state (Source) makes them return zero values.
type TraceEvent struct {
	Kind TraceKind `json:"kind"`
	// Rule is the name of the rule called.
	Rule string `json:"rule"`
	// Level is the binding level the rule was called with: for an Operators Rule rule called as
	// e:level, the operators that bind no tighter than that level are left to the caller. It is 0
	// for calls without a level.
	Level int `json:"level"`
	// Depth is the nesting depth of the call: 1 for the start rule.
	Depth int `json:"depth"`
	// Pos is the position at which the call starts.
	Pos int `json:"pos"`
	// Lookahead reports that the call is inside a lookahead (& or !), where failures record no
	// expectations.
	Lookahead bool `json:"lookahead"`

	// The fields below are set on TraceExit only.

	// Matched reports whether the rule matched.
	Matched bool `json:"matched"`
	// End is the position after the match (Pos if the rule failed).
	End int `json:"end"`
	// Memo reports that the result came from the memo, without evaluating the rule body.
	Memo bool `json:"memo"`
	// Evals is the number of times this call evaluated the rule body: 0 for a result from the
	// memo, 1 usually, and one more per step for a left-recursive rule whose match grows. Bodies of
	// nested calls are not included.
	Evals int `json:"evals"`
	// ReusedElements counts the repetition elements of a Document that this call reused, nested
	// calls included. It is 0 for a parser that does not resume repetitions.
	ReusedElements int `json:"reusedElements"`
	// Examined is the exclusive end of all input the call examined: looking at a character to
	// reject it counts, as do lookaheads. A result from the memo examined what the call that
	// computed it did.
	Examined int `json:"examined"`

	// SkipReason is set on TraceSkipped only.
	SkipReason SkipReason `json:"skipReason"`

	src TraceSource
}

// TraceSource is the parser state behind the events of a parse. It is implemented by the parsers
// that report events (see WithSource), not by their users. The methods are valid only while the
// trace function handles an event of the parse; Failure and Recovered describe that event.
type TraceSource interface {
	// LineCol returns the 1-based line and column of pos, or 0, 0 if the position is not
	// available.
	LineCol(pos int) (line, col int)
	// Text returns the input [start, end), or "" if it is not available.
	Text(start, end int) string
	// Failure returns the farthest failure recorded during the call of the current exit event.
	Failure() *SyntaxError
	// Recovered returns the errors recovered during the call of the current exit event.
	Recovered() []*SyntaxError
	// Retained returns the first position of the input the parser still holds: positions before
	// it are not parsed again.
	Retained() int
}

// WithSource returns e with the parser state src behind its methods. Parsers use it to report
// events.
func (e TraceEvent) WithSource(src TraceSource) TraceEvent {
	e.src = src
	return e
}

// Source returns the parser state that WithSource attached to e, or nil.
func (e TraceEvent) Source() TraceSource {
	return e.src
}

// LineCol returns the 1-based line and column of the position pos, or 0, 0 when the position is
// unknown: when the parser does not provide it, or, in a stream parse, for a position (other than
// 0) in input that has been discarded or not read yet.
func (e TraceEvent) LineCol(pos int) (line, col int) {
	if e.src == nil {
		return 0, 0
	}
	return e.src.LineCol(pos)
}

// Text returns the input the call matched: empty for an event other than the exit of a call that
// matched, and for input that a stream parse has discarded.
func (e TraceEvent) Text() string {
	if e.src == nil || e.Kind != TraceExit || !e.Matched {
		return ""
	}
	return e.src.Text(e.Pos, e.End)
}

// Failure returns, for an exit event, the farthest failure recorded during the call: the position
// and what was expected there, as a parse that failed there would report it. It is nil when the
// call recorded no expectation (inside a lookahead, or when nothing failed). A call that matched
// can have one too: the input after its match was tried and rejected.
func (e TraceEvent) Failure() *SyntaxError {
	if e.src == nil || e.Kind != TraceExit {
		return nil
	}
	return e.src.Failure()
}

// Recovered returns, for an exit event of a call that matched, the syntax errors that #recover
// recovered from during the call, in nested calls too, or taken from the memo with the call's
// result. The expectations of a recovered error are not part of Failure: recovering removes them
// from the record of the call that contains the #recover.
func (e TraceEvent) Recovered() []*SyntaxError {
	if e.src == nil || e.Kind != TraceExit || !e.Matched {
		return nil
	}
	return e.src.Recovered()
}

// Location is a position with its line and column.
type Location struct {
	Pos  int `json:"pos"`
	Line int `json:"line"`
	Col  int `json:"col"`
}

// String returns line:col, or "position N" when the line is unknown (Line is 0; see
// TraceEvent.LineCol).
func (l Location) String() string {
	if l.Line == 0 {
		return fmt.Sprintf("position %d", l.Pos)
	}
	return fmt.Sprintf("%d:%d", l.Line, l.Col)
}

// CallSite is where a rule was called: the position and the binding level of the call.
type CallSite struct {
	Location
	Level int `json:"level"`
}
