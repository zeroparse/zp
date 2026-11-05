package core

import (
	"os"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec"
)

// TestGoRuntimeCurrent checks that the printed compositions in the Go runtime are those the
// compositions print now.
func TestGoRuntimeCurrent(t *testing.T) {
	want, err := GoRuntime()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(GoRuntimeFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s is stale; run go generate ./internal/ir/exec/core ./gen", GoRuntimeFile)
	}
}

// TestGoStreamCurrent checks that the printed stream compositions in the Go runtime are those the
// compositions print now.
func TestGoStreamCurrent(t *testing.T) {
	want, err := GoStream()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(GoStreamFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s is stale; run go generate ./internal/ir/exec/core ./gen", GoStreamFile)
	}
}

// TestEngineCurrent checks that the printed compositions in the engine are those the compositions
// print now.
func TestEngineCurrent(t *testing.T) {
	want, err := Engine()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(EngineFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s is stale; run go generate ./internal/ir/exec/core", EngineFile)
	}
	want, err = EngineOperators()
	if err != nil {
		t.Fatal(err)
	}
	got, err = os.ReadFile(EngineOperatorsFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s is stale; run go generate ./internal/ir/exec/core", EngineOperatorsFile)
	}
	want, err = EngineStream()
	if err != nil {
		t.Fatal(err)
	}
	got, err = os.ReadFile(EngineStreamFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s is stale; run go generate ./internal/ir/exec/core", EngineStreamFile)
	}
}

// TestSpecializationsVerify checks that every operation the compositions call is declared,
// and that the functions verify for every specialization a target prints.
func TestSpecializationsVerify(t *testing.T) {
	m := Build()
	if _, err := GoRuntime(); err != nil {
		t.Fatal(err)
	}
	if _, err := GoStream(); err != nil {
		t.Fatal(err)
	}
	if _, err := Engine(); err != nil {
		t.Fatal(err)
	}
	if _, err := EngineOperators(); err != nil {
		t.Fatal(err)
	}
	ms, err := TSMethods()
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range ms {
		if !strings.Contains(x.Text, x.Name+"(") {
			t.Errorf("method %s prints as\n%s", x.Name, x.Text)
		}
	}
	for _, f := range m.Funcs {
		if f.Doc == "" {
			t.Errorf("composition %s has no doc", f.Name)
		}
		t.Log(exec.FuncString(f))
	}
}
