package print_test

import (
	"strings"
	"testing"
	"zeroparse.dev/zp/internal/testing/mutanttest"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/testing/codegentest"
)

// TestGeneratedTSRecoveredValues checks that generated TypeScript gives the engine's value for a
// parse that recovers from a syntax error.
func TestGeneratedTSRecoveredValues(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("runs generated code")
	}
	codegentest.TSParsersCorpus(t, codegentest.RequireNode(t), codegentest.RecoveredValueCases, common.GenOptions{})
}

// TestGeneratedTSRecoveredValuesDetectsMutation is the negative control of
// TestGeneratedTSRecoveredValues: a parser that builds a wrong struct differs from the engine on the
// inputs that recover from an error.
func TestGeneratedTSRecoveredValuesDetectsMutation(t *testing.T) {
	mutanttest.SkipUnlessChanged(t, mutanttest.Pipeline...)
	t.Parallel()
	if testing.Short() {
		t.Skip("runs generated code")
	}
	const from = `c.termSetField(tn3, "A", tv1);`
	var replaced bool
	mismatches := codegentest.TSParsersMismatches(t, codegentest.RequireNode(t), codegentest.RecoveredValueCases[1:2], common.GenOptions{}, func(src string) string {
		replaced = replaced || strings.Contains(src, from)
		return strings.ReplaceAll(src, from, `c.termSetField(tn3, "A", (tv1 as number) + 1);`)
	})
	if !replaced {
		t.Fatalf("the generated parser has no %q", from)
	}
	if len(mismatches) == 0 {
		t.Error("the mutated parser differs from the engine on none of the recovered parses")
	}
}
