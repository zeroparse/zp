package print_test

import (
	"maps"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	tspass "zeroparse.dev/zp/internal/backend/codegen/typescript/pass"
	"zeroparse.dev/zp/internal/testing/codegentest"
)

// tsExecPassForms holds, for each TypeScript Exec pass that the corpus exercises, a text that only
// the form it makes prints in a generated parser. The corpus has no charges in a row, which
// TestFlowExecStepRunFlag covers.
var tsExecPassForms = map[string]string{
	tspass.PassMergedRecords: "p.unisolateMerge(",
	tspass.PassTestedInits:   " === undefined",
}

// TestTSExecPassesTurnOff checks that the flag of each TypeScript Exec pass turns it off: the parsers
// generated from the corpus print the form the pass makes, and print fewer of it without the pass.
func TestTSExecPassesTurnOff(t *testing.T) {
	t.Parallel()
	cases := codegentest.Representative(codegentest.GenCorpus(t))
	count := func(form string, noOpt ...string) int {
		n := 0
		for _, c := range cases {
			g, err := parseTest(c.Src)
			if err != nil {
				t.Fatal(err)
			}
			files, err := codegentest.GenerateTS(g, common.GenOptions{Start: "main", LimitControls: true, Recognize: true, NoOpt: noOpt})
			if err != nil {
				t.Fatalf("%s: %v", c.Name, err)
			}
			for _, f := range files {
				if f.Path == "parser.ts" {
					n += strings.Count(string(f.Content), form)
				}
			}
		}
		return n
	}
	for _, name := range slices.Sorted(maps.Keys(tsExecPassForms)) {
		form := tsExecPassForms[name]
		on, off := count(form), count(form, name)
		if on == 0 || off >= on {
			t.Errorf("%s: %q prints %d times with the pass and %d times without it", name, form, on, off)
		}
	}
}

// TestTSExecPassesKeepResultsAndSpends checks that the parsers generated without the TypeScript Exec
// passes give the engine's results, and that each input of the corpus needs the same least budget
// with and without them: the forms the passes make charge what the operations they replace charge.
func TestTSExecPassesKeepResultsAndSpends(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("runs generated TypeScript")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found")
	}
	off := []string{tspass.PassSpendRuns, tspass.PassMergedRecords, tspass.PassTestedInits}
	cases := codegentest.Representative(codegentest.GenCorpus(t))
	codegentest.TSParsersCorpus(t, node, cases, common.GenOptions{NoOpt: off})
	var sources []string
	var inputs [][]string
	for _, c := range cases {
		sources, inputs = append(sources, c.Src), append(inputs, c.Inputs)
	}
	with, _ := tsSpends(t, node, sources, inputs, []string{}, nil, "parseWithOptions")
	for _, name := range off {
		without, _ := tsSpends(t, node, sources, inputs, []string{}, nil, "parseWithOptions", name)
		for i := range with {
			if !slices.Equal(with[i], without[i]) {
				t.Errorf("%s: grammar %d: least budgets with the pass %v, without it %v", name, i, with[i], without[i])
			}
		}
	}
}
