package libsrc

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestCopyCurrent checks that the copy holds exactly the library's source files, as they are.
func TestCopyCurrent(t *testing.T) {
	src := filepath.Join("..", "..", "..", "..", "..", "pkg", "runtime", "libgo")
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		want = append(want, n)
		data, err := os.ReadFile(filepath.Join(src, n))
		if err != nil {
			t.Fatal(err)
		}
		got, err := fs.ReadFile(FS, n)
		if err != nil || !bytes.Equal(got, data) {
			t.Errorf("the copy of %s is out of date; run go generate ./%s", n, "internal/backend/codegen/golang/libsrc")
		}
	}
	copied, err := fs.Glob(FS, "*")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(copied, want) {
		t.Errorf("the copy holds %v, the library %v; run go generate", copied, want)
	}
}
