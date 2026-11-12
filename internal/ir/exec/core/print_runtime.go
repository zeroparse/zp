//go:build ignore

// print_runtime writes the printed compositions of the Go runtime to runtime/call_exec.go,
// runtime/operators_exec.go and runtime/stream_exec.go, and those of the engine to
// internal/engine/core/call_exec.go, operators_exec.go, stream_exec.go and document_exec.go, and the
// term compositions of the engine's backends to internal/engine/closure/term_exec.go and
// internal/engine/vm/term_exec.go, and the engine's conformance bindings of the expectation record and of the doc operations to
// internal/engine/core/exp_bind_exec_test.go and doc_bind_exec_test.go. It runs from go generate in the directory of package core.
package main

import (
	"log"
	"os"

	"zeroparse.dev/zp/internal/ir/exec/core"
)

func main() {
	src, err := core.GoRuntime()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(core.GoRuntimeFile, []byte(src), 0o644); err != nil {
		log.Fatal(err)
	}
	terms, err := core.GoTerms()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(core.GoTermsFile, []byte(terms), 0o644); err != nil {
		log.Fatal(err)
	}
	operators, err := core.GoOperators()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(core.GoOperatorsFile, []byte(operators), 0o644); err != nil {
		log.Fatal(err)
	}
	stream, err := core.GoStream()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(core.GoStreamFile, []byte(stream), 0o644); err != nil {
		log.Fatal(err)
	}
	engine, err := core.Engine()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(core.EngineFile, []byte(engine), 0o644); err != nil {
		log.Fatal(err)
	}
	engineTerms, err := core.EngineTerms()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(core.EngineTermsFile, []byte(engineTerms), 0o644); err != nil {
		log.Fatal(err)
	}
	engineOperators, err := core.EngineOperators()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(core.EngineOperatorsFile, []byte(engineOperators), 0o644); err != nil {
		log.Fatal(err)
	}
	engineStream, err := core.EngineStream()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(core.EngineStreamFile, []byte(engineStream), 0o644); err != nil {
		log.Fatal(err)
	}
	engineDocument, err := core.EngineDocument()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(core.EngineDocumentFile, []byte(engineDocument), 0o644); err != nil {
		log.Fatal(err)
	}
	closureTerms, err := core.EngineClosureTerms()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(core.EngineClosureTermFile, []byte(closureTerms), 0o644); err != nil {
		log.Fatal(err)
	}
	vmTerms, err := core.EngineVMTerms()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(core.EngineVMTermFile, []byte(vmTerms), 0o644); err != nil {
		log.Fatal(err)
	}
	docBindings, err := core.EngineDocBindings()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(core.EngineDocBindingsFile, []byte(docBindings), 0o644); err != nil {
		log.Fatal(err)
	}
	expBindings, err := core.EngineExpBindings()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(core.EngineExpBindingsFile, []byte(expBindings), 0o644); err != nil {
		log.Fatal(err)
	}
}
