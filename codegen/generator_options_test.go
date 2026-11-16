package codegen

import (
	"reflect"
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/testing/codegentest"
	"zeroparse.dev/zp/internal/testing/compiletest"
)

// TestGeneratorRecognizerCarriesEveryOption compiles a grammar for the generators with each
// compile option on in turn and checks that the recognizer program, built on first use, has the
// same options except Recognition, which the recognizer sets. A new option fails the test until it
// is carried or listed here.
func TestGeneratorRecognizerCarriesEveryOption(t *testing.T) {
	g, err := compiletest.Parse(codegentest.LRBindingSource)
	if err != nil {
		t.Fatal(err)
	}
	typ := reflect.TypeFor[compile.Options]()
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		t.Run(name, func(t *testing.T) {
			var o compile.Options
			f := reflect.ValueOf(&o).Elem().FieldByName(name)
			switch f.Kind() {
			case reflect.Bool:
				f.SetBool(true)
			case reflect.Uint8:
				f.SetUint(1)
			default:
				t.Fatalf("option %s is a %s: the test must say how to set it", name, f.Kind())
			}
			o.LeftRecursionBindings = true // the grammar needs it
			prog, recognizer, err := compileForGenerator(g, o)
			if err != nil {
				t.Fatal(err)
			}
			rec, err := recognizer()
			if err != nil {
				t.Fatal(err)
			}
			got, want := reflect.ValueOf(rec.Opts), reflect.ValueOf(prog.Opts)
			for j := 0; j < typ.NumField(); j++ {
				f := typ.Field(j).Name
				expected := want.Field(j).Interface()
				if f == "Recognition" {
					expected = compile.Recognize
				}
				if got.Field(j).Interface() != expected {
					t.Errorf("recognizer: option %s is %v, want %v", f, got.Field(j).Interface(), expected)
				}
			}
		})
	}
}
