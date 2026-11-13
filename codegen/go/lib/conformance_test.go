package libgo_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen/go/lib"
	"zeroparse.dev/zp/internal/gentest"
)

func TestMain(m *testing.M) {
	removeGoCache := gentest.SetGoCache()
	code := m.Run()
	removeGoCache()
	os.Exit(code)
}

// libOverlay writes a go build overlay (go help build, -overlay) that adds the tests of
// testdata/conformance and the conformance bindings that the library's printer prints
// (libgo.GoExpBindings, libgo.GoDocBindings, libgo.GoRuleBindings, libgo.GoOprBindings,
// libgo.GoCtxBindings, libgo.GoFrameBindings, libgo.GoTypedFrameBindings, libgo.GoObsBindings) to the
// library package, and returns its path. The tests bind the operations to unexported code of the
// library, so they belong to package runtime; mutate, if it is not nil, changes the source of a file
// of the library from its name.
func libOverlay(t *testing.T, mutate func(name, src string) string) string {
	t.Helper()
	src := filepath.Join(gentest.ModuleRoot(), "codegen", "go", "lib")
	dir := t.TempDir()
	replace := map[string]string{}
	add := func(name string, data []byte) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		replace[filepath.Join(src, name)] = path
	}
	if mutate != nil {
		entries, err := os.ReadDir(src)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if n := e.Name(); !e.IsDir() && strings.HasSuffix(n, ".go") && !strings.HasSuffix(n, "_test.go") {
				data, err := os.ReadFile(filepath.Join(src, n))
				if err != nil {
					t.Fatal(err)
				}
				if m := mutate(n, string(data)); m != string(data) {
					add(n, []byte(m))
				}
			}
		}
	}
	tests, err := os.ReadDir(filepath.Join("testdata", "conformance"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range tests {
		data, err := os.ReadFile(filepath.Join("testdata", "conformance", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		add(e.Name(), data)
	}
	for file, print := range map[string]func() (string, error){
		"exp_bind_exec_test.go":    libgogen.GoExpBindings,
		"doc_bind_exec_test.go":    libgogen.GoDocBindings,
		"opr_bind_exec_test.go":    libgogen.GoOprBindings,
		"ctx_bind_exec_test.go":    libgogen.GoCtxBindings,
		"frame_bind_exec_test.go":  libgogen.GoFrameBindings,
		"tframe_bind_exec_test.go": libgogen.GoTypedFrameBindings,
		"rule_bind_exec_test.go":   libgogen.GoRuleBindings,
		libgogen.GoObsBindingsFile: libgogen.GoObsBindings,
	} {
		src, err := print()
		if err != nil {
			t.Fatal(err)
		}
		add(file, []byte(src))
	}
	data, err := json.Marshal(struct{ Replace map[string]string }{replace})
	if err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(overlay, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return overlay
}

// goTestLib returns the command that runs go test with args on the library package under the
// overlay of libOverlay.
func goTestLib(overlay string, args ...string) *exec.Cmd {
	cmd := exec.Command("go", append(append([]string{"test", "-overlay=" + overlay}, args...), "./codegen/go/lib")...)
	cmd.Dir = gentest.ModuleRoot()
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=-trimpath")
	return cmd
}

// TestLibExecConformance runs testdata/conformance/exec_conformance_test.go, which binds the
// operations of the parser VM to unexported code of the library, and the other tests of
// testdata/conformance as tests of package runtime, added to it by an overlay.
func TestLibExecConformance(t *testing.T) {
	args := []string{"-count=1", "-v", "-run", "^Test(ExecConformance|ValueConformance|ValueBoxedOperands|ValueBoxedStringCharges|ValueCompareStrings|ValueMemberTable|ValueBuiltinTable|ValueStructTable|ValueHullTable|ValueIterTable|ValuePushTable|ValueCastTable)"}
	if testing.Short() {
		args = append(args, "-short")
	}
	out, err := goTestLib(libOverlay(t, nil), args...).CombinedOutput()
	if err != nil {
		t.Fatalf("go test: %v\n%s", err, out)
	}
	for _, name := range []string{"TestExecConformance", "TestExecConformanceDetectsBrokenLib", "TestExecConformanceDetectsBrokenTypedLib", "TestExecConformanceRule", "TestExecConformanceRuleDetectsBrokenLib", "TestExecConformanceObs", "TestExecConformanceObsDetectsBrokenLib", "TestExecConformanceExp", "TestExecConformanceExpDetectsBrokenLib", "TestExecConformanceDoc", "TestExecConformanceDocDetectsBrokenLib", "TestExecConformanceOpr", "TestExecConformanceOprDetectsBrokenLib", "TestExecConformanceCtx", "TestExecConformanceCtxDetectsBrokenLib", "TestExecConformanceFrame", "TestExecConformanceFrameDetectsBrokenLib", "TestValueConformance", "TestValueConformanceDetectsBrokenLib", "TestValueBoxedOperands", "TestValueBoxedStringCharges", "TestValueCompareStrings", "TestValueMemberTable", "TestValueBuiltinTable", "TestValueStructTable", "TestValueHullTable", "TestValueIterTable", "TestValuePushTable", "TestValueCastTable"} {
		if !strings.Contains(string(out), "--- PASS: "+name+" ") {
			t.Errorf("go test did not run %s:\n%s", name, out)
		}
	}
}
