package lower_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestLoweringDoesNotImportTheCompiler checks the direction of the layers: the lowering of Flow IR
// reads the Core IR and the Exec IR, and does not import the compiler (the Core IR carries what the
// lowering needs from the grammar).
func TestLoweringDoesNotImportTheCompiler(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			if p, _ := strconv.Unquote(imp.Path.Value); strings.HasSuffix(p, "/internal/compile") {
				t.Errorf("%s imports %s", name, p)
			}
		}
	}
	if _, err := os.Stat("term.go"); err != nil {
		t.Fatal(err)
	}
}
