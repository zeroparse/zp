package tsgen_test

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
)

// TestGeneratedTSRecoveredValues checks that generated TypeScript gives the engine's value for a
// parse that recovers from a syntax error.
func TestGeneratedTSRecoveredValues(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("runs generated code")
	}
	gentest.TSParsersCorpus(t, gentest.RequireNode(t), gentest.RecoveredValueCases, gen.GenOptions{})
}

// TestGeneratedTSRecoveredValuesDetectsMutation is the negative control of
// TestGeneratedTSRecoveredValues: a parser that builds a wrong struct differs from the engine on the
// inputs that recover from an error.
func TestGeneratedTSRecoveredValuesDetectsMutation(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("runs generated code")
	}
	const from = `c.length(c.cap(0))`
	var replaced bool
	mismatches := gentest.TSParsersMismatches(t, gentest.RequireNode(t), gentest.RecoveredValueCases[1:2], gen.GenOptions{}, func(src string) string {
		replaced = replaced || strings.Contains(src, from)
		return strings.ReplaceAll(src, from, from+" + 1")
	})
	if !replaced {
		t.Fatalf("the generated parser has no %q", from)
	}
	if len(mismatches) == 0 {
		t.Error("the mutated parser differs from the engine on none of the recovered parses")
	}
}
