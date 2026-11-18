package engine_test

import (
	"zeroparse.dev/zp/internal/runtime/engine"
	"zeroparse.dev/zp/internal/testing/enginetest"
	"zeroparse.dev/zp/internal/testing/starttest"
)

// compileTestSource compiles ZeroParse source as enginetest.CompileSource does, for a test whose subject is
// not start rules: unless the source has declaration attributes, every rule becomes a start rule,
// the first one the default.
func compileTestSource(src, start string, opts ...enginetest.CompileOption) (*engine.Parser, error) {
	g, err := enginetest.ParseGrammar(src)
	if err != nil {
		return nil, err
	}
	return enginetest.Compile(starttest.MarkAll(g), start, opts...)
}
