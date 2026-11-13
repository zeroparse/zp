package engine_test

import (
	"reflect"
	"testing"

	"zeroparse.dev/zp/engine"
	iengine "zeroparse.dev/zp/internal/engine"
)

// TestDocumentChangeFields checks that engine.DocumentChange has the fields of the engine's change
// record, in order and with the same types except for the versions, so DrainChanges copies all
// of them.
func TestDocumentChangeFields(t *testing.T) {
	t.Parallel()
	zt, et := reflect.TypeFor[engine.DocumentChange](), reflect.TypeFor[iengine.DocumentChange]()
	if zt.NumField() != et.NumField() {
		t.Fatalf("engine.DocumentChange has %d fields, the engine's change record %d", zt.NumField(), et.NumField())
	}
	for i := range zt.NumField() {
		zf, ef := zt.Field(i), et.Field(i)
		if zf.Name != ef.Name {
			t.Errorf("field %d: %s, engine %s", i, zf.Name, ef.Name)
			continue
		}
		want := ef.Type
		if want == reflect.TypeFor[iengine.DocumentVersion]() {
			want = reflect.TypeFor[engine.DocumentVersion]()
		}
		if zf.Type != want {
			t.Errorf("field %s: %v, want %v", zf.Name, zf.Type, want)
		}
	}
}

// TestConfigNames checks that the backend and compiled-mode names are the engine's.
func TestConfigNames(t *testing.T) {
	t.Parallel()
	for b, want := range map[engine.Backend]iengine.Backend{engine.DefaultBackend: iengine.Default, engine.Closure: iengine.Closure, engine.Bytecode: iengine.Bytecode, engine.BytecodeIterative: iengine.BytecodeIterative} {
		if b.String() != want.String() {
			t.Errorf("engine.Backend %d: %q, engine %q", b, b, want)
		}
	}
	for m, want := range map[engine.CompiledMode]string{engine.CompiledParse: "parse", engine.CompiledRecognize: "recognize", engine.CompiledDocument: "document"} {
		if m.String() != want {
			t.Errorf("engine.CompiledMode %d: %q, want %q", m, m, want)
		}
	}
}
