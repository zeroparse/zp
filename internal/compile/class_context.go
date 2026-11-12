package compile

import (
	"strconv"
	"strings"
	"zeroparse.dev/zp/internal/cancel"

	"zeroparse.dev/zp/grammar"
)

// FirstTerminalControlled is firstTerminal polling control. Ordinary compilation and code
// generation keep their existing construction path. Controlled construction polls without
// capturing the compile context in the returned parser-time matchers.
func FirstTerminalControlled(prog *Program, e grammar.Expr, depth int, seen map[string]bool, control *cancel.Control) (grammar.Expr, int) {
	if control == nil {
		return firstTerminal(prog, e, depth, seen)
	}
	control.Poll()
	switch e := e.(type) {
	case *grammar.Literal:
		if e.Value != "" {
			return e, depth
		}
	case *grammar.CharClass:
		return e, depth
	case *grammar.Seq:
		if len(e.Items) > 0 {
			return FirstTerminalControlled(prog, e.Items[0], depth, seen, control)
		}
	case *grammar.Capture:
		return FirstTerminalControlled(prog, e.Expr, depth, seen, control)
	case *grammar.Atomic:
		return FirstTerminalControlled(prog, e.Expr, depth, seen, control)
	case *grammar.Discard:
		return FirstTerminalControlled(prog, e.Expr, depth, seen, control)
	case *grammar.Ref:
		r := prog.ByName[e.Name]
		if r == nil || r.Leader || seen[e.Name] || r.Def == nil {
			break
		}
		if _, operatorsRule := r.Def.Expr.(*grammar.OperatorsRule); operatorsRule {
			break
		}
		seen[e.Name] = true
		return FirstTerminalControlled(prog, r.Def.Expr, depth+r.EntryDepth(), seen, control)
	}
	return nil, 0
}

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
