package gogen_test

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

func TestGeneratedTypedFrameLayouts(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testGeneratedTypedFrameLayouts)
}

func testGeneratedTypedFrameLayouts(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated code")
	}
	cases := append([]codegentest.Case(nil), codegentest.TypedFrameLayoutCases...)
	for _, c := range codegentest.TypedLRBodyCases {
		if c.Name == "typed/LR_unreachable_capture" {
			cases = append(cases, c)
		}
	}
	for _, c := range codegentest.TypedOperatorsRuleBodyCases {
		if c.Name == "typed/OperatorsRule_unreachable_capture" {
			cases = append(cases, c)
		}
	}
	for _, c := range cases {
		g, err := parseTest(c.Src)
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		opts := codegen.GenOptions{Package: "layouts", Start: "main", Types: true, Recognize: true}
		candidate, err := codegentest.GenerateCode(t, g, opts)
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		marker := "(typed OperatorsRule body inlined)"
		if strings.Contains(c.Name, "/LR_") {
			marker = "(typed LR body inlined)"
		}
		if !strings.Contains(string(candidate), marker) {
			t.Fatalf("%s: candidate route not exercised", c.Name)
		}
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	codegentest.GeneratedTypesCorpus(t, goBin, cases, codegen.GenOptions{})
	codegentest.GeneratedTypesCorpus(t, goBin, cases, codegen.GenOptions{ConvertTypes: true})
}
