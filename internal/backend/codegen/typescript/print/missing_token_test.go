package print_test

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	tsprint "zeroparse.dev/zp/internal/backend/codegen/typescript/print"
	"zeroparse.dev/zp/internal/testing/codegentest"
)

// TestMissingTokensInsertionOnlyLeftRecursionRejected checks that a grammar whose left recursion
// exists only through an insertion is rejected like any grammar with #missing.
func TestMissingTokensInsertionOnlyLeftRecursionRejected(t *testing.T) {
	t.Parallel()
	g, err := parseTest(`rule main = a $$
rule a = "(" #missing a "x" / "y"`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tsprint.GenerateParser(g, compileForGenerator, codegentest.WithLib(common.GenOptions{Start: "main"}), nil); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatal("generated a parser for the grammar:", err)
	}
}
