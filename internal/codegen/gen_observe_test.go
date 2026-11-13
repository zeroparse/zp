package codegen_test

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

func TestGeneratedTraceOptions(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testGeneratedTraceOptions)
}

func testGeneratedTraceOptions(t *testing.T) {
	g, err := parseTest(`rule main = "a"`)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := codegentest.GenerateCode(t, g, codegen.GenOptions{Package: "fixture", Start: "main"})
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"type TraceEvent", "func newProfileObservation", "func WithTrace"} {
		if strings.Contains(string(plain), marker) {
			t.Fatalf("ordinary output contains %s", marker)
		}
	}
	if _, err := codegentest.GenerateCode(t, g, codegen.GenOptions{Package: "fixture", Start: "main", Types: true, Observability: true}); err != nil {
		t.Fatal("typed observability:", err)
	}
	for _, opts := range []codegen.GenOptions{{Document: true}, {Document: true, Types: true}} {
		opts.Package, opts.Start, opts.Observability = "fixture", "main", true
		if _, err := codegentest.GenerateCode(t, g, opts); err != nil {
			t.Fatal("Document observability:", err)
		}
	}
	if code, err := codegentest.GenerateTSBundle(g, codegen.GenOptions{Start: "main", Observability: true}); err != nil || !strings.Contains(string(code), "export interface TraceEvent") {
		t.Fatal("TypeScript observability:", err)
	}
	if code, err := codegentest.GenerateTSBundle(g, codegen.GenOptions{Start: "main", Observability: true, Document: true}); err != nil || !strings.Contains(string(code), "export function createDocument(") {
		t.Fatal("TypeScript Document observability:", err)
	}
}
