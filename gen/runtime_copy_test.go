package gen_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp/gen"
)

// TestRuntimeCopy checks that _runtime, the copy of the runtime that package gen embeds, holds
// exactly the non-test Go files of the runtime package. Run go generate ./gen to refresh it.
func TestRuntimeCopy(t *testing.T) {
	read := func(dir string) map[string][]byte {
		t.Helper()
		paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		files := map[string][]byte{}
		for _, p := range paths {
			if strings.HasSuffix(p, "_test.go") {
				continue
			}
			data, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			files[filepath.Base(p)] = data
		}
		return files
	}
	want, got := read(filepath.Join("..", "runtime")), read("_runtime")
	if len(want) == 0 {
		t.Fatal("the runtime package has no files")
	}
	for name, data := range want {
		if c, ok := got[name]; !ok {
			t.Errorf("_runtime lacks %s; run go generate ./gen", name)
		} else if !bytes.Equal(c, data) {
			t.Errorf("_runtime/%s differs from the runtime; run go generate ./gen", name)
		}
	}
	for name := range got {
		if _, ok := want[name]; !ok {
			t.Errorf("_runtime/%s is not in the runtime; run go generate ./gen", name)
		}
	}
}

// TestGenerateRuntimeUsesEmbeddedCopy checks that Go.GenerateRuntime copies the embedded _runtime,
// not a runtime directory that another package of the same test binary (internal/gentest, which
// serves the live directory to the generator tests) might have supplied.
func TestGenerateRuntimeUsesEmbeddedCopy(t *testing.T) {
	files, err := gen.Go{Module: "example.com/m"}.GenerateRuntime()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no files")
	}
	for _, f := range files {
		want, err := os.ReadFile(filepath.Join("_runtime", filepath.Base(f.Path)))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasSuffix(f.Content, want) {
			t.Errorf("%s is not the embedded copy", f.Path)
		}
	}
}
