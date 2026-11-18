// Package libsrc holds a copy of the source of the library of generated Go parsers
// (pkg/runtime/golib), which code generation reads and the -lib embed and -lib only modes copy
// into the generated module. The library cannot embed its own source, since every generated parser
// links it; go generate refreshes the copy and TestCopyCurrent keeps it current. The copy is in
// the directory _golib, which the go command does not build.
package libsrc

import (
	"embed"
	"io/fs"
)

//go:generate go run generate.go

//go:embed _golib/*.go
var files embed.FS

// FS holds the files of the library (its .go files but the tests), at the root.
var FS fs.FS = sub()

func sub() fs.FS {
	s, err := fs.Sub(files, "_golib")
	if err != nil {
		panic(err)
	}
	return s
}
