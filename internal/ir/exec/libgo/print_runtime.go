//go:build ignore

// print_runtime writes the printed compositions of the Go runtime to runtime/call_exec.go,
// runtime/term_exec.go, runtime/operators_exec.go and runtime/stream_exec.go, and those of the engine to
// internal/engine/core/call_exec.go, term_exec.go, operators_exec.go, stream_exec.go and document_exec.go, and the
// term compositions of the engine's backends to internal/engine/closure/term_exec.go and
// internal/engine/vm/term_exec.go, and the engine's conformance bindings to the *_bind_exec_test.go files of
// internal/engine/core. It runs from go generate in the directory of package libgo.
package main

import (
	"log"

	"zeroparse.dev/zp/internal/ir/exec/libgo"
)

func main() {
	if err := libgo.Write(); err != nil {
		log.Fatal(err)
	}
}
