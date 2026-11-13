package gengo_test

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gen/gengo"
)

// miniRuntime is a runtime source with one file whose text carries marker.
func miniRuntime(marker string) fstest.MapFS {
	return fstest.MapFS{"rt.go": {Data: []byte("package runtime\n\n// " + marker + "\nconst Protocol1 = 1\n")}}
}

// TestRuntimeSourceIsPerCall shows that the runtime a generation copies is the one in its options:
// two sources in one process give two outputs, and a generation without a source fails instead of
// falling back on one a package registered.
func TestRuntimeSourceIsPerCall(t *testing.T) {
	generate := func(src fstest.MapFS) string {
		t.Helper()
		opts := gen.GenOptions{Module: "example.com/m", Runtime: gen.RuntimeOnly}
		if src != nil {
			opts.RuntimeSource = src
		}
		files, err := gengo.Generate(nil, nil, opts)
		if err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		for _, f := range files {
			b.Write(f.Content)
		}
		return b.String()
	}
	a, b := generate(miniRuntime("source-a")), generate(miniRuntime("source-b"))
	if !strings.Contains(a, "source-a") || strings.Contains(a, "source-b") {
		t.Errorf("first source not used alone:\n%s", a)
	}
	if !strings.Contains(b, "source-b") || strings.Contains(b, "source-a") {
		t.Errorf("second source not used alone:\n%s", b)
	}
	if _, err := gengo.Generate(nil, nil, gen.GenOptions{Module: "example.com/m", Runtime: gen.RuntimeOnly}); err == nil || !strings.Contains(err.Error(), "RuntimeSource") {
		t.Errorf("generation without a runtime source: err = %v, want a RuntimeSource error", err)
	}
}

// unhashableFS is a runtime source that cannot be a map key: a struct with an interface field that
// holds a slice, which reflect reports as a comparable type.
type unhashableFS struct {
	fs.FS
	tag any
}

// TestRuntimeSourceNotHashable shows that a source that is not a valid map key is used, not cached.
func TestRuntimeSourceNotHashable(t *testing.T) {
	src := unhashableFS{FS: miniRuntime("source-u"), tag: []int{1}}
	files, err := gengo.Generate(nil, nil, gen.GenOptions{Module: "example.com/m", Runtime: gen.RuntimeOnly, RuntimeSource: src})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 || !strings.Contains(string(files[0].Content), "source-u") {
		t.Errorf("source not used: %v", files)
	}
}
