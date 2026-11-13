package parse

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
	return expectedMessage(e.Expected)
}

// expectedInMessage is the number of expected entries that the message of a syntax error lists.
// A longer list is cut after them and ends with the count of the rest; SyntaxError.Expected keeps
// every entry. The parsers charge the message they build by the same rule.
const expectedInMessage = 10

// expectedMessage describes the expected entries of a syntax error without messages.
func expectedMessage(expected []string) string {
	if len(expected) == 0 {
		return "syntax error"
	}
	shown, rest := expected, 0
	if len(expected) > expectedInMessage {
		shown, rest = expected[:expectedInMessage], len(expected)-expectedInMessage
	}
	msg := "syntax error: expected " + strings.Join(shown, ", ")
	if rest > 0 {
		msg += fmt.Sprintf(", ... and %d more", rest)
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
