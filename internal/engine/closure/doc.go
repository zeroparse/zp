// Package closure is the closure backend of the engine: it builds a Go closure for every
// expression of a compiled program, and evaluates actions and other value expressions with the
// term programs of package core (core.TermProgram), compiled from the Core IR of the program.
// Builder implements compile.ExprBuilder and returns the rules that the parser of package core runs.
//
// Builder.Guard, ClassAccept and ClassAcceptControlled are
// exported for the tests of package engine, which check the closure backend through programs
// that only that package compiles; the engine itself needs only NewBuilder and Builder.
//
// The closure backend is the reference execution path: the bytecode VMs share the runtime of core
// with it and must agree with it.
package closure
