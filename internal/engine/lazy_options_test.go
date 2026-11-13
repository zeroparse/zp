package engine

import (
	"reflect"
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/engine/core"
	"zeroparse.dev/zp/internal/gentest"
	"zeroparse.dev/zp/runtime/parse"
)

// lazyOverrides are the options that a lazily built program sets itself, by the field of
// compile.Options, for the controlled graph and for the recognizer. Every other option must be the
// program's own.
var lazyOverrides = map[string]struct{ controlled, recognizer *bool }{
	"ControlledDispatch": {controlled: ptr(true), recognizer: ptr(false)},
	"Recognize":          {recognizer: ptr(true)},
	"Recognizer":         {recognizer: ptr(true)},
}

func ptr[T any](v T) *T { return &v }

// TestLazyProgramsCarryEveryOption compiles a program with each compile option on in turn, builds
// its controlled closure graph and its recognizer, and checks that each option reaches both, except
// the three that the build sets itself. A new option fails the test until it is carried or listed in
// lazyOverrides.
func TestLazyProgramsCarryEveryOption(t *testing.T) {
	if !core.BudgetControlsEnabled {
		t.Skip("budget controls are unavailable in the reference build")
	}
	typ := reflect.TypeFor[compile.Options]()
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		if typ.Field(i).Type.Kind() != reflect.Bool {
			t.Fatalf("option %s is not a bool: the test must say how to set it", name)
		}
		t.Run(name, func(t *testing.T) {
			var o compile.Options
			reflect.ValueOf(&o).Elem().FieldByName(name).SetBool(true)
			o.LeftRecursionBindings = true // the grammar below needs it
			prog := mustCompile(t, gentest.LRBindingSource, o)
			if _, err := prog.ParseWith("main", "x", ParseOptions{Backend: Closure, Limits: parse.Limits{Compute: 1 << 30}}); err != nil {
				t.Fatal(err)
			}
			rec, err := prog.Recognizer()
			if err != nil {
				t.Fatal(err)
			}
			if prog.budgetProgram == nil {
				t.Fatal("the controlled graph was not built")
			}
			for _, c := range []struct {
				what string
				got  compile.Options
				pick func(struct{ controlled, recognizer *bool }) *bool
			}{
				{"controlled graph", prog.budgetProgram.Opts, func(s struct{ controlled, recognizer *bool }) *bool { return s.controlled }},
				{"recognizer", rec.Opts, func(s struct{ controlled, recognizer *bool }) *bool { return s.recognizer }},
			} {
				got, want := reflect.ValueOf(c.got), reflect.ValueOf(prog.Opts)
				for j := 0; j < typ.NumField(); j++ {
					f := typ.Field(j).Name
					expected := want.Field(j).Bool()
					if ov, ok := lazyOverrides[f]; ok && c.pick(ov) != nil {
						expected = *c.pick(ov)
					}
					if got.Field(j).Bool() != expected {
						t.Errorf("%s: option %s is %v, want %v (the program's is %v)", c.what, f, got.Field(j).Bool(), expected, want.Field(j).Bool())
					}
				}
			}
		})
	}
	for f := range lazyOverrides {
		if _, ok := typ.FieldByName(f); !ok {
			t.Errorf("lazyOverrides names %s, which is not an option", f)
		}
	}
}

// TestLeftRecursionBindingsOnTheLazyClosureGraph parses a left-recursive rule that a binding
// re-enters with other variable values, which compiles only with LeftRecursionBindings, on every
// backend, with the default budget (the controlled closure graph) and through the recognizer. The
// results agree with the bytecode backends and with the program's own closure graph.
func TestLeftRecursionBindingsOnTheLazyClosureGraph(t *testing.T) {
	prog := mustCompile(t, gentest.LRBindingSource, compile.Options{LeftRecursionBindings: true})
	rec, err := prog.Recognizer()
	if err != nil {
		t.Fatal(err)
	}
	for _, in := range gentest.LRBindingInputs {
		want := gentest.ResultJSON(prog.ParseWith("main", in, ParseOptions{Backend: Bytecode}))
		for _, b := range []Backend{Closure, BytecodeIterative} {
			if got := gentest.ResultJSON(prog.ParseWith("main", in, ParseOptions{Backend: b})); got != want {
				t.Errorf("%s: input %q: %s, want %s", b, in, got, want)
			}
		}
		_, werr := prog.ParseWith("main", in, ParseOptions{Backend: Bytecode})
		for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
			_, err := rec.ParseWith("main", in, ParseOptions{Backend: b})
			if (err == nil) != (werr == nil) {
				t.Errorf("recognizer, %s: input %q: error %v, the parse's is %v", b, in, err, werr)
			}
		}
	}
}
