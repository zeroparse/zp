package engine

import (
	"reflect"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/engine/state"
	"zeroparse.dev/zp/internal/testing/codegentest"
)

// lazyOverride gives, for an option the build sets itself, the value of the lazily built program:
// nil keeps the program's own.
type lazyOverride struct{ controlled, recognizer, insertion any }

// lazyOverrides are the options that a lazily built program sets itself, by the field of
// compile.Options. Every other option must be the program's own.
var lazyOverrides = map[string]lazyOverride{
	"Recognition": {recognizer: compile.Recognize},
}

// recognitionMode returns the compile mode of a program that recognizes (RecognizeKeepStreamValues) or builds
// values.
func recognitionMode(recognize bool) compile.Recognition {
	if recognize {
		return compile.RecognizeKeepStreamValues
	}
	return compile.Evaluate
}

// turnOn sets the option of o named name to a value other than its zero value.
func turnOn(t *testing.T, o *compile.Options, name string) {
	f := reflect.ValueOf(o).Elem().FieldByName(name)
	switch f.Kind() {
	case reflect.Bool:
		f.SetBool(true)
	case reflect.Uint8:
		f.SetUint(1)
	default:
		t.Fatalf("option %s is a %s: the test must say how to set it", name, f.Kind())
	}
}

// TestLazyProgramsCarryEveryOption compiles a program with each compile option on in turn, builds
// its controlled closure graph, its recognizer and its program of missing-token insertion, and
// checks that each option reaches all three, except those that the build sets itself. A new option
// fails the test until it is carried or listed in lazyOverrides.
func TestLazyProgramsCarryEveryOption(t *testing.T) {
	if !state.BudgetControlsEnabled {
		t.Skip("budget controls are unavailable in the reference build")
	}
	typ := reflect.TypeFor[compile.Options]()
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		t.Run(name, func(t *testing.T) {
			var o compile.Options
			turnOn(t, &o, name)
			o.LeftRecursionBindings = true // the grammar below needs it
			prog := mustCompile(t, codegentest.LRBindingSource, o)
			if _, err := prog.ParseWith("main", "x", ParseOptions{Backend: Closure, Limits: core.Limits{Compute: 1 << 30}}); err != nil {
				t.Fatal(err)
			}
			rec, err := prog.Recognizer()
			if err != nil {
				t.Fatal(err)
			}
			if prog.budgetProgram == nil {
				t.Fatal("the controlled graph was not built")
			}
			// The insertion program is built from a grammar that has an insertion.
			withIns := mustCompile(t, insertionCycle, o)
			withIns.ParseWith("main", "yx", ParseOptions{MissingTokens: 1000}) // only builds the program
			if withIns.ins == nil {
				t.Fatal("the insertion program was not built")
			}
			for _, c := range []struct {
				what      string
				from, got compile.Options
				pick      func(lazyOverride) any
			}{
				{"controlled graph", prog.Opts, prog.budgetProgram.Opts, func(s lazyOverride) any { return s.controlled }},
				{"recognizer", prog.Opts, rec.Opts, func(s lazyOverride) any { return s.recognizer }},
				{"insertion program", withIns.Opts, withIns.ins.Opts, func(s lazyOverride) any { return s.insertion }},
			} {
				got, want := reflect.ValueOf(c.got), reflect.ValueOf(c.from)
				for j := 0; j < typ.NumField(); j++ {
					f := typ.Field(j).Name
					expected := want.Field(j).Interface()
					if ov, ok := lazyOverrides[f]; ok && c.pick(ov) != nil {
						expected = c.pick(ov)
					}
					if got.Field(j).Interface() != expected {
						t.Errorf("%s: option %s is %v, want %v (the program's is %v)", c.what, f, got.Field(j).Interface(), expected, want.Field(j).Interface())
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
	prog := mustCompile(t, codegentest.LRBindingSource, compile.Options{LeftRecursionBindings: true})
	rec, err := prog.Recognizer()
	if err != nil {
		t.Fatal(err)
	}
	for _, in := range codegentest.LRBindingInputs {
		want := codegentest.ResultJSON(prog.ParseWith("main", in, ParseOptions{Backend: Bytecode}))
		for _, b := range []Backend{Closure, BytecodeIterative} {
			if got := codegentest.ResultJSON(prog.ParseWith("main", in, ParseOptions{Backend: b})); got != want {
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
