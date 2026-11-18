// Package lib holds the TypeScript library of generated parsers (lib.ts and the files it
// assembles), which the TypeScript generator reads and the -lib modes copy.
package lib

import "embed"

// FS holds the files of the library.
//
//go:embed *.ts
var FS embed.FS
