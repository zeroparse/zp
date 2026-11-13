package libgo

import (
	"os"
	"testing"
)

// TestEngineExpBindingsCurrent checks that the engine's conformance bindings of the expectation record
// are those the engine's printer prints now.
func TestEngineExpBindingsCurrent(t *testing.T) {
	want, err := printEngineExpBindings()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(engineExpBindingsFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s is stale; run go generate ./internal/ir/exec/libgo", engineExpBindingsFile)
	}
}

// TestEngineDocBindingsCurrent checks that the engine's conformance bindings of the doc operations are
// those the engine's printer prints now.
func TestEngineDocBindingsCurrent(t *testing.T) {
	want, err := printEngineDocBindings()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(engineDocBindingsFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s is stale; run go generate ./internal/ir/exec/libgo", engineDocBindingsFile)
	}
}

// TestEngineOprBindingsCurrent checks that the engine's conformance bindings of the Operators Rule
// operations are those the engine's printer prints now.
func TestEngineOprBindingsCurrent(t *testing.T) {
	want, err := printEngineOprBindings()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(engineOprBindingsFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s is stale; run go generate ./internal/ir/exec/libgo", engineOprBindingsFile)
	}
}

// TestEngineCtxBindingsCurrent checks that the engine's conformance bindings of the dependency memo
// are those the engine's printer prints now.
func TestEngineCtxBindingsCurrent(t *testing.T) {
	want, err := printEngineCtxBindings()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(engineCtxBindingsFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s is stale; run go generate ./internal/ir/exec/libgo", engineCtxBindingsFile)
	}
}

// TestEngineFrameBindingsCurrent checks that the engine's conformance bindings of the capture frames
// are those the engine's printer prints now.
func TestEngineFrameBindingsCurrent(t *testing.T) {
	want, err := printEngineFrameBindings()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(engineFrameBindingsFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s is stale; run go generate ./internal/ir/exec/libgo", engineFrameBindingsFile)
	}
}

// TestEngineRuleBindingsCurrent checks that the engine's conformance bindings of the operations of the
// rules are those the engine's printer prints now.
func TestEngineRuleBindingsCurrent(t *testing.T) {
	want, err := printEngineRuleBindings()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(engineRuleBindingsFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s is stale; run go generate ./internal/ir/exec/libgo", engineRuleBindingsFile)
	}
}

// TestEngineMissingBindingsCurrent checks that the engine's conformance bindings of the missing-token
// mode are those the engine's printer prints now.
func TestEngineMissingBindingsCurrent(t *testing.T) {
	want, err := printEngineMissingBindings()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(engineMissingBindingsFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s is stale; run go generate ./internal/ir/exec/libgo", engineMissingBindingsFile)
	}
}
