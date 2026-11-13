// Package gentest holds what the tests of the parser engine and its generators share: the grammars
// and inputs of the test corpus, and the generation, fixture forms and test modules of generated
// parsers, so the tests in each package run the same corpus the same way.
package gentest

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"zeroparse.dev/zp/internal/gen"
)

// RuntimeSource is the runtime source that tests give to the generator: the live runtime
// directory of this module at its root and the live core directory in core, so that a test
// exercises the packages as they are being edited. Package gen generates from its embedded copy
// instead.
func RuntimeSource() fs.FS { return runtimeDirs(moduleRoot()) }

// runtimeDirs is the file system of RuntimeSource over the module root it holds. It is a string, so
// that the generator caches what it parses from it.
type runtimeDirs string

func (r runtimeDirs) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	if name == "core" || strings.HasPrefix(name, "core/") {
		return os.DirFS(string(r)).Open(name)
	}
	return os.DirFS(filepath.Join(string(r), "runtime")).Open(name)
}

// WithRuntime returns opts with RuntimeSource set to the live runtime directory.
func WithRuntime(opts gen.GenOptions) gen.GenOptions {
	opts.RuntimeSource = RuntimeSource()
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
			panic("gentest: no go.mod above the working directory")
		}
		dir = parent
	}
})

// path returns the path of name, a slash-separated path relative to the directory of this package.
func path(t testing.TB, name string) string {
	t.Helper()
	return filepath.Join(moduleRoot(), "internal", "gentest", filepath.FromSlash(name))
}
