//go:build ignore

// generate copies the library of generated Go parsers (pkg/runtime/libgo) into _libgo.
package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	src := filepath.Join("..", "..", "..", "..", "..", "pkg", "runtime", "libgo")
	if err := os.RemoveAll("_libgo"); err != nil {
		log.Fatal(err)
	}
	if err := os.Mkdir("_libgo", 0o755); err != nil {
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
		if err := os.WriteFile(filepath.Join("_libgo", n), data, 0o644); err != nil {
			log.Fatal(err)
		}
	}
}
