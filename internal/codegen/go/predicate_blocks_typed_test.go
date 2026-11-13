package gogen_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

// These declared typed-result fixtures force typed library execution; ordinary
// Node parser parity alone does not cover generated typed Predicate Actions.
func TestPredicateBlocksGeneratedTypedParity(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testPredicateBlocksGeneratedTypedParity)
}

func testPredicateBlocksGeneratedTypedParity(t *testing.T) {
	cases := []codegentest.Case{
		{Name: "typed bindings", Src: `type Word = { Text string }
type Row = { Word Word; Code int }
var n int
rule main Row = {n := 1; n == 1} w:word $$ => Row{Word: $w, Code: n}
rule word Word = t:("é") #atomic => Word{Text: text($t)}`, Inputs: []string{"é", "", "a", "éé"}},
		{Name: "typed choice rollback", Src: `type Word = { Text string }
type Row = { Word Word; Code int }
var n int
rule main Row = {n := 0} w:word ({n := 1; false} / {n := 2; n == 2}) $$ => Row{Word: $w, Code: n}
rule word Word = t:("é") #atomic => Word{Text: text($t)}`, Inputs: []string{"é", "", "a", "éé"}},
	}
	codegentest.GeneratedTypesCorpus(t, filepath.Join(runtime.GOROOT(), "bin", "go"), cases, codegen.GenOptions{ConvertTypes: true}, len(cases))
}
