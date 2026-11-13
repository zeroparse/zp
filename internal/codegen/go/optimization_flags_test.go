package gogen_test

import (
	"fmt"
	"maps"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
	genopt "zeroparse.dev/zp/internal/ir/opt"
)

// optimizationCases are grammars on which the optimizations of lean typed Go act, with inputs at their
// edges: guards that skip alternatives, value-free repetitions of classes, runs of classes, classes of
// many ranges, and literals around the length that is compared in place.
func optimizationCases() []codegentest.Case {
	alphabet := append(strings.Split("abcdefghijklmnopqrstuvwxyz0123456789 _é日", ""), "\xff", "\xe6\x97")
	cases := []codegentest.Case{
		{Name: "guards of words", Src: `rule main = (word / num / ident / -" ")* $$
rule word = "if" / "else" / "while" / "fn"
rule num = [0-9]+
rule ident = [a-z] [a-z0-9_]*
`, Inputs: []string{"if else while fn", "if1 x9", "123 abc", "", " ", "else9", "é", "while_", "fn fn fn", "zz 01 if"}},
		{Name: "scans", Src: `rule main = word (-[ ]* word)* -.* $$
rule word = [a-z]+
`, Inputs: []string{"a b c", "abc   def  ", "a  b ~~~ tail", "", "a", " ", "a\xff", "aé b"}},
		{Name: "class runs", Src: `rule main = (-([a-c] [d-f] [g-h] [0-9]) / -([x-z] [é] [日] "!") / -" ")* $$
`, Inputs: []string{"adg1 xé日!", "adg", "ad", "xé", "adh9 cfg0", "x日", "", "adg1adg1", "\xffdg1"}},
		{Name: "class tables", Src: `rule main = ([a-cf-hk-mp-rt-vx-z0-35-7]+ / [^a-cf-hk-mp-rt-vx-z0-35-7 ]+ / -" ")* $$
`, Inputs: []string{"abcfghklm", "defij", "abc def 012", "4 8", "xyz789", "", "é日", "\xff", "a4b"}},
		{Name: "inline literals", Src: `rule main = (l:lit / -" ")* $$
rule lit = "a" / "ab" / "abc" / "abcd" / "abcde" / "abcdef" / "日本" / "日本語日本語"
`, Inputs: []string{"a ab abc abcd abcde abcdef", "abcdefg", "日本 日本語日本語", "日本語", "ab abc", "abcd abcd", "", "x", "abcde"}},
	}
	rng := rand.New(rand.NewSource(7))
	for i := range cases {
		for range 120 {
			var b strings.Builder
			for range rng.Intn(14) {
				b.WriteString(alphabet[rng.Intn(len(alphabet))])
			}
			cases[i].Inputs = append(cases[i].Inputs, b.String())
		}
	}
	return append(cases, guardedClassCases()...)
}

// optimizationTests says for each registered optimization the test that shows a parser generated
// without it returns what the engine does, or "" where this file's TestOptimizationDisabledParity does.
// A flag that is missing here fails TestOptimizationsHaveTests, so a new optimization comes with a test.
var optimizationTests = map[string]string{
	"terminals":              "TestTerminalProgramDisabledParity",
	"text-dispatch":          "TestLiteralTextPredicateDispatchDisabledParity",
	"field-compare":          "TestGeneratedTextFieldComparisons",
	"guards":                 "",
	"guarded-classes":        "TestGuardedClassDisabledParity",
	"scans":                  "",
	"class-runs":             "",
	"class-tables":           "",
	"inline-literals":        "",
	"method-sharing":         "TestGeneratedUnsharedParsersMatchEngine",
	"literal-sharing":        "TestGeneratedUnsharedParsersMatchEngine",
	"ts-literal-sharing":     "TestGeneratedSharingIdentity (internal/codegen)",
	"typed-object-slabs":     "TestGeneratedTypedOutputRetentionFixture",
	"typed-list-slabs":       "TestGeneratedTypedOutputRetentionFixture",
	"typed-output-bootstrap": "TestGeneratedTypedOutputRetentionFixture",
	"sparse-memo":            "TestGeneratedTSSparseMemoControls (tsgen)",
}

// outputFlagTests says for each registered optimization that changes the output (ChangesOutput) the
// test of the contract it changes. Such an optimization is not in optimizationTests: a parser
// generated without it is the ordinary one, and a parser generated with it is not compared with the
// engine as the ordinary one is.
var outputFlagTests = map[string]string{}

// flagTestProblems returns what is wrong with the tests that the flags name: an optimization without
// a test of its disabled form (or, if it changes the output, of its changed contract), an output
// change that is also in the parity tests, a test that names an unregistered optimization, and a
// contract test for a flag that does not change the output.
func flagTestProblems(flags []genopt.Flag, keeps, changes map[string]string) []string {
	var out []string
	known := map[string]genopt.Flag{}
	for _, f := range flags {
		known[f.Name] = f
		if f.ChangesOutput {
			if changes[f.Name] == "" {
				out = append(out, fmt.Sprintf("the optimization %s changes the output and names no test of its contract", f.Name))
			}
			if _, ok := keeps[f.Name]; ok {
				out = append(out, fmt.Sprintf("the optimization %s changes the output, so the parity tests of the flags that keep behavior do not apply to it", f.Name))
			}
		} else if _, ok := keeps[f.Name]; !ok {
			out = append(out, fmt.Sprintf("the optimization %s has no test of its disabled form", f.Name))
		}
		if f.Doc == "" {
			out = append(out, fmt.Sprintf("the optimization %s has no description", f.Name))
		}
	}
	for _, tests := range []map[string]string{keeps, changes} {
		for name := range tests {
			if _, ok := known[name]; !ok {
				out = append(out, fmt.Sprintf("a test names %s, which is not a registered optimization", name))
			}
		}
	}
	for name := range changes {
		if f, ok := known[name]; ok && !f.ChangesOutput {
			out = append(out, fmt.Sprintf("%s names a contract test but does not change the output", name))
		}
	}
	return out
}

// TestOptimizationsHaveTests fails for an optimization that is registered and has no test of its
// disabled form (or, if it changes the output, of its changed contract), and for a test that names an
// optimization that does not exist.
func TestOptimizationsHaveTests(t *testing.T) {
	for _, p := range flagTestProblems(genopt.Flags, optimizationTests, outputFlagTests) {
		t.Error(p)
	}
}

// A registry with an optimization that changes the output, which no flag of the real one does yet,
// shows the test check on its positive path: such a flag must name a contract test in place of a
// disabled-form test (TestOutputChangingFlagRegistry in internal/ir/opt checks the opt-in rule).
func TestOutputChangingFlagChecks(t *testing.T) {
	fake := genopt.Flag{Name: "fake-output", Kind: genopt.Printer, Doc: "changes the generated API", ChangesOutput: true}
	flags := append(slices.Clone(genopt.Flags), fake)

	has := func(problems []string, want string) bool {
		return slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, want) })
	}
	if got := flagTestProblems(flags, optimizationTests, outputFlagTests); !has(got, "fake-output changes the output and names no test") {
		t.Errorf("no contract test: %v", got)
	}
	contract := map[string]string{"fake-output": "TestFakeOutputContract"}
	if got := flagTestProblems(flags, optimizationTests, contract); len(got) != 0 {
		t.Errorf("with a contract test: %v, want none", got)
	}
	both := maps.Clone(optimizationTests)
	both["fake-output"] = ""
	if got := flagTestProblems(flags, both, contract); !has(got, "parity tests of the flags that keep behavior do not apply") {
		t.Errorf("in the parity tests too: %v", got)
	}
	if got := flagTestProblems(genopt.Flags, optimizationTests, contract); !has(got, "not a registered optimization") {
		t.Errorf("a contract test for an unregistered flag: %v", got)
	}
	if got := flagTestProblems(genopt.Flags, optimizationTests, map[string]string{"guards": "TestX"}); !has(got, "does not change the output") {
		t.Errorf("a contract test for a flag that keeps behavior: %v", got)
	}
}

// The guide lists every registered optimization, with the name that -no-opt takes, and the row
// carries the kind and the default of the registry (and "changes output" for a flag that does).
func TestOptimizationsAreInTheGuide(t *testing.T) {
	doc, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "guide", "code-generation.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range genopt.Flags {
		prefix := "| `" + f.Name + "` | "
		row := ""
		for _, line := range strings.Split(string(doc), "\n") {
			if strings.HasPrefix(line, prefix) {
				row = line
			}
		}
		if row == "" {
			t.Errorf("the guide has no row for the optimization %s", f.Name)
			continue
		}
		if want := prefix + f.Kind.String() + " | " + f.DefaultText() + " | "; !strings.HasPrefix(row, want) {
			t.Errorf("the guide row of %s does not start with %q (the kind and the default of the registry): %.80s", f.Name, want, row)
		}
		if f.ChangesOutput && !strings.Contains(row, "changes output") {
			t.Errorf("the guide row of %s does not say that it changes the output", f.Name)
		}
	}
}

// off returns the options for a lean typed parser with the optimizations off.
func off(names ...string) codegen.GenOptions {
	return codegen.GenOptions{Types: true, TypedOnly: true, NoOpt: names}
}

// generateShowingIR returns the code of a parser and the Core IR after the guards pass, which shows
// what the passes up to it did even where the code does not.
func generateShowingIR(t *testing.T, g *grammar.Grammar, o codegen.GenOptions) ([]byte, string) {
	t.Helper()
	var ir strings.Builder
	o.PrintAfter, o.PrintTo = "guards", &ir
	code, err := codegentest.GenerateCode(t, g, o)
	if err != nil {
		t.Fatal(err)
	}
	return code, ir.String()
}

// optInValues is the value that TestOptimizationDisabledParity gives an optimization that the speed
// profile leaves off, and that is not 1.
var optInValues = map[string]int{"class-tables": 5}

// TestOptimizationDisabledParity compares lean typed parsers generated without an optimization with
// the engine, in both position units, for the optimizations that no other test covers; and checks
// that turning it off changes the generated code or the Core IR, so that a flag that does nothing is caught.
// For an optimization that the speed profile leaves off, it compares the parsers generated with it on,
// and checks that turning it on changes them.
func TestOptimizationDisabledParity(t *testing.T) {
	t.Parallel()
	codegentest.PinOrdinary(t)
	cases := optimizationCases()
	for _, f := range genopt.Flags {
		if optimizationTests[f.Name] != "" || f.ChangesOutput {
			continue
		}
		variant := off(f.Name)
		if f.Default == 0 {
			v := 1
			if o, ok := optInValues[f.Name]; ok {
				v = o
			}
			variant = off()
			variant.Opt = map[string]int{f.Name: v}
		}
		t.Run(f.Name, func(t *testing.T) {
			changed := false
			for _, c := range cases {
				g, err := parseTest(c.Src)
				if err != nil {
					t.Fatal(err)
				}
				on, onIR := generateShowingIR(t, g, off())
				without, withoutIR := generateShowingIR(t, g, variant)
				changed = changed || string(on) != string(without) || onIR != withoutIR
			}
			if !changed {
				t.Errorf("generating with the %s setting changes no parser of the cases: the flag does nothing there", f.Name)
			}
			codegentest.GeneratedTypesCorpus(t, filepath.Join(runtime.GOROOT(), "bin", "go"), cases, variant, 0)
		})
	}
}

// TestIntegerOptimizationValuesParity compares parsers generated with other values of the optimizations
// that take a count with the engine: any value is correct, and a profile may pick it.
func TestIntegerOptimizationValuesParity(t *testing.T) {
	t.Parallel()
	codegentest.PinOrdinary(t)
	cases := optimizationCases()
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	for _, f := range genopt.Flags {
		if !f.Int || f.ChangesOutput {
			continue
		}
		for _, v := range []int{1, 2, 3, 5, 8} {
			if v == f.Default {
				continue
			}
			t.Run(fmt.Sprintf("%s=%d", f.Name, v), func(t *testing.T) {
				opts := off()
				opts.Opt = map[string]int{f.Name: v}
				codegentest.GeneratedTypesCorpus(t, goBin, cases, opts, 0)
			})
		}
	}
}

// TestOptimizationCombinationsParity compares parsers generated with every optimization of the Core
// passes and of lowering off, and with a seeded sample of pairs off, with the engine.
func TestOptimizationCombinationsParity(t *testing.T) {
	t.Parallel()
	codegentest.PinOrdinary(t)
	var names []string
	for _, f := range genopt.Flags {
		if f.Kind != genopt.Runtime && !f.ChangesOutput {
			names = append(names, f.Name)
		}
	}
	cases := optimizationCases()
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	t.Run("all", func(t *testing.T) { codegentest.GeneratedTypesCorpus(t, goBin, cases, off(names...), 0) })
	rng := rand.New(rand.NewSource(11))
	pairs := 3
	if codegentest.Long() {
		pairs = 12
	}
	for range pairs {
		a, b := names[rng.Intn(len(names))], names[rng.Intn(len(names))]
		t.Run(a+"+"+b, func(t *testing.T) { codegentest.GeneratedTypesCorpus(t, goBin, cases, off(a, b), 0) })
	}
}

// nodeRouteModes are the ways a Node or recognizer parser is generated, which take routes other than
// lean typed Go: the ordinary Node parser, one with observability, one with limit controls, and the
// recognizer.
var nodeRouteModes = []struct {
	name string
	opts codegen.GenOptions
}{
	{"plain", codegen.GenOptions{}},
	{"observability", codegen.GenOptions{Observability: true}},
	{"limits", codegen.GenOptions{LimitControls: true}},
	{"recognize", codegen.GenOptions{Recognize: true}},
}

// TestOptimizationDisabledNodeRouteParity compares Node and recognizer parsers generated with an
// optimization off with the engine, in every mode of nodeRouteModes. The flag tests above generate
// lean typed Go; these are the other routes, where a pass applies in some modes and not in others.
func TestOptimizationDisabledNodeRouteParity(t *testing.T) {
	t.Parallel()
	codegentest.PinOrdinary(t)
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	var all []string
	for _, f := range genopt.Flags {
		if f.Kind != genopt.Runtime && !f.ChangesOutput {
			all = append(all, f.Name)
		}
	}
	sets := [][]string{{"guards"}, {"scans"}, {"text-dispatch"}, {"inline-literals"}, {"guarded-classes"}, all}
	for _, mode := range nodeRouteModes {
		for _, set := range sets {
			name := mode.name + "/" + strings.Join(set, "+")
			if len(set) == len(all) {
				name = mode.name + "/all"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				o := mode.opts
				o.NoOpt = set
				codegentest.GeneratedParsersCorpus(t, goBin, codegentest.FlagParityCases(), o)
			})
		}
	}
}

// TestOptimizationDisabledNodeRouteNegativeControl shows that the comparison of the test above
// fails for a parser that is wrong: with the guards off and a literal of the grammar changed in the
// generated code, the parser differs from the engine on the inputs that contain the literal.
func TestOptimizationDisabledNodeRouteNegativeControl(t *testing.T) {
	t.Parallel()
	codegentest.PinOrdinary(t)
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	for _, mode := range nodeRouteModes {
		t.Run(mode.name, func(t *testing.T) {
			t.Parallel()
			o := mode.opts
			o.NoOpt = []string{"guards"}
			var changed atomic.Bool
			mutate := func(code []byte) []byte {
				out := strings.ReplaceAll(string(code), `"while"`, `"whilx"`)
				if out != string(code) {
					changed.Store(true)
				}
				return []byte(out)
			}
			if got := codegentest.GeneratedParsersMismatches(t, goBin, codegentest.FlagParityCases(), o, mutate); len(got) == 0 {
				t.Errorf("a parser with a changed literal matches the engine")
			}
			if !changed.Load() {
				t.Errorf("the generated code has no literal \"while\" to change: the control proves nothing")
			}
		})
	}
}

// TestProfilesParity compares lean typed parsers generated with each profile with the engine, in both
// position units: a profile only fixes flag values, so it changes the code and never the results.
func TestProfilesParity(t *testing.T) {
	t.Parallel()
	codegentest.PinOrdinary(t)
	cases := optimizationCases()
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	for _, name := range genopt.Profiles() {
		t.Run(name, func(t *testing.T) {
			opts := off()
			opts.Optimize = name
			codegentest.GeneratedTypesCorpus(t, goBin, cases, opts, 0)
		})
	}
}

// profileRouteModes are the Go routes on which the profiles are compared with the engine beyond lean
// typed Go: the ordinary Node parser, with observability, with limit controls, the recognizer, and the
// Document runner.
var profileRouteModes = append(slices.Clone(nodeRouteModes), struct {
	name string
	opts codegen.GenOptions
}{"document", codegen.GenOptions{Document: true}})

// TestProfilesRouteParity compares Node, recognizer and Document parsers generated with each profile
// other than speed with the engine, in every mode of profileRouteModes and in both position units. A
// profile only fixes flag values, so it must change the code and never the results on any route;
// TestProfilesParity does the same for lean typed Go, TestProfilesStreamParity for ParseStream, and
// the tsgen package for TypeScript.
func TestProfilesRouteParity(t *testing.T) {
	t.Parallel()
	codegentest.PinOrdinary(t)
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	for _, name := range genopt.Profiles()[1:] {
		for _, mode := range profileRouteModes {
			t.Run(name+"/"+mode.name, func(t *testing.T) {
				t.Parallel()
				o := mode.opts
				o.Optimize = name
				codegentest.GeneratedParsersCorpus(t, goBin, codegentest.FlagParityCases(), o)
			})
		}
	}
}

// TestProfilesRouteNegativeControl shows that the comparison of the test above notices a wrong parser
// on the Document route, which only this file's tests compare: a changed literal differs from the
// engine, and the unchanged parsers do not.
func TestProfilesRouteNegativeControl(t *testing.T) {
	t.Parallel()
	codegentest.PinOrdinary(t)
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	o := codegen.GenOptions{Document: true, Optimize: "size"}
	if got := codegentest.GeneratedParsersMismatches(t, goBin, codegentest.FlagParityCases(), o, nil); len(got) != 0 {
		t.Fatalf("the unchanged Document parsers differ from the engine:\n%s", got[0])
	}
	var changed atomic.Bool
	mutate := func(code []byte) []byte {
		out := strings.ReplaceAll(string(code), `"while"`, `"whilx"`)
		if out != string(code) {
			changed.Store(true)
		}
		return []byte(out)
	}
	got := codegentest.GeneratedParsersMismatches(t, goBin, codegentest.FlagParityCases(), o, mutate)
	if !changed.Load() {
		t.Fatal(`the generated code has no literal "while" to change: the control proves nothing`)
	}
	if len(got) == 0 {
		t.Errorf("a Document parser with a changed literal matches the engine")
	}
}

// TestProfilesStreamParity compares ParseStream generated with each profile other than speed with the
// engine's: the elements, the reads before each and the final error, as TestGeneratedStreamMatchesEngine does.
func TestProfilesStreamParity(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, func(t *testing.T) {
		if testing.Short() {
			t.Skip("builds generated code")
		}
		goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
		cases := append([]codegentest.Case(nil), streamCorpus...)
		for _, c := range codegentest.GenCorpus(t) {
			if strings.Contains(c.Src, "#stream") {
				cases = append(cases, c)
			}
		}
		for _, name := range genopt.Profiles()[1:] {
			t.Run(name, func(t *testing.T) { testGeneratedStreams(t, goBin, cases, codegen.GenOptions{Optimize: name}) })
		}
	})
}

// Each profile other than speed changes the generated code, so that a profile that sets nothing is caught.
func TestProfilesChangeTheCode(t *testing.T) {
	t.Parallel()
	codegentest.PinOrdinary(t)
	for _, name := range genopt.Profiles()[1:] {
		changed := false
		for _, c := range optimizationCases() {
			g, err := parseTest(c.Src)
			if err != nil {
				t.Fatal(err)
			}
			base, _ := generateShowingIR(t, g, off())
			o := off()
			o.Optimize = name
			got, _ := generateShowingIR(t, g, o)
			changed = changed || string(base) != string(got)
		}
		if !changed {
			t.Errorf("the profile %s generates the same parsers as speed for every case", name)
		}
	}
}

// The guide lists every profile with the flags it sets.
func TestProfilesAreInTheGuide(t *testing.T) {
	doc, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "guide", "code-generation.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range genopt.Profiles() {
		if !strings.Contains(string(doc), "| `"+n+"` | ") {
			t.Errorf("the guide has no row for the profile %s", n)
		}
		for flag := range genopt.ProfileValues(n) {
			row := ""
			for _, line := range strings.Split(string(doc), "\n") {
				if strings.HasPrefix(line, "| `"+n+"` | ") {
					row = line
				}
			}
			if !strings.Contains(row, "`"+flag+"`") {
				t.Errorf("the guide row of the profile %s does not name the flag %s that it sets", n, flag)
			}
		}
	}
}
