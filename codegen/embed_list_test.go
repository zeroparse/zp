package codegen

import (
	"io/fs"
	"os"
	"slices"
	"strings"
	"testing"
)

// TestGoLibEmbedsItsSources checks that the files the package embeds from go/lib are the sources of
// the library, which generation copies into packages generated with LibEmbed, and none of its
// tests: a library file missing from the go:embed list of goLibFiles would be missing from the
// copies, and a test file would only make every program that imports this package larger.
func TestGoLibEmbedsItsSources(t *testing.T) {
	entries, err := os.ReadDir("go/lib")
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, e := range entries {
		if n := e.Name(); !e.IsDir() && strings.HasSuffix(n, ".go") && !strings.HasSuffix(n, "_test.go") {
			want = append(want, n)
		}
	}
	embedded, err := fs.ReadDir(goLib, ".")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range embedded {
		got = append(got, e.Name())
	}
	for _, n := range want {
		if !slices.Contains(got, n) {
			t.Errorf("go/lib/%s is not embedded: add it to the go:embed list of goLibFiles", n)
		}
	}
	for _, n := range got {
		if !slices.Contains(want, n) {
			t.Errorf("go/lib/%s is embedded but is not a source of the library: remove it from the go:embed list of goLibFiles", n)
		}
	}
}
