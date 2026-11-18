//go:build ignore

// generate copies the library of generated Go parsers (pkg/runtime/golib) into _golib.
package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	src := filepath.Join("..", "..", "..", "..", "..", "pkg", "runtime", "golib")
	if err := os.RemoveAll("_golib"); err != nil {
		log.Fatal(err)
	}
	if err := os.Mkdir("_golib", 0o755); err != nil {
		log.Fatal(err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		log.Fatal(err)
	}
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(src, n))
		if err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join("_golib", n), data, 0o644); err != nil {
			log.Fatal(err)
		}
	}
}
