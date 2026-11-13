package codegentest

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// CorpusWithParsers returns the grammars of the schema agreement tests: the backend corpus and the
// grammars of testdata/grammars, which are the examples of the generated parsers.
func CorpusWithParsers(t *testing.T) []Case {
	cases := GenCorpus(t)
	paths, err := filepath.Glob(path(t, "../../testdata/grammars/*/*.zp"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		name := "grammars/" + filepath.Base(filepath.Dir(path))
		if shortSkips(src) {
			continue
		}
		cases = append(cases, Case{Name: name, Src: string(src)})
	}
	return cases
}

// shortSkips reports whether the short tier leaves a grammar source out of CorpusWithParsers.
func shortSkips(src []byte) bool { return testing.Short() && len(src) > 20000 }

// CorpusWithParsersSkipped returns the names of the grammars of testdata/grammars that
// CorpusWithParsers leaves out in this run (in the short tier, the large ones), so that a test
// that compares the corpus with a stored list can tell a stale entry from a grammar not run.
func CorpusWithParsersSkipped(t *testing.T) map[string]bool {
	paths, err := filepath.Glob(path(t, "../../testdata/grammars/*/*.zp"))
	if err != nil {
		t.Fatal(err)
	}
	skipped := map[string]bool{}
	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if shortSkips(src) {
			skipped["grammars/"+filepath.Base(filepath.Dir(path))] = true
		}
	}
	return skipped
}

// SchemaCorpus returns the grammars of GenCorpus and of the constructor, field check and result check
// tests, for tests of values against output schemas.
func SchemaCorpus(t *testing.T) []Case {
	return slices.Concat(GenCorpus(t), ConstructorCases(), FieldCheckCases(), ResultCheckCases())
}
