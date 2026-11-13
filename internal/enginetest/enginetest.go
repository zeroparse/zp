// Package enginetest compiles and runs grammars with the engine for the tests of the packages that
// compare with it or build on a compiled engine.Program: the generators, the output schema, the
// value model and the bytecode. The tests of package engine itself cannot import it (it imports
// the engine); they keep their own helpers of the same names.
package enginetest

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	"zeroparse.dev/zp/internal/engine"
	"zeroparse.dev/zp/internal/gentest"
	ischema "zeroparse.dev/zp/internal/schema"
	"zeroparse.dev/zp/internal/syntax"
	"zeroparse.dev/zp/schema"
)

// UseEngine makes gentest compile the grammars that tests generate parsers for with the engine,
// and compare the generated parsers with it, as package engine does. Call it from an init function of
// the test package.
func UseEngine() {
	gentest.Compile = CompileForGenerator
	gentest.Reference = Reference
}

// Main runs the tests of a package that builds generated programs: with the Go flags and the
// private build cache of gentest, and failing when a test builds generated fixtures without
// choosing their form. Call it from TestMain.
func Main(m *testing.M) {
	// Tests build generated programs in fresh temporary directories, with a build cache private
	// to the test process (SetGoCache) so that their builds, which no later run reuses, do not
	// accumulate in the shared cache. -trimpath makes the entries independent of the directory,
	// so the tests of the package share the builds of the standard library and of this module;
	// commands that set GOFLAGS add it themselves. -mod=mod lets the temporary modules that
	// replace this module resolve it without a go.sum.
	gentest.SetGoFlags()
	removeGoCache := gentest.SetGoCache()
	code := m.Run()
	removeGoCache()
	if names := gentest.UnwrappedFixtureTests(); len(names) > 0 {
		fmt.Fprintf(os.Stderr, "FAIL: tests build generated fixtures without choosing their form: %s\nWrap the test body in gentest.EachMode, or call gentest.NeedsBudgetControls, or pin the form with gentest.PinOrdinary\n", strings.Join(names, ", "))
		code = 1
	}
	os.Exit(code)
}

// Parse parses ZeroParse source for a test whose subject is not start rules (compiletest.Parse).
func Parse(src string) (*grammar.Grammar, error) { return compiletest.Parse(src) }

// MustParse is Parse that panics on a syntax error.
func MustParse(src string) *grammar.Grammar {
	g, err := Parse(src)
	if err != nil {
		panic(err)
	}
	return g
}

// MustCompile compiles ZeroParse source with source information, so that results show rule names
// and capture labels.
func MustCompile(t testing.TB, src string, opts ...compile.Options) *engine.Program {
	t.Helper()
	var o compile.Options
	if len(opts) > 0 {
		o = opts[0]
	}
	o.SourceInfo = true
	return CompileBare(t, src, o)
}

// CompileBare compiles ZeroParse source with the options as given: without source information
// unless they request it.
func CompileBare(t testing.TB, src string, opts ...compile.Options) *engine.Program {
	t.Helper()
	g, err := Parse(src)
	if err != nil {
		t.Fatalf("syntax error: %v\n%s", err, src)
	}
	var o compile.Options
	if len(opts) > 0 {
		o = opts[0]
	}
	prog, err := engine.Compile(g, o)
	if err != nil {
		t.Fatalf("compile error: %v\n%s", err, src)
	}
	return prog
}

// CompileStarts compiles source as written, without marking start rules for the test.
func CompileStarts(t testing.TB, src string) (*engine.Program, error) {
	t.Helper()
	g, err := syntax.Parse(src)
	if err != nil {
		t.Fatalf("syntax error: %v\n%s", err, src)
	}
	return engine.Compile(g, compile.Options{})
}

// CompileMessage compiles src and returns the error message, or "" if it compiles.
func CompileMessage(t testing.TB, src string, opts ...compile.Options) string {
	t.Helper()
	g, err := Parse(src)
	if err != nil {
		t.Fatalf("%q: %v", src, err)
	}
	var o compile.Options
	if len(opts) > 0 {
		o = opts[0]
	}
	_, err = engine.Compile(g, o)
	if err == nil {
		return ""
	}
	return err.Error()
}

// Reference compiles src with the engine for the comparisons of generated parsers with it. Its
// parses record expectations, as the generated parsers' do.
func Reference(t *testing.T, src string, opts compile.Options) gentest.ReferenceParse {
	t.Helper()
	prog := CompileBare(t, src, opts)
	return func(input string, bytes, recognize bool) (*core.Node, error) {
		unit := core.CodePoints
		if bytes {
			unit = core.Bytes
		}
		return prog.ParseWith("main", input, engine.ParseOptions{Unit: unit, Recognize: recognize, Expectations: true})
	}
}

// CompileForGenerator compiles g for the generators, as the compileForGenerator of package
// codegen does.
func CompileForGenerator(g *grammar.Grammar, opts compile.Options) (*compile.Program, func() (*compile.Program, error), error) {
	prog, err := engine.Compile(g, opts)
	if err != nil {
		return nil, nil, err
	}
	recognizer := func() (*compile.Program, error) {
		rec, err := prog.Recognizer()
		if err != nil {
			return nil, err
		}
		return rec.Static(), nil
	}
	return prog.Static(), recognizer, nil
}

// OutputSchema exports the output schema of p for start, as Parser.OutputSchema does.
func OutputSchema(p *engine.Program, start string, allRules bool) (*schema.Output, error) {
	sp, err := p.SchemaProgram(start)
	if err != nil {
		return nil, err
	}
	return ischema.Export(sp, start, allRules)
}

// Result parses input from main with o and returns the result, or the error prefixed "error: ".
func Result(prog *engine.Program, input string, o engine.ParseOptions) string {
	n, err := prog.ParseWith("main", input, o)
	if err != nil {
		return "error: " + err.Error()
	}
	return n.String()
}
