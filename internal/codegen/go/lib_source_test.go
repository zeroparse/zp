package gogen_test

import (
	"io/fs"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegen/go"
)

// miniLib is a library source with one file whose text carries marker.
func miniLib(marker string) fstest.MapFS {
	return fstest.MapFS{
		"lib.go": {Data: []byte("package libgo\n\n// " + marker + "\nconst Protocol1 = 1\n")},
	}
}

// TestGoLibIsPerCall shows that the library a generation copies is the one in its options:
// two sources in one process give two outputs, and a generation without a source fails instead of
// falling back on one a package registered.
func TestGoLibIsPerCall(t *testing.T) {
	generate := func(src fstest.MapFS) string {
		t.Helper()
		opts := codegen.GenOptions{Module: "example.com/m", Lib: codegen.LibOnly}
		if src != nil {
			opts.GoLib = src
		}
		files, err := gogen.Generate(nil, nil, opts)
		if err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		for _, f := range files {
			b.Write(f.Content)
		}
		return b.String()
	}
	a, b := generate(miniLib("source-a")), generate(miniLib("source-b"))
	if !strings.Contains(a, "source-a") || strings.Contains(a, "source-b") {
		t.Errorf("first source not used alone:\n%s", a)
	}
	if !strings.Contains(b, "source-b") || strings.Contains(b, "source-a") {
		t.Errorf("second source not used alone:\n%s", b)
	}
	if _, err := gogen.Generate(nil, nil, codegen.GenOptions{Module: "example.com/m", Lib: codegen.LibOnly}); err == nil || !strings.Contains(err.Error(), "GoLib") {
		t.Errorf("generation without a library source: err = %v, want a GoLib error", err)
	}
}

// unhashableFS is a library source that cannot be a map key: a struct with an interface field that
// holds a slice, which reflect reports as a comparable type.
type unhashableFS struct {
	fs.FS
	tag any
}

// TestGoLibNotHashable shows that a source that is not a valid map key is used, not cached.
func TestGoLibNotHashable(t *testing.T) {
	src := unhashableFS{FS: miniLib("source-u"), tag: []int{1}}
	files, err := gogen.Generate(nil, nil, codegen.GenOptions{Module: "example.com/m", Lib: codegen.LibOnly, GoLib: src})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(files, func(f codegen.GeneratedFile) bool { return strings.Contains(string(f.Content), "source-u") }) {
		t.Errorf("source not used: %v", files)
	}
}
