package gogen_test

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

func TestGeneratedTypedFramedCuts(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testGeneratedTypedFramedCuts)
}

func testGeneratedTypedFramedCuts(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated code")
	}
	cases := codegentest.TypedFramedCutCases(t)
	for _, c := range cases {
		g, err := parseTest(c.Src)
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		opts := codegen.GenOptions{Package: "framedcuts", Start: "main", Types: true, Recognize: true}
		code, err := codegentest.GenerateCode(t, g, opts)
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		marker := "(typed OperatorsRule body inlined)"
		if c.Name == "typed/framed_cut_LR_elements" {
			marker = "(typed LR body inlined)"
		}
		if !strings.Contains(string(code), "TParse(trules") || !strings.Contains(string(code), marker) {
			t.Fatalf("%s: cut-bearing typed body must be exercised", c.Name)
		}
	}
	cases = append(cases, codegentest.PrefixPartCutCorpus())
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	codegentest.GeneratedTypesCorpus(t, goBin, cases, codegen.GenOptions{})
	codegentest.GeneratedTypesCorpus(t, goBin, cases, codegen.GenOptions{ConvertTypes: true})
}
