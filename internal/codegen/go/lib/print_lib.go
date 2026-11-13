//go:build ignore

// print_lib writes the printed compositions of the Go library to codegen/go/lib/call_exec.go,
// term_exec.go, operators_exec.go and stream_exec.go, and those of the engine to
// internal/engine/state/call_exec.go, term_exec.go, operators_exec.go, stream_exec.go and document_exec.go, and the
// term compositions of the engine's backends to internal/engine/closure/term_exec.go and
// internal/engine/vm/term_exec.go, and the engine's conformance bindings to the *_bind_exec_test.go files of
// internal/engine/state. It runs from go generate in the directory of package libgogen.
package main

import (
	"log"

	libgogen "zeroparse.dev/zp/internal/codegen/go/lib"
)

func main() {
	if err := libgogen.Write(); err != nil {
		log.Fatal(err)
	}
}
