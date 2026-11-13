package apitest_test

import (
	"reflect"
	"testing"

	"zeroparse.dev/zp"
	"zeroparse.dev/zp/internal/engine"
)

// TestDocumentChangeFields checks that zp.DocumentChange has the fields of the engine's change
// record, in order and with the same types except for the versions, so DrainChanges copies all
// of them.
func TestDocumentChangeFields(t *testing.T) {
	t.Parallel()
	zt, et := reflect.TypeFor[zp.DocumentChange](), reflect.TypeFor[engine.DocumentChange]()
	if zt.NumField() != et.NumField() {
		t.Fatalf("zp.DocumentChange has %d fields, the engine's change record %d", zt.NumField(), et.NumField())
	}
	for i := range zt.NumField() {
		zf, ef := zt.Field(i), et.Field(i)
		if zf.Name != ef.Name {
			t.Errorf("field %d: %s, engine %s", i, zf.Name, ef.Name)
			continue
		}
		want := ef.Type
		if want == reflect.TypeFor[engine.DocumentVersion]() {
			want = reflect.TypeFor[zp.DocumentVersion]()
		}
		if zf.Type != want {
			t.Errorf("field %s: %v, want %v", zf.Name, zf.Type, want)
		}
	}
}

// TestConfigNames checks that the backend and compiled-mode names are the engine's.
func TestConfigNames(t *testing.T) {
	t.Parallel()
	for b, want := range map[zp.Backend]engine.Backend{zp.DefaultBackend: engine.Default, zp.Closure: engine.Closure, zp.Bytecode: engine.Bytecode, zp.BytecodeIterative: engine.BytecodeIterative} {
		if b.String() != want.String() {
			t.Errorf("zp.Backend %d: %q, engine %q", b, b, want)
		}
	}
	for m, want := range map[zp.CompiledMode]string{zp.CompiledParse: "parse", zp.CompiledRecognize: "recognize", zp.CompiledDocument: "document"} {
		if m.String() != want {
			t.Errorf("zp.CompiledMode %d: %q, want %q", m, m, want)
		}
	}
}
