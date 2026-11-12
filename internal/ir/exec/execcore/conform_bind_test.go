package execcore

import (
	"os"
	"testing"
)

// TestEngineExpBindingsCurrent checks that the engine's conformance bindings of the expectation record
// are those the engine's printer prints now.
func TestEngineExpBindingsCurrent(t *testing.T) {
	want, err := EngineExpBindings()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(EngineExpBindingsFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s is stale; run go generate ./internal/ir/exec/execcore", EngineExpBindingsFile)
	}
}

// TestEngineDocBindingsCurrent checks that the engine's conformance bindings of the doc operations are
// those the engine's printer prints now.
func TestEngineDocBindingsCurrent(t *testing.T) {
	want, err := EngineDocBindings()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(EngineDocBindingsFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s is stale; run go generate ./internal/ir/exec/execcore", EngineDocBindingsFile)
	}
}
