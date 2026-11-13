package gen_test

import (
	"zeroparse.dev/zp/engine"
	"zeroparse.dev/zp/internal/starttest"
)

// compileTestSource compiles ZeroParse source as engine.CompileSource does, for a test whose subject is
// not start rules: unless the source has declaration attributes, every rule becomes a start rule,
// the first one the default.
func compileTestSource(src, start string, opts ...engine.CompileOption) (*engine.Parser, error) {
	g, err := engine.ParseGrammar(src)
	if err != nil {
		return nil, err
	}
	return engine.Compile(starttest.MarkAll(g), start, opts...)
}
