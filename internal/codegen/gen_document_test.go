package codegen_test

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/gentest"
)

func TestGenerateDocumentCombinations(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testGenerateDocumentCombinations)
}

func testGenerateDocumentCombinations(t *testing.T) {
	g, err := parseTest("rule main = \"a\"")
	if err != nil {
		t.Fatal(err)
	}
	typed, err := gentest.GenerateCode(t, g, codegen.GenOptions{Package: "fixture", Start: "main", Document: true, Types: true})
	if err != nil || !strings.Contains(string(typed), "func (d *Document) ParseValue(") {
		t.Fatalf("typed Document generation: %v", err)
	}
	ts, err := gentest.GenerateTSBundle(g, codegen.GenOptions{Start: "main", Document: true, Types: true})
	if err != nil || !strings.Contains(string(ts), "export function createDocument(") || !strings.Contains(string(ts), "  parseValue(options?: ") {
		t.Fatalf("typed TypeScript Document generation: %v", err)
	}
}
