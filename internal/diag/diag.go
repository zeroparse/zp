// Package diag holds the positioned errors that the static work on a grammar reports: compilation
// and the analyses that check the grammar.
package diag

import (
	"fmt"
	"strings"

	"zeroparse.dev/zp/grammar"
)

// Error is an error at a position in the grammar source.
type Error struct {
	Pos grammar.Pos
	Msg string
}

// Error returns the message, prefixed with the line and column when the position is known.
func (e *Error) Error() string {
	if e.Pos.IsValid() {
		return fmt.Sprintf("%d:%d: %s", e.Pos.Line, e.Pos.Col, e.Msg)
	}
	return e.Msg
}

// ErrorList is a list of errors.
type ErrorList []*Error

// Error returns the messages of the errors, one per line.
func (l ErrorList) Error() string {
	msgs := make([]string, len(l))
	for i, e := range l {
		msgs[i] = e.Error()
	}
	return strings.Join(msgs, "\n")
}
