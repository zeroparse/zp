package gen_test

import (
	"testing"

	"zeroparse.dev/zp/internal/enginetest"
)

// The tests of this package compile the grammars that they generate parsers for with the engine,
// as package engine does.
func init() { enginetest.UseEngine() }

func TestMain(m *testing.M) { enginetest.Main(m) }

var (
	mustCompile           = enginetest.MustCompile
	compileBare           = enginetest.CompileBare
	referenceProgram      = enginetest.Reference
	compileForGenerator   = enginetest.CompileForGenerator
	parseTest             = enginetest.Parse
	mustParse             = enginetest.MustParse
	compileStarts         = enginetest.CompileStarts
	compileMessage        = enginetest.CompileMessage
	outputSchemaPrototype = enginetest.OutputSchema
)
