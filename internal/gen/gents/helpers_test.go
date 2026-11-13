package gents_test

import (
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/engine"
	"zeroparse.dev/zp/internal/enginetest"
)

// The tests of this package compile the grammars that they generate parsers for with the engine,
// as package engine does.
func init() { enginetest.UseEngine() }

var (
	mustCompile         = enginetest.MustCompile
	compileBare         = enginetest.CompileBare
	referenceProgram    = enginetest.Reference
	compileForGenerator = enginetest.CompileForGenerator
	parseTest           = enginetest.Parse
	mustParse           = enginetest.MustParse
	compileStarts       = enginetest.CompileStarts
)

// inferredType returns the value type the checker infers for the rule name, with the rules it
// refers to replaced by their values at the top level and in unions and optional types. RuleType
// refers to a rule without a declared type by name instead.
func inferredType(p *engine.Program, name string) (string, bool) {
	t, ok := p.Typed.Values[name]
	if !ok {
		return "", false
	}
	v := &compile.ValueInference{Prog: p.Static().Checker(), Out: p.Typed.Values}
	return v.Expand(t).String(), true
}
