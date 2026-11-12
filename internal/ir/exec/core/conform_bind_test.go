package core

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
		t.Errorf("%s is stale; run go generate ./internal/ir/exec/core", EngineExpBindingsFile)
	}
}

// TestEngineMissingBindingsCurrent checks that the engine's conformance bindings of the missing-token
// mode are those the engine's printer prints now.
func TestEngineMissingBindingsCurrent(t *testing.T) {
	want, err := EngineMissingBindings()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(EngineMissingBindingsFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s is stale; run go generate ./internal/ir/exec/core", EngineMissingBindingsFile)
	}
}
