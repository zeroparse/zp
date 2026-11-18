package common_test

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/testing/codegentest"
)

// Generated Go parsers, plain and typed, and generated TypeScript parsers read a scalar operator
// local and the folded value as the engine does.
func TestOperatorsRuleLocalsGenerated(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testOperatorsRuleLocalsGenerated)
}

func testOperatorsRuleLocalsGenerated(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	cases := codegentest.OperatorsRuleLocalCases
	t.Run("go", func(t *testing.T) { codegentest.GeneratedParsersCorpus(t, goBin, cases, common.GenOptions{}) })
	t.Run("go source info", func(t *testing.T) {
		codegentest.GeneratedParsersCorpus(t, goBin, cases, common.GenOptions{SourceInfo: true})
	})
	t.Run("typed go", func(t *testing.T) {
		codegentest.GeneratedTypesCorpus(t, goBin, cases, common.GenOptions{ConvertTypes: true}, len(cases))
	})
	t.Run("typescript", func(t *testing.T) {
		node, err := exec.LookPath("node")
		if err != nil {
			t.Skip("node not found")
		}
		codegentest.TSParsersCorpus(t, node, cases, common.GenOptions{})
		codegentest.TSParsersCorpus(t, node, cases, common.GenOptions{SourceInfo: true})
	})
}
