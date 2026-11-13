package core

import (
	"fmt"
	"strings"
)

// SyntaxError reports that the input did not match the grammar.
type SyntaxError struct {
	Pos  int // position, in the position unit of the parse
	Line int // 1-based line
	Col  int // 1-based column, in the position unit of the parse
	// Expected lists every entry that was expected at Pos: names, then literals, then classes,
	// each in byte order. Message shows at most the first ten of them.
	Expected []string
	Messages []string // messages given by @error
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("%d:%d: %s", e.Line, e.Col, e.Message())
}

// Message describes the error without its position: the messages given by @error, or else the
// expected entries.
func (e *SyntaxError) Message() string {
	if len(e.Messages) > 0 {
		return strings.Join(e.Messages, "; ")
	}
	return e.expectedMessage()
}

// expectedInMessage is the number of expected entries that the message of a syntax error lists.
const expectedInMessage = 10

// ExpectedInMessage returns the expected entries that Message lists when e has no messages from
// @error, and the number of entries after them that it leaves out. Message lists at most the
// first ten entries and ends a longer list with the count of the rest; Expected keeps every entry.
// Parsers that charge the message they build use it to build the same text.
func (e *SyntaxError) ExpectedInMessage() (shown []string, omitted int) {
	if len(e.Expected) > expectedInMessage {
		return e.Expected[:expectedInMessage], len(e.Expected) - expectedInMessage
	}
	return e.Expected, 0
}

// expectedMessage describes the expected entries of e, which has no messages.
func (e *SyntaxError) expectedMessage() string {
	if len(e.Expected) == 0 {
		return "syntax error"
	}
	shown, omitted := e.ExpectedInMessage()
	msg := "syntax error: expected " + strings.Join(shown, ", ")
	if omitted > 0 {
		msg += fmt.Sprintf(", ... and %d more", omitted)
	}
	return msg
}

// SyntaxErrors is a list of syntax errors, including those recovered by error recovery.
type SyntaxErrors []*SyntaxError

func (l SyntaxErrors) Error() string {
	msgs := make([]string, len(l))
	for i, e := range l {
		msgs[i] = e.Error()
	}
	return strings.Join(msgs, "\n")
}

// TruncatedSyntaxErrors is the incomplete result of a stream parsed with a positive diagnostic
// limit. Errors is the retained tail in occurrence order. Saturated counts are lower bounds;
// truncation and the tail remain exact.
type TruncatedSyntaxErrors struct {
	Errors          SyntaxErrors
	Total           uint64
	Dropped         uint64
	CountsSaturated bool
}

func (e *TruncatedSyntaxErrors) Error() string {
	return fmt.Sprintf("recovered syntax errors truncated (total %d, dropped %d):\n%s", e.Total, e.Dropped, e.Errors)
}

// Unwrap exposes the retained errors to errors.As as SyntaxErrors.
func (e *TruncatedSyntaxErrors) Unwrap() error { return e.Errors }

// MissingTokenLimitError reports that a parse exhausted its budget of #missing insertions.
// Position is in the position unit of the parse. Exhaustion aborts parsing, including when an
// alternative that used the budget would otherwise be abandoned.
type MissingTokenLimitError struct {
	Limit, Position int
}

func (e *MissingTokenLimitError) Error() string {
	return fmt.Sprintf("missing token insertion limit %d exceeded at position %d", e.Limit, e.Position)
}
