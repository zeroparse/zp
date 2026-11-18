//go:build !zp_budget_controls_reference

package engine

import (
	"zeroparse.dev/zp/internal/testing/codegentest"

	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/pkg/runtime"
	"zeroparse.dev/zp/pkg/runtime/input"
)

func TestBudgetLazyClosureSavedFlagsAndOptions(t *testing.T) {
	t.Parallel()
	original := mustCompile(t, `rule main = first:item rest:(-"," value:item)* $$ => concat(list($first), map($rest, (e) => e.$value))
rule item = [a-z]+`)
	flags := make([]compile.RuleFlags, len(original.Static().RuleList))
	for i, r := range original.Static().RuleList {
		flags[i] = compile.RuleFlags{Memo: r.Memo, Leader: r.Leader, Positional: r.Positional, Twin: r.TwinOK}
	}
	for _, savedFlags := range [][]compile.RuleFlags{nil, flags} {
		for _, noProjections := range []bool{false, true} {
			p, err := build(original.Static().Grammar, compile.Options{NoProjections: noProjections}, savedFlags)
			if err != nil {
				t.Fatal(err)
			}
			for _, unit := range []input.PositionUnit{input.CodePoints, input.Bytes} {
				o := ParseOptions{Unit: unit}
				plain, plainErr := p.ParseWith("main", "abc,def", o)
				o.Limits = runtime.Limits{Compute: 1000000}
				active, activeErr := p.ParseWith("main", "abc,def", o)
				if codegentest.ResultJSON(plain, plainErr) != codegentest.ResultJSON(active, activeErr) || activeErr != nil {
					t.Fatalf("options changed output: %v", activeErr)
				}
				o.Recognize = true
				if n, err := p.ParseWith("main", "abc,def", o); n != nil || err != nil {
					t.Fatalf("recognition options: %v", err)
				}
			}
			if p.budgetProgram == nil || p.budgetProgram.Static().NoProjections != noProjections {
				t.Fatal("projection policy changed")
			}
		}
	}
}
