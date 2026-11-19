package engine

import (
	"testing"
	"zeroparse.dev/zp/internal/driver/checked"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/testing/codegentest"
	zpinput "zeroparse.dev/zp/pkg/runtime/input"
	"zeroparse.dev/zp/pkg/runtime/value"
)

// The tests of this package compile the grammars that they generate parsers for with the engine.
func init() {
	codegentest.Compile = checked.GeneratorCompiler(compileForGenerator)
	codegentest.Reference = referenceProgram
}

// referenceProgram compiles src with the engine for the comparisons of generated parsers with it.
func referenceProgram(t *testing.T, src string, opts compile.Options) codegentest.ReferenceParse {
	t.Helper()
	prog := compileBare(t, src, opts)
	return func(input string, bytes, recognize bool) (*value.Node, error) {
		unit := zpinput.CodePoints
		if bytes {
			unit = zpinput.Bytes
		}
		return prog.ParseWith("main", input, ParseOptions{Unit: unit, Recognize: recognize})
	}
}
