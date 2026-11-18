package codegen

import "zeroparse.dev/zp/internal/pass"

// The flags that the Go and TypeScript printers share, registered here because this package reads
// them for both.
func init() {
	pass.Register(
		pass.Flag{Name: "text-dispatch", Kind: pass.KindLowering, Default: 1, Doc: "a predicate that compares the text of a capture with literals tests them at once when the parse runs without limit controls"},
		pass.Flag{Name: "inline-literals", Kind: pass.KindLowering, Int: true, Default: 4, Doc: "compare literals of at most this many code points in place first; 0 turns it off"},
		pass.Flag{Name: "method-sharing", Kind: pass.KindPrinter, Default: 1, Doc: "emit one helper for generated bodies that are exactly equal (Go and TypeScript)"},
		pass.Flag{Name: "literal-sharing", Kind: pass.KindPrinter, Default: 1, Doc: "emit one table for equal literals (Go; TypeScript also needs ts-literal-sharing)"},
		pass.Flag{Name: "ts-literal-sharing", Kind: pass.KindPrinter, Default: 0, Doc: "share literal tables in TypeScript parsers; off because a small cold parse measured about 1.5 times slower (design 090)"},
		pass.Flag{Name: "sparse-memo", Kind: pass.KindRuntime, Default: 1, Doc: "keep the first-call table of large TypeScript parsers sparse"},
	)
}
