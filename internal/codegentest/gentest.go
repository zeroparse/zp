// Package codegentest holds what the tests of the parser engine and its generators share: the grammars
// and inputs of the test corpus, and the generation, fixture forms and test modules of generated
// parsers, so the tests in each package run the same corpus the same way.
package codegentest

import (
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
)

// GoLibSource is the Go library source that tests give to the generator: the live directory
// codegen/go/lib of this module, so that a test exercises the library as it is being edited. Package
// codegen generates from its embedded files instead.
func GoLibSource() fs.FS { return os.DirFS(filepath.Join(moduleRoot(), "codegen", "go", "lib")) }

// TSLibSource is the TypeScript library source that tests give to the generator: the live
// directory codegen/ts/lib of this module.
func TSLibSource() fs.FS { return os.DirFS(filepath.Join(moduleRoot(), "codegen", "ts", "lib")) }

// WithLib returns opts with the library sources set to the live library directories.
func WithLib(opts codegen.GenOptions) codegen.GenOptions {
	opts.GoLib = GoLibSource()
	opts.TSLib = TSLibSource()
	return opts
}

// ModuleRoot returns the root directory of this module (see moduleRoot).
func ModuleRoot() string { return moduleRoot() }

// moduleRoot returns the root directory of this module, the nearest directory above the working
// directory that has a go.mod; tests run in the directory of their package.
var moduleRoot = sync.OnceValue(func() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("codegentest: no go.mod above the working directory")
		}
		dir = parent
	}
})

// path returns the path of name, a slash-separated path relative to the directory of this package.
func path(t testing.TB, name string) string {
	t.Helper()
	return filepath.Join(moduleRoot(), "internal", "codegentest", filepath.FromSlash(name))
}
