package gogen

import (
	"go/scanner"
	"go/token"
)

// freeIdents returns the identifiers of the Go statements src that are not selected from a value.
func freeIdents(src string) []string {
	var sc scanner.Scanner
	fset := token.NewFileSet()
	sc.Init(fset.AddFile("", fset.Base(), len(src)), []byte(src), func(token.Position, string) {}, 0)
	var ids []string
	prev := token.ILLEGAL
	for {
		_, tok, lit := sc.Scan()
		if tok == token.EOF {
			return ids
		}
		if tok == token.IDENT && prev != token.PERIOD {
			ids = append(ids, lit)
		}
		prev = tok
	}
}
