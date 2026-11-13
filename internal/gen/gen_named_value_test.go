package gen_test

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
)

func TestGeneratedNamedValueUntypedAndTS(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testGeneratedNamedValueUntypedAndTS)
}

func testGeneratedNamedValueUntypedAndTS(t *testing.T) {
	g, err := parseTest(gentest.NamedValueGrammar)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := gentest.GenerateCode(t, g, gen.GenOptions{Package: "plain", Start: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plain), "func ParseRuleValue(") {
		t.Fatal("typed entry point in untyped parser")
	}
	ts, err := gentest.GenerateTSBundle(g, gen.GenOptions{Package: "plain", Start: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(ts), "function ParseRuleValue(") {
		t.Fatal("Go API leaked into TypeScript")
	}
}
