package frontend_test

import (
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	"zeroparse.dev/zp/internal/factstest"
	"zeroparse.dev/zp/internal/frontend"
	"zeroparse.dev/zp/internal/ir/coreir"
	"zeroparse.dev/zp/internal/ir/coreir/build"
	"zeroparse.dev/zp/internal/ir/coreir/facts"
	"zeroparse.dev/zp/internal/syntax"
)

// variants are the compiler options that change the Core IR of a grammar.
var variants = []struct {
	name string
	opts compile.Options
}{
	{"parse", compile.Options{}},
	{"recognize", compile.Options{Recognize: true}},
	{"no projections", compile.Options{NoProjections: true}},
	{"default value", compile.Options{DefaultValue: true}},
}

// TestRunMatchesCompile requires, for every corpus grammar and variant, that Run's module prints as
// the module built directly from the compiled program, that its facts are the module's facts and
// that it has no fact error.
func TestRunMatchesCompile(t *testing.T) {
	t.Parallel()
	for _, path := range factstest.Corpus {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			g, err := compiletest.Parse(factstest.Read(t, path))
			if err != nil {
				t.Fatal(err)
			}
			for _, v := range variants {
				prog, err := compiletest.Compile(g, v.opts)
				if err != nil {
					continue // the grammar does not compile with this variant
				}
				want, err := build.CoreFrom(prog, nil)
				if err != nil {
					t.Fatal(err)
				}
				facts.Compute(want, nil) // Run's module has its facts set
				res, err := frontend.Run(g, v.opts, nil)
				if err != nil {
					t.Fatalf("%s: %v", v.name, err)
				}
				if got, want := coreir.Print(res.Module, true), coreir.Print(want, true); got != want {
					t.Errorf("%s: module differs:\n%s\nwant\n%s", v.name, got, want)
				}
				if len(res.Facts.Rules) != len(res.Module.Rules) {
					t.Errorf("%s: facts for %d rules, module has %d", v.name, len(res.Facts.Rules), len(res.Module.Rules))
				}
				if errs := res.Facts.Errors(res.Module, true); len(errs) != 0 {
					t.Errorf("%s: fact errors %v", v.name, errs)
				}
			}
		})
	}
}

// TestRunReportsCompileErrors requires Run to return the compiler's error list as is.
func TestRunReportsCompileErrors(t *testing.T) {
	t.Parallel()
	g, err := syntax.Parse("type T = { A string }\n@start\nrule main = t $$\nrule t = \"a\" => T{A: 1}")
	if err != nil {
		t.Fatal(err)
	}
	_, want := compiletest.Compile(g, compile.Options{})
	if want == nil {
		t.Fatal("the grammar compiles")
	}
	res, got := frontend.Run(g, compile.Options{}, nil)
	if res != nil || got == nil || got.Error() != want.Error() {
		t.Fatalf("Run = %v, %v; want the error %v", res, got, want)
	}
}
