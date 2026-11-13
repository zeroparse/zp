package tsgen_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegen/ts"
	"zeroparse.dev/zp/internal/gentest"
)

// TestEveryFlowFunctionRunsTheIRPipelines generates TypeScript parsers whose functions take every route
// from Flow IR (flat and framed rules, lines of an Operators Rule, outlined nodes) in the Node and
// recognizer tables, and checks from the dumps of "flow" and "exec" that every function printed from
// Exec IR went through the Flow pipeline and the Exec pipeline once per lowering.
func TestEveryFlowFunctionRunsTheIRPipelines(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "grammars", "cel", "cel.zp"))
	if err != nil {
		t.Fatal(err)
	}
	g, err := parseTest(string(src))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		opts codegen.GenOptions
	}{
		{"ordinary", codegen.GenOptions{Recognize: true}},
		{"types", codegen.GenOptions{Recognize: true, Types: true}},
		{"controls+observability", codegen.GenOptions{Recognize: true, LimitControls: true, Observability: true}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var dump bytes.Buffer
			o := c.opts
			o.PrintAfter, o.PrintTo, o.VerifyEachPass = "flow,exec", &dump, true
			if _, err := tsgen.GenerateParser(g, compileForGenerator, gentest.WithLib(o), nil); err != nil {
				t.Fatal(err)
			}
			gentest.CheckIRPipelines(t, dump.String(), "rules", "recRules")
		})
	}
}

// A Flow pass and an Exec pass that break the functions of one table are caught by the verifiers of
// Flow IR and Exec IR in each table: at the end of the run without VerifyEachPass, and by name right
// after the pass with it.
func TestVerifyEachPassNamesTheBreakingFlowAndExecPass(t *testing.T) {
	pg, err := parseTest(`rule main = (word / num / -" ")* $$
rule word = "if" / "else" / [a-z]+
rule num = [0-9]+
`)
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"rules", "recRules"} {
		for _, ir := range []string{"flow", "exec"} {
			t.Run(table+"/"+ir, func(t *testing.T) {
				seen := 0
				h := &tsgen.Hooks{}
				if ir == "flow" {
					h.FlowRun = gentest.BrokenFlowRun(table, &seen)
				} else {
					h.ExecRun = gentest.BrokenExecRun(table, &seen)
				}
				opts := codegen.GenOptions{Recognize: true}
				_, err := tsgen.GenerateParser(pg, compileForGenerator, gentest.WithLib(opts), h)
				if err == nil || !strings.Contains(err.Error(), "of table "+table+": ") || !strings.Contains(err.Error(), "verification:") || strings.Contains(err.Error(), "break-"+ir) {
					t.Fatalf("without VerifyEachPass: %v", err)
				}
				if seen == 0 {
					t.Fatalf("no function of the %s table ran through the %s pipeline", table, ir)
				}
				opts.VerifyEachPass = true
				_, err = tsgen.GenerateParser(pg, compileForGenerator, gentest.WithLib(opts), h)
				if err == nil || !strings.Contains(err.Error(), "of table "+table+": ") || !strings.Contains(err.Error(), "after pass break-"+ir+":") {
					t.Fatalf("with VerifyEachPass: %v", err)
				}
			})
		}
	}
}
