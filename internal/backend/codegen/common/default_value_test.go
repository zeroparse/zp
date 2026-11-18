package common_test

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/testing/codegentest"
)

func TestDefaultValueGeneratedRejectsTypes(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testDefaultValueGeneratedRejectsTypes)
}

func testDefaultValueGeneratedRejectsTypes(t *testing.T) {
	g := mustParse("rule main = \"a\" => 1")
	if _, err := codegentest.GenerateCode(t, g, common.GenOptions{Package: "p", Start: "main", Types: true, DefaultValue: true}); err == nil || !strings.Contains(err.Error(), "default values") {
		t.Errorf("typed output with default values: %v", err)
	}
	if _, err := codegentest.GenerateTSBundle(g, common.GenOptions{Start: "main", Types: true, DefaultValue: true}); err == nil || !strings.Contains(err.Error(), "default values") {
		t.Errorf("typed TypeScript output with default values: %v", err)
	}
	if _, err := codegentest.GenerateCode(t, g, common.GenOptions{Package: "p", Start: "main", DefaultValue: true}); err != nil {
		t.Errorf("generation with default values: %v", err)
	}
}
