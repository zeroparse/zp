//go:build ignore

// coreapi_generate writes coreapi_gen.go, the index of the declarations of the package core that
// the Go generator reads (print.CoreAPISource). Run it with go generate in this directory.
package main

import (
	"log"
	"os"
	"path/filepath"

	"zeroparse.dev/zp/internal/backend/codegen/golang/print"
)

func main() {
	src, err := print.CoreAPISource(os.DirFS(filepath.Join("..", "..", "..", "..", "..", "core")))
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("coreapi_gen.go", src, 0o644); err != nil {
		log.Fatal(err)
	}
}
