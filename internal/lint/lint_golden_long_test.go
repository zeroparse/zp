//go:build zp_long

package lint

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/factstest"
	"zeroparse.dev/zp/internal/syntax"
)

// TestLintGolden pins the findings, with the cost classification, of the grammars in testdata and
// examples from their default start rules.
func TestLintGolden(t *testing.T) {
	root := factstest.Root(t)
	files, err := filepath.Glob(filepath.Join(root, "testdata", "grammars", "*", "*.zp"))
	if err != nil {
		t.Fatal(err)
	}
	more, err := filepath.Glob(filepath.Join(root, "examples", "*", "*.zp"))
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, more...)
	out := make([]string, len(files))
	t.Run("grammars", func(t *testing.T) {
		for i, file := range files {
			path, err := filepath.Rel(root, file)
			if err != nil {
				t.Fatal(err)
			}
			path = filepath.ToSlash(path)
			t.Run(path, func(t *testing.T) {
				t.Parallel()
				g, err := syntax.Parse(factstest.Read(t, path))
				if err != nil {
					t.Fatal(err)
				}
				fs, err := Config{Classify: true}.Run(context.Background(), g, "")
				if err != nil {
					t.Fatal(err)
				}
				var b strings.Builder
				fmt.Fprintf(&b, "# %s\n", path)
				for _, f := range fs {
					fmt.Fprintf(&b, "%s rule=%s\n", f, f.Rule)
				}
				out[i] = b.String()
			})
		}
	})
	if !t.Failed() {
		factstest.GoldenFile(t, "lint.txt", strings.Join(out, ""))
	}
}
