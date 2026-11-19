// Package print generates Go parsers: the code of a grammar's rules (Generate), the Go backend of
// Exec IR that prints it, and the Go side of the library of generated parsers, whose source it
// reads from pkg/runtime/libgo, with the declarations of core from the generated index coreapi_gen.go.
package print
