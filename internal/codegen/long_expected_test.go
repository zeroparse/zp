package codegen_test

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

// TestGeneratedLongExpectedLists checks that generated Go parsers, untyped and typed, and generated
// TypeScript parsers give the engine's messages for syntax errors that expect more than ten
// entries, in both units.
func TestGeneratedLongExpectedLists(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testGeneratedLongExpectedLists)
}

func testGeneratedLongExpectedLists(t *testing.T) { checkGeneratedCase(t, "long expected lists") }

// TestGeneratedLiteralExpectedText checks that generated Go parsers, untyped and typed, and generated
// TypeScript parsers print the text of literal expected items, escaped, non-ASCII and
// case-insensitive, as the engine does.
func TestGeneratedLiteralExpectedText(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, func(t *testing.T) { checkGeneratedCase(t, "literal expected text") })
}

func checkGeneratedCase(t *testing.T, name string) {
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	var cases []codegentest.Case
	for _, c := range codegentest.GenCorpus(t) {
		if c.Name == name {
			cases = append(cases, c)
		}
	}
	if len(cases) != 1 {
		t.Fatalf("%d cases", len(cases))
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	codegentest.GeneratedParsersCorpus(t, goBin, cases, codegen.GenOptions{})
	codegentest.GeneratedTypesCorpus(t, goBin, cases, codegen.GenOptions{ConvertTypes: true}, 1)
	if node, err := exec.LookPath("node"); err == nil {
		codegentest.TSParsersCorpus(t, node, cases, codegen.GenOptions{})
	}
}
