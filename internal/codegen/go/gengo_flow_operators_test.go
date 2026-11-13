package gogen_test

import (
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegen/go"
	"zeroparse.dev/zp/internal/codegentest"
	"zeroparse.dev/zp/internal/compile"
)

// The lines of an Operators Rule in the Node and recognizer tables are printed from Flow IR. The tests
// below check the routes of the lines and run the parsers of the corpus against the engine; the generated
// text of the checked-in grammars with an Operators Rule is frozen in testdata/output_manifest.txt of the alpha-parsers repository.

// TestOperatorsLinesWithRecoveryPrintFromFlow checks that a line with recovery is printed from Flow IR in the
// Node and recognizer tables (a line Flow IR does not print fails the generation).
func TestOperatorsLinesWithRecoveryPrintFromFlow(t *testing.T) {
	t.Parallel()
	codegentest.PinOrdinary(t)
	for _, c := range codegentest.OperatorsLineRecoveryCases {
		t.Run(c.Name, func(t *testing.T) {
			g, err := parseTest(c.Src)
			if err != nil {
				t.Fatal(err)
			}
			flowed := map[string]int{}
			gh := &gogen.Hooks{}
			codegentest.SetGenHooks(t, gh)
			gh.OperatorsRoute = func(table string, r *compile.Rule) { flowed[table+"/"+r.Name]++ }
			if _, err := codegentest.GenerateCode(t, g, codegen.GenOptions{Start: "main", Package: "gen", Recognize: true}); err != nil {
				t.Fatal(err)
			}
			for _, table := range []string{"rules", "recRules"} {
				if flowed[table+"/"+c.Rule] == 0 {
					t.Errorf("no line of %s is printed from Flow IR in the %s table (%v)", c.Rule, table, flowed)
				}
			}
		})
	}
}
