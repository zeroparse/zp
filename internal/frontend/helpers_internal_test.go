package frontend

import (
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/frontend/desugar"
	"zeroparse.dev/zp/internal/frontend/parser"
	"zeroparse.dev/zp/internal/testing/starttest"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// The internal tests cannot import compiletest, which imports this package; these are its Parse
// and Compile.

func parseMarked(src string) (*ast.Grammar, error) {
	g, err := parser.Parse(src)
	return starttest.MarkAll(g), err
}

func compileGrammar(g *ast.Grammar, opts compile.Options) (*compile.Program, error) {
	names := desugar.InlineNames{}
	p, err := Check(g, opts, nil, names, nil)
	if err != nil {
		return nil, names.Rewrite(err)
	}
	return p, nil
}
