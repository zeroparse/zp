package codegentest

import (
	"os"
	"strings"
)

// SetGoFlags sets the GOFLAGS of the test process, which the go commands it runs inherit, for
// building generated programs: -trimpath, so that their builds in fresh temporary directories share
// the build cache, -mod=mod, so that the temporary modules that replace this module resolve it
// without a go.sum.
func SetGoFlags() {
	os.Setenv("GOFLAGS", strings.TrimSpace(os.Getenv("GOFLAGS")+" -trimpath -mod=mod"))
}

// SetGoCache points the build cache of the go commands that the test process runs at a fresh
// temporary directory and returns a function that removes it. Every test builds a different
// generated program, so the shared build cache would only accumulate entries that are never reused;
// a cache private to the test process still shares the builds of the standard library and of this
// module among the tests of a package.
func SetGoCache() (remove func()) {
	dir, err := os.MkdirTemp("", "zp-gocache-")
	if err != nil {
		panic(err)
	}
	os.Setenv("GOCACHE", dir)
	return func() { os.RemoveAll(dir) }
}
