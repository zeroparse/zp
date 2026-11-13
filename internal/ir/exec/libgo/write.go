// Package libgo prints the compositions of internal/ir/exec/core into the Go code that uses them:
// the runtime that generated Go parsers embed (runtime/), the parse state of the engine and its
// backends (internal/engine/core, closure and vm), and the engine's and the runtime's conformance
// bindings. It owns the paths of those files and the receiver and parameter text of the engine's
// types, so that the compositions stay free of their consumers' layout. The TypeScript runtime's
// compositions are printed by the TypeScript generator.
package libgo

import "os"

//go:generate go run print_runtime.go

// outputs are the files that go generate writes, relative to the directory of this package, each
// with the function that prints it.
var outputs = []struct {
	path  string
	print func() (string, error)
}{
	{goRuntimeFile, printGoRuntime},
	{goTermsFile, printGoTerms},
	{goOperatorsFile, printGoOperators},
	{goStreamFile, printGoStream},
	{engineFile, printEngine},
	{engineTermsFile, printEngineTerms},
	{engineOperatorsFile, printEngineOperators},
	{engineStreamFile, printEngineStream},
	{engineDocumentFile, printEngineDocument},
	{engineClosureTermFile, printEngineClosureTerms},
	{engineVMTermFile, printEngineVMTerms},
	{engineOprBindingsFile, printEngineOprBindings},
	{engineCtxBindingsFile, printEngineCtxBindings},
	{engineFrameBindingsFile, printEngineFrameBindings},
	{engineDocBindingsFile, printEngineDocBindings},
	{engineExpBindingsFile, printEngineExpBindings},
	{engineRuleBindingsFile, printEngineRuleBindings},
	{engineMissingBindingsFile, printEngineMissingBindings},
}

// Write prints every file of outputs and writes it. Paths are relative to the directory of this
// package, so it runs there, as go generate does.
func Write() error {
	for _, o := range outputs {
		src, err := o.print()
		if err != nil {
			return err
		}
		if err := os.WriteFile(o.path, []byte(src), 0o644); err != nil {
			return err
		}
	}
	return nil
}
