//go:build ignore

// print_runtime writes the printed compositions of the Go runtime to runtime/call_exec.go,
// runtime/operators_exec.go and runtime/stream_exec.go, and those of the engine to
// internal/engine/core/call_exec.go, operators_exec.go, stream_exec.go and document_exec.go, and the
// term compositions of the engine's backends to internal/engine/closure/term_exec.go and
// internal/engine/vm/term_exec.go, and the engine's conformance bindings of the expectation record, of the doc and
// Operators Rule operations and of the missing-token mode to internal/engine/core/exp_bind_exec_test.go,
// doc_bind_exec_test.go, opr_bind_exec_test.go and missing_bind_exec_test.go. It runs from go generate in the
// directory of package core.
package main

import (
	"log"
	"os"

	"zeroparse.dev/zp/internal/ir/exec/execcore"
)

func main() {
	src, err := execcore.GoRuntime()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(execcore.GoRuntimeFile, []byte(src), 0o644); err != nil {
		log.Fatal(err)
	}
	terms, err := execcore.GoTerms()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(execcore.GoTermsFile, []byte(terms), 0o644); err != nil {
		log.Fatal(err)
	}
	operators, err := execcore.GoOperators()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(execcore.GoOperatorsFile, []byte(operators), 0o644); err != nil {
		log.Fatal(err)
	}
	stream, err := execcore.GoStream()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(execcore.GoStreamFile, []byte(stream), 0o644); err != nil {
		log.Fatal(err)
	}
	engine, err := execcore.Engine()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(execcore.EngineFile, []byte(engine), 0o644); err != nil {
		log.Fatal(err)
	}
	engineTerms, err := execcore.EngineTerms()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(execcore.EngineTermsFile, []byte(engineTerms), 0o644); err != nil {
		log.Fatal(err)
	}
	engineOperators, err := execcore.EngineOperators()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(execcore.EngineOperatorsFile, []byte(engineOperators), 0o644); err != nil {
		log.Fatal(err)
	}
	engineStream, err := execcore.EngineStream()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(execcore.EngineStreamFile, []byte(engineStream), 0o644); err != nil {
		log.Fatal(err)
	}
	engineDocument, err := execcore.EngineDocument()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(execcore.EngineDocumentFile, []byte(engineDocument), 0o644); err != nil {
		log.Fatal(err)
	}
	closureTerms, err := execcore.EngineClosureTerms()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(execcore.EngineClosureTermFile, []byte(closureTerms), 0o644); err != nil {
		log.Fatal(err)
	}
	vmTerms, err := execcore.EngineVMTerms()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(execcore.EngineVMTermFile, []byte(vmTerms), 0o644); err != nil {
		log.Fatal(err)
	}
	oprBindings, err := execcore.EngineOprBindings()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(execcore.EngineOprBindingsFile, []byte(oprBindings), 0o644); err != nil {
		log.Fatal(err)
	}
	ctxBindings, err := execcore.EngineCtxBindings()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(execcore.EngineCtxBindingsFile, []byte(ctxBindings), 0o644); err != nil {
		log.Fatal(err)
	}
	frameBindings, err := execcore.EngineFrameBindings()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(execcore.EngineFrameBindingsFile, []byte(frameBindings), 0o644); err != nil {
		log.Fatal(err)
	}
	docBindings, err := execcore.EngineDocBindings()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(execcore.EngineDocBindingsFile, []byte(docBindings), 0o644); err != nil {
		log.Fatal(err)
	}
	expBindings, err := execcore.EngineExpBindings()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(execcore.EngineExpBindingsFile, []byte(expBindings), 0o644); err != nil {
		log.Fatal(err)
	}
	ruleBindings, err := execcore.EngineRuleBindings()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(execcore.EngineRuleBindingsFile, []byte(ruleBindings), 0o644); err != nil {
		log.Fatal(err)
	}
	missingBindings, err := execcore.EngineMissingBindings()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(execcore.EngineMissingBindingsFile, []byte(missingBindings), 0o644); err != nil {
		log.Fatal(err)
	}
}
