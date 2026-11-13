package tsgen

import (
	"os"
	"path/filepath"
	"testing"
)

// libDir is the directory of the TypeScript library, from the directory of this package, in which
// go test runs.
var libDir = filepath.Join("..", "..", "..", "codegen", "ts", "lib")

// readLib returns the text of the file name of the TypeScript library.
func readLib(t testing.TB, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(libDir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
