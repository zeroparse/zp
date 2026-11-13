package engine_test

import (
	"path/filepath"
	"testing"
)

// TestModuleRootHoldsNoPackage checks that the root of the module holds no Go file: the module's
// packages are its subdirectories (core, engine, codegen and the others), and a package at the root
// would be imported as zeroparse.dev/zp beside them.
func TestModuleRootHoldsNoPackage(t *testing.T) {
	chdirModuleRoot(t)
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		t.Errorf("%s: the module root holds a Go file; put it in the package it belongs to", name)
	}
}
