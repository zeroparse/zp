package tsgen_test

import (
	"math"
	"math/rand"
	"strconv"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/testing/codegentest"
)

// tsRunSpan is the threshold of the run sites in the library, which tsRunAt changes.
const tsRunSpan = "const RUN_HOT_SPAN = 64;\n"

// tsRunAt returns a mutation of a generated parser bundle that makes its run sites turn hot at
// span, after the replacements of mutation.
func tsRunAt(t *testing.T, span int, mutation [][2]string) func(string) string {
	return func(code string) string {
		for _, r := range append(mutation[:len(mutation):len(mutation)], [2]string{tsRunSpan, strings.Replace(tsRunSpan, "64", strconv.Itoa(span), 1)}) {
			if !strings.Contains(code, r[0]) {
				t.Fatalf("mutation: %q not in the library", r[0])
			}
			code = strings.Replace(code, r[0], r[1], 1)
		}
		return code
	}
}

// TestRunSitesTSMatchEngine checks that generated TypeScript parsers, whose run sites record and
// reuse their walks, give the engine's results on the run-site grammars. Short inputs run with the
// sites turning hot at the first walk that walks input again, long ones at the threshold of the
// library.
func TestRunSitesTSMatchEngine(t *testing.T) {
	t.Parallel()
	node := codegentest.RequireNode(t)
	if testing.Short() {
		t.Skip("runs generated TypeScript")
	}
	for _, m := range codegentest.TSParsersMismatches(t, node, codegentest.RunSiteCases(40, false), codegen.GenOptions{}, tsRunAt(t, 1, nil)) {
		t.Error(m)
	}
	codegentest.TSParsersCorpus(t, node, codegentest.RunSiteCases(4, true), codegen.GenOptions{})
}

// tsRunSiteMutations are the changes of the library for the negative controls, as in the Go
// runtime's: sites that never turn hot, records found under any environment, hits that keep the
// environment of the walk, records of value-free walks reused by walks that build lists, and
// records stored when their iteration ends that the end of the walk leaves as they are.
var tsRunSiteMutations = map[string][][2]string{
	"never hot": {{
		">= RUN_HOT_SPAN) c.hot = true;",
		">= RUN_HOT_SPAN) c.hot = false;",
	}},
	"ignore env": {{
		"    if (k === null) {\n      return noKey;\n",
		"    if (true) {\n      return noKey;\n",
	}},
	"keep env": {{
		"    if (w.key !== null) {\n      for (let i = 0; i < w.key.bind.length; i++) {\n",
		"    if (false) {\n      for (let i = 0; i < w.key!.bind.length; i++) {\n",
	}},
	// The key holds the environment as an object besides the values: walks under equal values
	// built apart share no record.
	"key by object": {{
		"    return { vals, h: memoKeyHash(0, 0, vals, this.control) };\n",
		"    return { vals: [...vals!, this.env as unknown as Value], h: memoKeyHash(0, 0, vals, this.control) };\n",
	}},
	// A lookup steps through every record of the site before it takes the one of its key.
	"scan keys": {{
		"    const r = this.recs.get(k.h)?.get(pos);\n",
		"    for (const m of this.recs.values()) {\n      control?.charge(m.size);\n    }\n    const r = this.recs.get(k.h)?.get(pos);\n",
	}},
	"any list": {{
		"if (r === undefined || (w.build && !r.list)) {",
		"if (r === undefined) {",
	}},
	"early store": tsSkipRunMutations["early store"],
}

// TestRunSitesTSLinear checks that grammars which restart walks of a run site from every position
// spend a budget linear in the input in generated TypeScript parsers, and that sites that never turn hot make them quadratic.
func TestRunSitesTSLinear(t *testing.T) {
	t.Parallel()
	node := codegentest.RequireNode(t)
	if testing.Short() {
		t.Skip("runs generated TypeScript")
	}
	sources := []string{
		"@start\nrule main = X* \"y\"\nrule X = [ab] bs \"c\" / [ab]\nrule bs = B*\nrule B = \"b\"\n",
		"@start\nrule main = (a / \"a\")* $$\nrule a = xs:(\"a\" / \"b\")* \"c\" => len($xs)\n",
		// A class that a choice tries again from every "[" of a line that does not close it: the
		// repetition in the class is a run site, so the tries after the first take its records.
		"@start\nrule main = (c / .)*\n" + `rule c = "[" ("\\" . / [^\]\\\n])* "]"` + "\n",
		"@start\nrule main = (r \"!\" / [ab])* $$\nrule r = (\"ab\" / \"b\")*\n",
	}
	linear := 3 // the sources whose spends are linear
	sizes := []int{100, 200, 400, 800}
	var inputs [][]string
	for i := range sources {
		var row []string
		for _, n := range sizes {
			switch i {
			case 0:
				row = append(row, "a"+strings.Repeat("b", n)+"y")
			case 1:
				row = append(row, strings.Repeat("a", n))
			case 2:
				row = append(row, strings.Repeat("[", n)+"\n")
			default:
				row = append(row, strings.Repeat("ab", n))
			}
		}
		inputs = append(inputs, row)
	}
	slopes := func(row []uint64) (lo, hi float64) {
		lo, hi = math.Inf(1), math.Inf(-1)
		for i := 1; i < len(row); i++ {
			s := math.Log2(float64(row[i]) / float64(row[i-1]))
			lo, hi = min(lo, s), max(hi, s)
		}
		return lo, hi
	}
	spends, _ := tsSpends(t, node, sources, inputs, []string{}, nil, "parseWithOptions")
	t.Logf("spends at %v: %v", sizes, spends)
	for i, row := range spends[:linear] {
		if _, hi := slopes(row); hi > 1.2 {
			t.Errorf("%q: spends %v grow faster than linearly", sources[i], row)
		}
	}
	cold, _ := tsSpends(t, node, sources, inputs, []string{}, tsRunSiteMutations["never hot"], "parseWithOptions")
	t.Logf("spends of sites that never turn hot: %v", cold)
	for i, row := range cold[:linear] {
		if lo, _ := slopes(row); lo < 1.7 {
			t.Errorf("%q, sites that never turn hot: spends %v are not quadratic", sources[i], row)
		}
	}
}

// TestRunSitesTSNegativeControls checks that the comparisons of the records detect a key without
// the environment, a hit that leaves the environment as it was, a list made of the record of a
// value-free walk and a record stored before its walk ends, on the inputs that reach them.
func TestRunSitesTSNegativeControls(t *testing.T) {
	t.Parallel()
	node := codegentest.RequireNode(t)
	if testing.Short() {
		t.Skip("runs generated TypeScript")
	}
	valueFree := struct{ Name, Src, Alphabet string }{"value-free walks", "rule main = (r \"!\" / r \"?\" / r \",\" / [ab])* $$\nrule r = (\"ab\" / \"b\")* => 1", "ab!?,"}
	for _, tc := range []struct{ mutation, grammar string }{
		{"ignore env", "two environments at one position"},
		{"keep env", "bindings that a record restores"},
		{"keep env", "a binding that the element does not read"},
		{"any list", "walks of a value-free twin"},
		{"any list", "lookahead walks before lists"},
		{"early store", "value-free walks"},
	} {
		for _, g := range append(codegentest.RunValueGrammars[:len(codegentest.RunValueGrammars):len(codegentest.RunValueGrammars)], valueFree) {
			if g.Name != tc.grammar {
				continue
			}
			inputs := codegentest.RunValueInputs[g.Name]
			if inputs == nil {
				inputs = codegentest.SkipRunInputs(g.Alphabet, 40, rand.New(rand.NewSource(1)))
			}
			cases := []codegentest.Case{{Name: g.Name, Src: g.Src, Inputs: inputs}}
			bundles := codegentest.TSBundles(t, cases, codegen.GenOptions{})
			want := codegentest.TSOutputs(t, node, cases, bundles, tsRunAt(t, 1, nil))
			got := codegentest.TSOutputs(t, node, cases, bundles, tsRunAt(t, 1, tsRunSiteMutations[tc.mutation]))
			if got == want {
				t.Errorf("mutation %s: no difference on the inputs of %s", tc.mutation, g.Name)
			}
		}
	}
}
