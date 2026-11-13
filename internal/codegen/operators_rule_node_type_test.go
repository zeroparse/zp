package codegen_test

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

// Generated Go parsers, plain and typed, and generated TypeScript parsers build the same trees as
// the engine for Operators Rules with and without a named node type.
func TestOperatorsRuleNamedNodeTypeGenerated(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testOperatorsRuleNamedNodeTypeGenerated)
}

func testOperatorsRuleNamedNodeTypeGenerated(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	cases := codegentest.OperatorsNamedNodeCases()
	t.Run("go", func(t *testing.T) { codegentest.GeneratedParsersCorpus(t, goBin, cases, codegen.GenOptions{}) })
	t.Run("go source info", func(t *testing.T) {
		codegentest.GeneratedParsersCorpus(t, goBin, cases, codegen.GenOptions{SourceInfo: true})
	})
	t.Run("typed go", func(t *testing.T) {
		codegentest.GeneratedTypesCorpus(t, goBin, cases, codegen.GenOptions{ConvertTypes: true}, len(cases))
	})
	t.Run("typescript", func(t *testing.T) {
		node, err := exec.LookPath("node")
		if err != nil {
			t.Skip("node not found")
		}
		codegentest.TSParsersCorpus(t, node, cases, codegen.GenOptions{})
		codegentest.TSParsersCorpus(t, node, cases, codegen.GenOptions{SourceInfo: true})
	})
}
