//go:build ignore

// copy_runtime replaces the copy of the runtime in _runtime with the non-test Go files of
// ../runtime and of the packages it imports, ../core and ../core/noderep, which go to the same
// directories under _runtime. It runs from go generate in the directory of package gen.
package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

// dirs are the package directories that generated code needs, relative to the module root, and
// their directories in _runtime.
var dirs = [][2]string{{"runtime", "."}, {"core", "core"}, {"core/noderep", "core/noderep"}}

func main() {
	if err := os.RemoveAll("_runtime"); err != nil {
		log.Fatal(err)
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join("_runtime", dir[1]), 0o755); err != nil {
			log.Fatal(err)
		}
		files, err := filepath.Glob(filepath.Join("..", dir[0], "*.go"))
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
			if err := os.WriteFile(filepath.Join("_runtime", dir[1], filepath.Base(f)), data, 0o644); err != nil {
				log.Fatal(err)
			}
		}
	}
}
