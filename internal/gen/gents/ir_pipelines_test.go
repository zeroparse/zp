package gents_test

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gen/gents"
	"zeroparse.dev/zp/internal/gentest"
)

// A Flow pass that breaks the functions of one table is caught by the Flow verifier in each table: at
// the end of the run without VerifyEachPass, and by name right after the pass with it.
func TestVerifyEachPassNamesTheBreakingFlowPass(t *testing.T) {
	pg, err := parseTest(`rule main = (word / num / -" ")* $$
rule word = "if" / "else" / [a-z]+
rule num = [0-9]+
`)
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"rules", "recRules"} {
		t.Run(table, func(t *testing.T) {
			seen := 0
			h := &gents.Hooks{FlowRun: gentest.BrokenFlowRun(table, &seen)}
			opts := gen.GenOptions{Recognize: true}
			_, err := gents.GenerateParser(pg, compileForGenerator, opts, h)
			if err == nil || !strings.Contains(err.Error(), "of table "+table+": ") || !strings.Contains(err.Error(), "verification:") || strings.Contains(err.Error(), "break-flow") {
				t.Fatalf("without VerifyEachPass: %v", err)
			}
			if seen == 0 {
				t.Fatalf("no function of the %s table ran through the Flow pipeline", table)
			}
			opts.VerifyEachPass = true
			_, err = gents.GenerateParser(pg, compileForGenerator, opts, h)
			if err == nil || !strings.Contains(err.Error(), "of table "+table+": ") || !strings.Contains(err.Error(), "after pass break-flow:") {
				t.Fatalf("with VerifyEachPass: %v", err)
			}
		})
	}
}
