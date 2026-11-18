package frontend

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/frontend/desugar"
	"zeroparse.dev/zp/internal/frontend/parser"
	"zeroparse.dev/zp/internal/testing/starttest"
)

// The internal tests cannot import compiletest, which imports this package; these are its Parse
// and Compile.

func parseMarked(src string) (*grammar.Grammar, error) {
	g, err := parser.Parse(src)
	return starttest.MarkAll(g), err
}

func compileGrammar(g *grammar.Grammar, opts compile.Options) (*compile.Program, error) {
	names := desugar.InlineNames{}
	p, err := Check(g, opts, nil, names, nil)
	if err != nil {
		return nil, names.Rewrite(err)
	}
	return p, nil
}
