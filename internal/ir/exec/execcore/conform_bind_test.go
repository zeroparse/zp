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

// TestEngineOprBindingsCurrent checks that the engine's conformance bindings of the Operators Rule
// operations are those the engine's printer prints now.
func TestEngineOprBindingsCurrent(t *testing.T) {
	want, err := EngineOprBindings()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(EngineOprBindingsFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s is stale; run go generate ./internal/ir/exec/execcore", EngineOprBindingsFile)
	}
}

// TestEngineCtxBindingsCurrent checks that the engine's conformance bindings of the dependency memo
// are those the engine's printer prints now.
func TestEngineCtxBindingsCurrent(t *testing.T) {
	want, err := EngineCtxBindings()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(EngineCtxBindingsFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s is stale; run go generate ./internal/ir/exec/execcore", EngineCtxBindingsFile)
	}
}

// TestEngineFrameBindingsCurrent checks that the engine's conformance bindings of the capture frames
// are those the engine's printer prints now.
func TestEngineFrameBindingsCurrent(t *testing.T) {
	want, err := EngineFrameBindings()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(EngineFrameBindingsFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s is stale; run go generate ./internal/ir/exec/execcore", EngineFrameBindingsFile)
	}
}

// TestEngineRuleBindingsCurrent checks that the engine's conformance bindings of the operations of the
// rules are those the engine's printer prints now.
func TestEngineRuleBindingsCurrent(t *testing.T) {
	want, err := EngineRuleBindings()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(EngineRuleBindingsFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s is stale; run go generate ./internal/ir/exec/execcore", EngineRuleBindingsFile)
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
		t.Errorf("%s is stale; run go generate ./internal/ir/exec/execcore", EngineMissingBindingsFile)
	}
}
