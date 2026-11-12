package language_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"zeroparse.dev/zp"
	"zeroparse.dev/zp/gen"
)

// TestGeneratedSourceParserCurrent requires that the checked-in generated parser of the grammar
// source is what the generator writes from zp.zp with the options of its go:generate line, so that
// the frontend a build uses is the grammar of the language. The first line, which names the version
// of the generator, is not compared.
func TestGeneratedSourceParserCurrent(t *testing.T) {
	_, src := sourceGrammar(t)
	g, err := zp.ParseGrammar(src)
	if err != nil {
		t.Fatal(err)
	}
	files, err := gen.Go{Module: "zeroparse.dev/zp/internal/syntax/genparser", Runtime: gen.RuntimeImport, Types: true, TypedOnly: true}.Generate(g, "main")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join("..", "..", "internal", "syntax", "genparser")
	var want []string
	for _, f := range files {
		want = append(want, filepath.ToSlash(f.Path))
		got, err := os.ReadFile(filepath.Join(dir, f.Path))
		if err != nil {
			t.Errorf("%s: %v (run go generate ./internal/syntax)", f.Path, err)
			continue
		}
		if !bytes.Equal(afterFirstLine(got), afterFirstLine(f.Content)) {
			t.Errorf("%s is stale: run go generate ./internal/syntax", f.Path)
		}
	}
	var have []string
	err = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(dir, p)
			have = append(have, filepath.ToSlash(rel))
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(want)
	slices.Sort(have)
	if !slices.Equal(have, want) {
		t.Errorf("files of %s: %v, want %v", dir, have, want)
	}
}

func afterFirstLine(b []byte) []byte {
	_, rest, _ := bytes.Cut(b, []byte("\n"))
	return rest
}
