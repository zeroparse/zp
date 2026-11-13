package gogen_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

func TestGeneratedTypedOperatorsRuleBodies(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testGeneratedTypedOperatorsRuleBodies)
}

func testGeneratedTypedOperatorsRuleBodies(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated code")
	}
	calculator, err := os.ReadFile("../../../examples/calculator/calc.zp")
	if err != nil {
		t.Fatal(err)
	}
	cases := append([]codegentest.Case(nil), codegentest.TypedOperatorsRuleBodyCases...)
	cases = append(cases, codegentest.Case{Name: "typed/OperatorsRule_calculator", Src: string(calculator), Inputs: []string{"1", "1+2*3", "2^3^4", "-(1+2)", "1+", "(1+2", "é", ""}})
	cases = append(cases, codegentest.PrefixPartCutCorpus())
	for _, c := range cases {
		g, err := parseTest(c.Src)
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		opts := codegen.GenOptions{Package: "operatorsRulebody", Start: "main", Types: true, Recognize: true}
		code, err := codegentest.GenerateCode(t, g, opts)
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		if !strings.Contains(string(code), "TParse(trules") {
			t.Fatalf("%s: typed library is not exercised", c.Name)
		}
		inlined := strings.Contains(string(code), "(typed OperatorsRule body inlined)")
		fallback := strings.HasSuffix(c.Name, "_fallback")
		if inlined == fallback {
			t.Fatalf("%s: incorrect OperatorsRule body route (inline=%v)", c.Name, inlined)
		}
		if c.Name == "typed/OperatorsRule_mixed_routes" && strings.Count(string(code), "(typed OperatorsRule body inlined)") != 3 {
			t.Fatal("mixed routes: all three operands must use unfinished bodies")
		}
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	codegentest.GeneratedTypesCorpus(t, goBin, cases, codegen.GenOptions{})
	codegentest.GeneratedTypesCorpus(t, goBin, cases, codegen.GenOptions{ConvertTypes: true})
}
