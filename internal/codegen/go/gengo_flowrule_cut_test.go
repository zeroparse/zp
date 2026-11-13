package gogen_test

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

func TestGeneratedTypedDirectCuts(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testGeneratedTypedDirectCuts)
}

func testGeneratedTypedDirectCuts(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated code")
	}
	for _, c := range codegentest.TypedDirectCutCases {
		g, err := parseTest(c.Src)
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		code, err := codegentest.GenerateCode(t, g, codegen.GenOptions{Package: "cuts", Start: "main", Types: true})
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		if !strings.Contains(string(code), "TParse(trules") {
			t.Fatalf("%s: typed library must be exercised", c.Name)
		}
		if !strings.Contains(string(code), "// main, invoked as by invoke (body inlined)") &&
			!strings.Contains(string(code), "// main, called as by invokePlain (body inlined)") {
			t.Fatalf("%s: main must use the direct typed route", c.Name)
		}
		if c.Name == "typed/direct_cut_callee" && !strings.Contains(string(code), "// part, called as by invokePlain (body inlined)") {
			t.Fatal("callee containing cuts must use the direct typed route")
		}
		for _, r := range mustCompile(t, c.Src).RuleList {
			if !codegentest.DirectCuts(r.Def.Expr) {
				continue
			}
			inlined := false
			for _, route := range []string{"invoked as by invoke", "called as by invokePlain"} {
				marker := "// " + r.Name + ", " + route + " (body inlined)"
				inlined = inlined || strings.Contains(string(code), marker)
			}
			if !inlined {
				t.Fatalf("%s: cut-bearing %s must use the direct typed route", c.Name, r.Name)
			}
		}
	}
	codegentest.GeneratedTypesCorpus(t, filepath.Join(runtime.GOROOT(), "bin", "go"), codegentest.TypedDirectCutCases, codegen.GenOptions{ConvertTypes: true})
}
