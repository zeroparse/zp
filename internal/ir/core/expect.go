package core

import (
	"strconv"
	"strings"

	"zeroparse.dev/zp/grammar"
)

// FixedDescs are the descriptions of the fixed expectations of the runtime, in ExpID order: Any
// and the anchors, which name no written text.
var FixedDescs = []string{"any character", "beginning of input", "end of input", "beginning of line", "end of line"}

// LiteralDesc is the expectation text of a literal, as written: a case-insensitive literal keeps its
// suffix i.
func LiteralDesc(e *grammar.Literal) string {
	if e.IgnoreCase {
		return strconv.Quote(e.Value) + "i"
	}
	return strconv.Quote(e.Value)
}

// CharClassString returns the character class as grammar syntax writes it, which names the class
// in expectations.
func CharClassString(e *grammar.CharClass) string {
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
		b.WriteString(esc(rg.Lo))
		if rg.Hi != rg.Lo {
			b.WriteString("-" + esc(rg.Hi))
		}
	}
	b.WriteString("]")
	return b.String()
}
