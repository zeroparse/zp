// Package closure is the closure backend of the engine: Build builds a Go closure for every node of
// the Core IR of a compiled program, after the closure pipeline (choice guards and scans), and
// evaluates actions and other value expressions with the term programs of package core
// (core.TermProgram), compiled from the Core IR terms. It returns the rules that the parser of
// package core runs.
//
// Matcher, ClassAccept and ClassAcceptControlled are exported for the tests of package engine,
// which check single closures of programs that only that package compiles; the engine itself needs
// only Build.
//
// The closure backend is the reference execution path: the bytecode VMs share the runtime of core
// with it and must agree with it.
package closure
