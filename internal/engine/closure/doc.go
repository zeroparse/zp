// Package closure is the closure backend of the engine: Core builds the Core IR of a compiled program
// and optimizes it with the closure pipeline (choice guards and scans), and Build builds a Go closure
// for every node of that module, in one family of rules (Family), evaluating actions and other value
// expressions with the term programs of package core (core.TermProgram), compiled from the Core IR
// terms. Build returns the rules that the parser of package core runs.
//
// Matcher and ClassAccept are exported for the tests of package engine, which check single closures
// of programs that only that package compiles; the engine itself needs only Core and Build.
//
// The closure backend is the reference execution path: the bytecode VMs share the runtime of core
// with it and must agree with it.
package closure
