//go:build ignore

// print_runtime writes the printed compositions of the Go runtime to runtime/call_exec.go,
// runtime/operators_exec.go and runtime/stream_exec.go, and those of the engine to
// internal/engine/core/call_exec.go and operators_exec.go. It runs
// from go generate in the directory of package core.
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
}
