package document

import (
	"zeroparse.dev/zp/internal/runtime/state"
	"zeroparse.dev/zp/pkg/runtime/value"
)

// copyDocumentDiagnostics publishes owned recovered errors without exposing
// memoized error objects or their mutable slice containers. Ordinary failures
// are already freshly constructed by Parser.SyntaxError.
func copyDocumentDiagnostics(p *state.Parser, src value.SyntaxErrors) value.SyntaxErrors {
	if src == nil {
		return nil
	}
	copyStrings := func(src []string) []string {
		if src == nil {
			return nil
		}
		p.SpendValue(len(src))
		out := make([]string, len(src))
		state.CopySpend(p, out, src)
		return out
	}
	p.SpendValue(len(src))
	out := make(value.SyntaxErrors, len(src))
	for i, e := range src {
		p.Spend()
		if e == nil {
			continue
		}
		cloned := new(value.SyntaxError)
		*cloned = *e
		cloned.Expected = copyStrings(e.Expected)
		cloned.Messages = copyStrings(e.Messages)
		out[i] = cloned
	}
	return out
}
