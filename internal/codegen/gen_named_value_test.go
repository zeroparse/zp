package codegen_test

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

func TestGeneratedNamedValueUntypedAndTS(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testGeneratedNamedValueUntypedAndTS)
}

func testGeneratedNamedValueUntypedAndTS(t *testing.T) {
	g, err := parseTest(codegentest.NamedValueGrammar)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := codegentest.GenerateCode(t, g, codegen.GenOptions{Package: "plain", Start: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plain), "func ParseRuleValue(") {
		t.Fatal("typed entry point in untyped parser")
	}
	ts, err := codegentest.GenerateTSBundle(g, codegen.GenOptions{Package: "plain", Start: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(ts), "function ParseRuleValue(") {
		t.Fatal("Go API leaked into TypeScript")
	}
}
