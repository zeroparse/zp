// Package engine runs compiled programs: whole-input parses, recognition, prefix parses, streams
// and Documents, on the closure backend (the reference parser) or on the recursive or iterative
// bytecode VM, under the limits, tracing and profiling of a parse.
//
// A Program is what the compiler or the loader of a compiled grammar produces; Parser and Document
// run it with ParseOption functions. The public parsers, package vm and the reference parser of
// package compiler, wrap them with the options of package runtime, which Use converts. The tests
// and tools of the repository use the engine directly, with the options that select the backend
// (WithBackend) and recognition (WithRecognize). It imports neither the compiler driver nor the
// generators.
package engine
