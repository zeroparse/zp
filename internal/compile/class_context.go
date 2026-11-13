package compile

import (
	"strconv"
	"strings"
	"zeroparse.dev/zp/internal/cancel"

	"zeroparse.dev/zp/grammar"
)

// CharClassStringControlled is CharClassString polling control for cancellation; a nil control
// never cancels.
func CharClassStringControlled(e *grammar.CharClass, control *cancel.Control) string {
	if control == nil {
		return CharClassString(e)
	}
	var b strings.Builder
	b.WriteString("[")
	if e.Negated {
		b.WriteString("^")
	}
	esc := func(r rune) string {
		switch r {
		case '-', '[', ']', '^', '\\':
			return `\` + string(r)
		}
		s := strconv.QuoteRune(r)
		return s[1 : len(s)-1]
	}
	for _, rg := range e.Ranges {
		control.Poll()
		b.WriteString(esc(rg.Lo))
		if rg.Hi != rg.Lo {
			b.WriteString("-" + esc(rg.Hi))
		}
	}
	b.WriteString("]")
	return b.String()
}
