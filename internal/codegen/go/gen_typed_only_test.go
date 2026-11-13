package gogen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

func TestGeneratedTypedOnlyConvertedEntries(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testGeneratedTypedOnlyConvertedEntries)
}

func testGeneratedTypedOnlyConvertedEntries(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated code")
	}
	program := strings.NewReplacer(`"sum"`, `"main"`, `"a"`, `"item"`).Replace(codegentest.OrdinaryOnlyTypedEntries)
	g, err := parseTest(codegentest.OrdinaryOnlyConverted)
	if err != nil {
		t.Fatal(err)
	}
	files, err := codegentest.Generate(t, g, codegen.GenOptions{Package: "p", Module: "example.com/p", Start: "main", Lib: codegen.LibImport, Types: true, TypedOnly: true})
	if err != nil {
		t.Fatalf("a typed-only parser whose entries convert Nodes: %v", err)
	}
	src := string(codegentest.MergeGenerated(files))
	if !strings.Contains(src, "RunParseConverted(") {
		t.Fatal("the entries do not convert a Node: the grammar tests nothing")
	}
	for _, absent := range []string{"func Parse(", "func ParseRule(", "func Recognize("} {
		if strings.Contains(src, absent) {
			t.Errorf("a typed-only parser has %q", absent)
		}
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(codegentest.GoMod("ordtest")), 0o644); err != nil {
		t.Fatal(err)
	}
	codegentest.OrdinaryOnlyPackage(t, dir, "full", codegentest.OrdinaryOnlyConverted, codegen.GenOptions{Types: true, Recognize: true}, program, codegentest.OrdinaryOnlyConvertedInputs)
	codegentest.OrdinaryOnlyPackage(t, dir, "only", codegentest.OrdinaryOnlyConverted, codegen.GenOptions{Types: true, TypedOnly: true}, program, codegentest.OrdinaryOnlyConvertedInputs)
	codegentest.OrdinaryOnlyPackage(t, dir, "mutated", strings.Replace(codegentest.OrdinaryOnlyConverted, `"!"`, `"?"`, 1), codegen.GenOptions{Types: true, TypedOnly: true}, program, codegentest.OrdinaryOnlyConvertedInputs)
	out := codegentest.RunTranscripts(t, dir, "full", "only", "mutated")
	if out["full"] != out["only"] {
		t.Errorf("the typed-only parser differs from the full one:\n%s", codegentest.FirstTranscriptDifference(out["full"], out["only"]))
	}
	if !strings.Contains(out["only"], `"Names":[{"Start":0,"End":3,"Value":[`) || !strings.Contains(out["only"], "syntax error") {
		t.Errorf("the transcript has neither a value nor a syntax error: %.300s", out["only"])
	}
	if out["mutated"] == out["only"] {
		t.Error("the transcripts of a mutated grammar are the same: the comparison cannot fail")
	}

	// Negative control: without the Node rule functions the converting entries have no Node rule
	// bodies to run, which fails generation in every form.
	_, err = codegentest.Generate(t, g, codegen.GenOptions{Package: "p", Module: "example.com/p", Start: "main", Lib: codegen.LibImport, Types: true, TypedOnly: true, DisableKeepNodes: true})
	if err == nil || !strings.Contains(err.Error(), "internal error: a typed entry point converts Nodes") {
		t.Errorf("a typed-only parser without the Node rule functions: error %v, want an internal error", err)
	}
	if codegentest.CountBodies([]byte(src)) == 0 {
		t.Error("the typed-only parser with the Node rule functions has no Node rule bodies")
	}
}

// A hidden rule that splices into its caller, one that calls itself where it takes one element, and one
// with an action behave the same in a typed-only parser as in a parser with every entry point, and the
// typed-only parser refuses to start at a hidden rule.
const ordinaryOnlyHidden = `rule main = _t _w "!"
rule _t = "(" _t? ")"
rule _w = x:[a-z]+ #atomic => text($x)`

var ordinaryOnlyHiddenInputs = []string{"()ab!", "(())a!", "((()))abc!", "()!", "(()ab!", "ab!", "", "(\xff)a!", "()ab!x"}

func TestGeneratedTypedOnlyHiddenRules(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds generated code")
	}
	// The entries start at main and at the hidden rule _t.
	program := strings.NewReplacer(`"sum"`, `"main"`, `"a"`, `"_t"`).Replace(codegentest.OrdinaryOnlyTypedEntries)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(codegentest.GoMod("ordtest")), 0o644); err != nil {
		t.Fatal(err)
	}
	codegentest.OrdinaryOnlyPackage(t, dir, "full", ordinaryOnlyHidden, codegen.GenOptions{Types: true, Recognize: true}, program, ordinaryOnlyHiddenInputs)
	codegentest.OrdinaryOnlyPackage(t, dir, "only", ordinaryOnlyHidden, codegen.GenOptions{Types: true, TypedOnly: true}, program, ordinaryOnlyHiddenInputs)
	codegentest.OrdinaryOnlyPackage(t, dir, "mutated", strings.Replace(ordinaryOnlyHidden, `"!"`, `"?"`, 1), codegen.GenOptions{Types: true, TypedOnly: true}, program, ordinaryOnlyHiddenInputs)
	out := codegentest.RunTranscripts(t, dir, "full", "only", "mutated")
	if out["full"] != out["only"] {
		t.Errorf("the typed-only parser differs from the full one:\n%s", codegentest.FirstTranscriptDifference(out["full"], out["only"]))
	}
	if !strings.Contains(out["only"], "rule _t is hidden") || !strings.Contains(out["only"], `"E3":"ab"`) || !strings.Contains(out["only"], "syntax error") {
		t.Errorf("the transcript lacks the refusal of a hidden rule, a value or a syntax error:\n%s", out["only"])
	}
	if out["mutated"] == out["only"] {
		t.Error("the transcripts of a mutated grammar are the same: the comparison cannot fail")
	}
}
