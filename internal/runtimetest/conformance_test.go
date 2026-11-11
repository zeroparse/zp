package runtimetest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gentest"
)

func TestMain(m *testing.M) {
	removeGoCache := gentest.SetGoCache()
	code := m.Run()
	removeGoCache()
	os.Exit(code)
}

// runtimeCopy writes the runtime module, with the tests of testdata, into a temporary directory and
// returns it. mutate, if it is not nil, changes the source of a file of the runtime from its name.
func runtimeCopy(t *testing.T, mutate func(name, src string) string) string {
	t.Helper()
	root := gentest.ModuleRoot()
	dir := t.TempDir()
	src := filepath.Join(root, "runtime")
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	copyFile := func(from, to string, mutate func(string) string) {
		t.Helper()
		data, err := os.ReadFile(from)
		if err != nil {
			t.Fatal(err)
		}
		if mutate != nil {
			data = []byte(mutate(string(data)))
		}
		if err := os.WriteFile(to, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, e := range entries {
		if n := e.Name(); !e.IsDir() && strings.HasSuffix(n, ".go") && !strings.HasSuffix(n, "_test.go") {
			var m func(string) string
			if mutate != nil {
				m = func(s string) string { return mutate(n, s) }
			}
			copyFile(filepath.Join(src, n), filepath.Join(dir, n), m)
		}
	}
	tests, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range tests {
		copyFile(filepath.Join("testdata", e.Name()), filepath.Join(dir, e.Name()), nil)
	}
	mod := "module zeroparse.dev/zp/runtime\n\ngo 1.27.1\n\nrequire zeroparse.dev/zp v0.0.0\n\nreplace zeroparse.dev/zp => " + root + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestRuntimeExecConformance runs testdata/exec_conformance_test.go, which binds the operations of
// the parser VM to unexported code of the runtime, as a test of package runtime: in a temporary
// copy of the runtime module that requires this module for the conformance suite.
func TestRuntimeExecConformance(t *testing.T) {
	dir := runtimeCopy(t, nil)
	args := []string{"test", "-count=1", "-v", "-run", "^Test(ExecConformance|ValueConformance|ValueBoxedOperands|ValueCompareStrings|ValueMemberTable|ValueBuiltinTable|ValueStructTable|ValueHullTable|ValueIterTable|ValuePushTable|ValueCastTable)"}
	if testing.Short() {
		args = append(args, "-short")
	}
	cmd := exec.Command("go", append(args, ".")...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=-mod=mod -trimpath")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test: %v\n%s", err, out)
	}
	for _, name := range []string{"TestExecConformance", "TestExecConformanceDetectsBrokenRuntime", "TestExecConformanceDetectsBrokenTypedRuntime", "TestValueConformance", "TestValueConformanceDetectsBrokenRuntime", "TestValueBoxedOperands", "TestValueCompareStrings", "TestValueMemberTable", "TestValueBuiltinTable", "TestValueStructTable", "TestValueHullTable", "TestValueIterTable", "TestValuePushTable", "TestValueCastTable"} {
		if !strings.Contains(string(out), "--- PASS: "+name+" ") {
			t.Errorf("go test did not run %s:\n%s", name, out)
		}
	}
}
