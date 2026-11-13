//go:build ignore

// copy_runtime replaces the copy of the runtime module in _runtime with the non-test Go files of
// ../runtime and of its package directories noderep and parse. It runs from go generate in the
// directory of package gen.
package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

// dirs are the package directories of the runtime module that generated code needs, relative to
// the runtime's.
var dirs = []string{".", "noderep", "parse"}

func main() {
	if err := os.RemoveAll("_runtime"); err != nil {
		log.Fatal(err)
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join("_runtime", dir), 0o755); err != nil {
			log.Fatal(err)
		}
		files, err := filepath.Glob(filepath.Join("..", "runtime", dir, "*.go"))
		if err != nil {
			log.Fatal(err)
		}
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			data, err := os.ReadFile(f)
			if err != nil {
				log.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join("_runtime", dir, filepath.Base(f)), data, 0o644); err != nil {
				log.Fatal(err)
			}
		}
	}
}
