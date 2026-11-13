package document

import (
	"zeroparse.dev/zp/internal/engine/core"
	"zeroparse.dev/zp/runtime/parse"
)

// copyDocumentDiagnostics publishes owned recovered errors without exposing
// memoized error objects or their mutable slice containers. Ordinary failures
// are already freshly constructed by Parser.SyntaxError.
func copyDocumentDiagnostics(p *core.Parser, src parse.SyntaxErrors) parse.SyntaxErrors {
	if src == nil {
		return nil
	}
	copyStrings := func(src []string) []string {
		if src == nil {
			return nil
		}
		p.SpendValue(len(src))
		out := make([]string, len(src))
		core.CopySpend(p, out, src)
		return out
	}
	p.SpendValue(len(src))
	out := make(parse.SyntaxErrors, len(src))
	for i, e := range src {
		p.Spend()
		if e == nil {
			continue
		}
		cloned := new(parse.SyntaxError)
		*cloned = *e
		cloned.Expected = copyStrings(e.Expected)
		cloned.Messages = copyStrings(e.Messages)
		out[i] = cloned
	}
	return out
}
