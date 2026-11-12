package libgo

import (
	"os"
	"testing"
)

// TestGoRuntimeCurrent checks that the printed compositions in the Go runtime are those the
// compositions print now.
func TestGoRuntimeCurrent(t *testing.T) {
	want, err := printGoRuntime()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(goRuntimeFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s is stale; run go generate ./internal/ir/exec/libgo ./gen", goRuntimeFile)
	}
}

// TestGoTermsCurrent checks that the printed term compositions in the Go runtime are those the
// compositions print now. The generated runtime's copy is held equal to this file by the runtime
// copy test of the generator.
func TestGoTermsCurrent(t *testing.T) {
	want, err := printGoTerms()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(goTermsFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s is stale; run go generate ./internal/ir/exec/libgo ./gen", goTermsFile)
	}
}

// TestGoStreamCurrent checks that the printed stream compositions in the Go runtime are those the
// compositions print now.
func TestGoStreamCurrent(t *testing.T) {
	want, err := printGoStream()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(goStreamFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s is stale; run go generate ./internal/ir/exec/libgo ./gen", goStreamFile)
	}
}

// TestEngineCurrent checks that the printed compositions in the engine are those the compositions
// print now.
func TestEngineCurrent(t *testing.T) {
	for _, f := range []struct {
		file string
		gen  func() (string, error)
	}{
		{engineFile, printEngine},
		{engineTermsFile, printEngineTerms},
		{engineOperatorsFile, printEngineOperators},
		{engineStreamFile, printEngineStream},
		{engineDocumentFile, printEngineDocument},
		{engineClosureTermFile, printEngineClosureTerms},
		{engineVMTermFile, printEngineVMTerms},
	} {
		want, err := f.gen()
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(f.file)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("%s is stale; run go generate ./internal/ir/exec/libgo", f.file)
		}
	}
}

// TestSpecializationsVerify checks that the functions verify for every specialization the Go
// runtime and the engine print.
func TestSpecializationsVerify(t *testing.T) {
	if _, err := printGoRuntime(); err != nil {
		t.Fatal(err)
	}
	if _, err := printGoStream(); err != nil {
		t.Fatal(err)
	}
	if _, err := printEngine(); err != nil {
		t.Fatal(err)
	}
	if _, err := printEngineOperators(); err != nil {
		t.Fatal(err)
	}
	if _, err := printEngineDocument(); err != nil {
		t.Fatal(err)
	}
}
