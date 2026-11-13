//go:build zp_long

package gogen_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

// Generate itself reports a grammar whose rule table does not fit the compact form.
func TestGenerateRejectsRuleTableOverLimit(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("generates a grammar with 65537 distinct capture lists")
	}
	const n = math.MaxUint16 + 2
	var b strings.Builder
	b.WriteString("rule main = r0")
	for i := 1; i < n; i++ {
		fmt.Fprintf(&b, " / r%d", i)
	}
	b.WriteString("\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "rule r%d = c%d:\"a\"\n", i, i)
	}
	g, err := parseTest(b.String())
	if err != nil {
		t.Fatal(err)
	}
	_, err = codegentest.Generate(t, g, codegen.GenOptions{Package: "p", Start: "main", Lib: codegen.LibImport})
	if err == nil || !strings.Contains(err.Error(), "distinct capture or variable lists") {
		t.Fatalf("Generate = %v, want the rule table limit error", err)
	}
}
