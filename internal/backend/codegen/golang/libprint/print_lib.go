//go:build ignore

// print_lib writes the printed compositions of the Go library to pkg/runtime/libgo/call_exec.go,
// term_exec.go, operators_exec.go and stream_exec.go, and those of the engine to
// internal/runtime/state/call_exec.go, term_exec.go, operators_exec.go, stream_exec.go and document_exec.go, and the
// term compositions of the engine's backends to internal/backend/closure/term_exec.go and
// internal/runtime/vm/term_exec.go, and the engine's conformance bindings to the *_bind_exec_test.go files of
// internal/runtime/state. It runs from go generate in the directory of package libprint.
package main

import (
	"log"

	libprint "zeroparse.dev/zp/internal/backend/codegen/golang/libprint"
)

func main() {
	if err := libprint.Write(); err != nil {
		log.Fatal(err)
	}
}
