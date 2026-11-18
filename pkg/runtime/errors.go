package runtime

import (
	"fmt"
)

// MissingTokenLimitError reports that a parse exhausted its budget of #missing insertions.
// Position is in the position unit of the parse. Exhaustion aborts parsing, including when an
// alternative that used the budget would otherwise be abandoned.
type MissingTokenLimitError struct {
	Limit, Position int
}

func (e *MissingTokenLimitError) Error() string {
	return fmt.Sprintf("missing token insertion limit %d exceeded at position %d", e.Limit, e.Position)
}
