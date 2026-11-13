package apitest_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// TestRootHoldsOnlyExamples checks that the test files of the root package are example files: named
// example*_test.go and declaring only Example functions, which document package zp in godoc and
// run in the package they document. Every other test of package zp, and every test of a mechanism
// that package zp exposes, belongs in this package or in the package of the mechanism.
func TestRootHoldsOnlyExamples(t *testing.T) {
	chdirModuleRoot(t)
	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if !strings.HasPrefix(name, "example") {
			t.Errorf("%s: a test file in the root is not an example file; move it to internal/apitest or to the package it tests", name)
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && !strings.HasPrefix(fd.Name.Name, "Example") {
				t.Errorf("%s: function %s in a root example file is not an Example; move it to internal/apitest or to the package it tests", name, fd.Name.Name)
			}
		}
	}
}
