//go:build ignore

// coreapi_generate writes coreapi_gen.go, the index of the declarations of the package core that
// the Go generator reads (gogen.CoreAPISource). Run it with go generate in this directory.
package main

import (
	"log"
	"os"
	"path/filepath"

	gogen "zeroparse.dev/zp/internal/codegen/go"
)

func main() {
	src, err := gogen.CoreAPISource(os.DirFS(filepath.Join("..", "..", "..", "core")))
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("coreapi_gen.go", src, 0o644); err != nil {
		log.Fatal(err)
	}
}
