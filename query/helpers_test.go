package query_test

import (
	"testing"

	"zeroparse.dev/zp/engine"
	"zeroparse.dev/zp/internal/starttest"
	"zeroparse.dev/zp/query"
	"zeroparse.dev/zp/schema"
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

// mustCompile compiles ZeroParse source with the start rule main.
func mustCompile(t testing.TB, src string, opts ...engine.CompileOption) *engine.Parser {
	t.Helper()
	p, err := compileTestSource(src, "main", opts...)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// mustSchema exports the output schema of p.
func mustSchema(t testing.TB, p *engine.Parser, opts ...engine.SchemaOption) *schema.Output {
	t.Helper()
	s, err := p.OutputSchema(opts...)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func field(name string, p query.Pattern) query.FieldPattern {
	return query.FieldPattern{Name: name, Pattern: p}
}

// textStruct matches a value of the text struct type name, such as Int or Name.
func textStruct(name string) query.Pattern {
	return query.Pattern{Type: name, Fields: []query.FieldPattern{{Name: "Text", Pattern: query.Pattern{Category: "string"}}}}
}
