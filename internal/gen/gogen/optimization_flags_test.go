package gogen_test

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/gen"
	genopt "zeroparse.dev/zp/internal/gen/opt"
	"zeroparse.dev/zp/internal/gentest"
)

// optimizationCases are grammars on which the optimizations of lean typed Go act, with inputs at their
// edges: guards that skip alternatives, value-free repetitions of classes, runs of classes, classes of
// many ranges, and literals around the length that is compared in place.
func optimizationCases() []gentest.Case {
	alphabet := append(strings.Split("abcdefghijklmnopqrstuvwxyz0123456789 _é日", ""), "\xff", "\xe6\x97")
	cases := []gentest.Case{
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
	"ts-literal-sharing":     "TestGeneratedSharingIdentity (internal/gen)",
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

// TestOptimizationsHaveTests fails for an optimization that is registered and has no test of its
// disabled form (or, if it changes the output, of its changed contract), and for a test that names an
// optimization that does not exist.
func TestOptimizationsHaveTests(t *testing.T) {
	for _, f := range genopt.Flags {
		if f.ChangesOutput {
			if outputFlagTests[f.Name] == "" {
				t.Errorf("the optimization %s changes the output and names no test of its contract", f.Name)
			}
			if _, ok := optimizationTests[f.Name]; ok {
				t.Errorf("the optimization %s changes the output, so the parity tests of the flags that keep behavior do not apply to it", f.Name)
			}
		} else if _, ok := optimizationTests[f.Name]; !ok {
			t.Errorf("the optimization %s has no test of its disabled form", f.Name)
		}
		if f.Doc == "" {
			t.Errorf("the optimization %s has no description", f.Name)
		}
	}
	for _, tests := range []map[string]string{optimizationTests, outputFlagTests} {
		for name := range tests {
			if !slices.Contains(genopt.Names(), name) {
				t.Errorf("a test names %s, which is not a registered optimization", name)
			}
		}
	}
	for name := range outputFlagTests {
		if f, ok := lookupFlag(name); ok && !f.ChangesOutput {
			t.Errorf("%s names a contract test but does not change the output", name)
		}
	}
}

// The guide lists every registered optimization, with the name that -no-opt takes.
func TestOptimizationsAreInTheGuide(t *testing.T) {
	doc, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "guide", "code-generation.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range genopt.Names() {
		if !strings.Contains(string(doc), "| `"+n+"` | ") {
			t.Errorf("the guide has no row for the optimization %s", n)
		}
	}
}

func lookupFlag(name string) (genopt.Flag, bool) {
	for _, f := range genopt.Flags {
		if f.Name == name {
			return f, true
		}
	}
	return genopt.Flag{}, false
}

// off returns the options for a lean typed parser with the optimizations off.
func off(names ...string) gen.GenOptions {
	return gen.GenOptions{Types: true, TypedOnly: true, NoOpt: names}
}

// generateShowingIR returns the code of a parser and the Core IR after the guards pass, which shows
// what the passes up to it did even where the code does not.
func generateShowingIR(t *testing.T, g *grammar.Grammar, o gen.GenOptions) ([]byte, string) {
	t.Helper()
	var ir strings.Builder
	o.PrintAfter, o.PrintTo = "guards", &ir
	code, err := gentest.GenerateCode(t, g, o)
	if err != nil {
		t.Fatal(err)
	}
	return code, ir.String()
}

// TestOptimizationDisabledParity compares lean typed parsers generated without an optimization with
// the engine, in both position units, for the optimizations that no other test covers; and checks
// that turning it off changes the generated code or the Core IR, so that a flag that does nothing is caught.
func TestOptimizationDisabledParity(t *testing.T) {
	t.Parallel()
	gentest.PinOrdinary(t)
	cases := optimizationCases()
	for _, f := range genopt.Flags {
		if optimizationTests[f.Name] != "" || f.ChangesOutput {
			continue
		}
		t.Run(f.Name, func(t *testing.T) {
			changed := false
			for _, c := range cases {
				g, err := parseTest(c.Src)
				if err != nil {
					t.Fatal(err)
				}
				on, onIR := generateShowingIR(t, g, off())
				without, withoutIR := generateShowingIR(t, g, off(f.Name))
				changed = changed || string(on) != string(without) || onIR != withoutIR
			}
			if !changed {
				t.Errorf("generating without %s changes no parser of the cases: the flag does nothing there", f.Name)
			}
			gentest.GeneratedTypesCorpus(t, filepath.Join(runtime.GOROOT(), "bin", "go"), cases, off(f.Name), 0)
		})
	}
}

// TestIntegerOptimizationValuesParity compares parsers generated with other values of the optimizations
// that take a count with the engine: any value is correct, and a profile may pick it.
func TestIntegerOptimizationValuesParity(t *testing.T) {
	t.Parallel()
	gentest.PinOrdinary(t)
	cases := optimizationCases()
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	for _, f := range genopt.Flags {
		if !f.Int || f.ChangesOutput {
			continue
		}
		for _, v := range []int{1, 2, 3, 8} {
			if v == f.Default {
				continue
			}
			t.Run(fmt.Sprintf("%s=%d", f.Name, v), func(t *testing.T) {
				opts := off()
				opts.Opt = map[string]int{f.Name: v}
				gentest.GeneratedTypesCorpus(t, goBin, cases, opts, 0)
			})
		}
	}
}

// TestOptimizationCombinationsParity compares parsers generated with every optimization of the Core
// passes and of lowering off, and with a seeded sample of pairs off, with the engine.
func TestOptimizationCombinationsParity(t *testing.T) {
	t.Parallel()
	gentest.PinOrdinary(t)
	var names []string
	for _, f := range genopt.Flags {
		if f.Kind != genopt.Runtime && !f.ChangesOutput {
			names = append(names, f.Name)
		}
	}
	cases := optimizationCases()
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	t.Run("all", func(t *testing.T) { gentest.GeneratedTypesCorpus(t, goBin, cases, off(names...), 0) })
	rng := rand.New(rand.NewSource(11))
	pairs := 3
	if gentest.Long() {
		pairs = 12
	}
	for range pairs {
		a, b := names[rng.Intn(len(names))], names[rng.Intn(len(names))]
		t.Run(a+"+"+b, func(t *testing.T) { gentest.GeneratedTypesCorpus(t, goBin, cases, off(a, b), 0) })
	}
}

// nodeRouteModes are the ways a Node or recognizer parser is generated, which take routes other than
// lean typed Go: the ordinary Node parser, one with observability, one with limit controls, and the
// recognizer.
var nodeRouteModes = []struct {
	name string
	opts gen.GenOptions
}{
	{"plain", gen.GenOptions{}},
	{"observability", gen.GenOptions{Observability: true}},
	{"limits", gen.GenOptions{LimitControls: true}},
	{"recognize", gen.GenOptions{Recognize: true}},
}

// TestOptimizationDisabledNodeRouteParity compares Node and recognizer parsers generated with an
// optimization off with the engine, in every mode of nodeRouteModes. The flag tests above generate
// lean typed Go; these are the other routes, where a pass applies in some modes and not in others.
func TestOptimizationDisabledNodeRouteParity(t *testing.T) {
	t.Parallel()
	gentest.PinOrdinary(t)
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
				gentest.GeneratedParsersCorpus(t, goBin, gentest.FlagParityCases(), o)
			})
		}
	}
}

// TestOptimizationDisabledNodeRouteNegativeControl shows that the comparison of the test above
// fails for a parser that is wrong: with the guards off and a literal of the grammar changed in the
// generated code, the parser differs from the engine on the inputs that contain the literal.
func TestOptimizationDisabledNodeRouteNegativeControl(t *testing.T) {
	t.Parallel()
	gentest.PinOrdinary(t)
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
			if got := gentest.GeneratedParsersMismatches(t, goBin, gentest.FlagParityCases(), o, mutate); len(got) == 0 {
				t.Errorf("a parser with a changed literal matches the engine")
			}
			if !changed.Load() {
				t.Errorf("the generated code has no literal \"while\" to change: the control proves nothing")
			}
		})
	}
}
