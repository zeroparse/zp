package engine

import (
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/gentest"
)

// The tests of this package compile the grammars that they generate parsers for with the engine.
func init() {
	gentest.Compile = compileForGenerator
	gentest.Reference = referenceProgram
}

// referenceProgram compiles src with the engine for the comparisons of generated parsers with it.
func referenceProgram(t *testing.T, src string, opts compile.Options) gentest.ReferenceParse {
	t.Helper()
	prog := compileBare(t, src, opts)
	return func(input string, bytes, recognize bool) (*core.Node, error) {
		unit := core.CodePoints
		if bytes {
			unit = core.Bytes
		}
		return prog.ParseWith("main", input, ParseOptions{Unit: unit, Recognize: recognize})
	}
}
