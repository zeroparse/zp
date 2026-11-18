package input

import (
	"fmt"
)

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
