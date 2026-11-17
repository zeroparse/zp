package gogen_test

import (
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/testing/codegentest"
)

// runSpanAt returns a mutation of a generated parser that imports the library which makes the run
// sites of the program turn hot at span instead of the threshold of the library.
func runSpanAt(t *testing.T, span int) func([]byte) []byte {
	return func(code []byte) []byte {
		const lib = "rt \"zeroparse.dev/zp/codegen/go/lib\""
		if !bytes.Contains(code, []byte(lib)) {
			t.Fatalf("the generated parser does not import the library as rt")
		}
		return append(code, fmt.Sprintf("\nfunc init() { rt.RunHotSpan = %d }\n", span)...)
	}
}

// TestRunSitesGeneratedMatchEngine checks that generated parsers, whose run sites record and reuse
// their walks, give the engine's results on the run-site grammars: values, syntax errors with their
// expectations and recovered errors, from the Node, Document and typed entry points. Short inputs
// run with the sites turning hot at the first walk that walks input again, long ones at the
// threshold of the library.
func TestRunSitesGeneratedMatchEngine(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testRunSitesGeneratedMatchEngine)
}

func testRunSitesGeneratedMatchEngine(t *testing.T) {
	goBin := skipRunsGoBin(t)
	short, long := codegentest.RunSiteCases(40, false), codegentest.RunSiteCases(4, true)
	for _, m := range codegentest.GeneratedParsersMismatches(t, goBin, short, codegen.GenOptions{}, runSpanAt(t, 1)) {
		t.Error(m)
	}
	for _, m := range codegentest.GeneratedParsersMismatches(t, goBin, short, codegen.GenOptions{Document: true}, runSpanAt(t, 1)) {
		t.Error("Document: " + m)
	}
	for _, m := range codegentest.GeneratedTypesMismatches(t, goBin, short, codegen.GenOptions{}, runSpanAt(t, 1)) {
		t.Error(m)
	}
	codegentest.GeneratedParsersCorpus(t, goBin, long, codegen.GenOptions{})
	codegentest.GeneratedTypesCorpus(t, goBin, long, codegen.GenOptions{})
}

// TestRunSitesGeneratedStream checks a generated stream whose elements restart walks of run sites
// over input long enough for the sites to turn hot and the stream to drop records at its commits.
func TestRunSitesGeneratedStream(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testRunSitesGeneratedStream)
}

func testRunSitesGeneratedStream(t *testing.T) {
	goBin := skipRunsGoBin(t)
	unit := strings.Repeat("ab", 40) + "?" + strings.Repeat("b", 20) + "!" + strings.Repeat("ab", 30) + "w"
	cases := []codegentest.Case{{Name: "run sites", Src: `rule main = (r "!" / r "?" / s "w" / [ab])* #stream $$
rule r = ("ab" / "b")*
rule s = xs:("ab" / "b")+ => len($xs)`, Inputs: []string{strings.Repeat(unit, 50), strings.Repeat(unit, 50) + "c"}}}
	testGeneratedStreams(t, goBin, cases, codegen.GenOptions{})
}

// TestRunSitesGeneratedDocuments checks generated Documents, which reuse results by the input they
// examined, against batch parses along random edits of the run-site grammars whose elements look
// ahead, look at line starts, read variables or recover errors, with sites that turn hot at once.
func TestRunSitesGeneratedDocuments(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testRunSitesGeneratedDocuments)
}

func testRunSitesGeneratedDocuments(t *testing.T) {
	goBin := skipRunsGoBin(t)
	var grammars []struct{ Name, Src, Alphabet string }
	for _, set := range [][]struct{ Name, Src, Alphabet string }{codegentest.RunSiteGrammars, codegentest.RunValueGrammars} {
		for _, g := range set {
			switch g.Name {
			case "lookahead in the element", "line starts", "two environments at one position", "errors that a record replays", "lookahead and silent walks", "walks of a value-free twin":
				grammars = append(grammars, g)
			}
		}
	}
	generatedDocumentEdits(t, goBin, grammars, "xxy!xx\nab?xxxq!\nxxz?x", 1)
}

// runSiteMutations are the changes of the embedded library for the negative controls: sites that
// never turn hot, records found under any environment, hits that keep the environment of the walk,
// records of value-free walks reused by walks that build lists, and records stored when their
// iteration ends that the end of the walk leaves as they are.
var runSiteMutations = map[string][][2]string{
	"never hot": {{
		"		c.hot = true\n",
		"		c.hot = false\n",
	}},
	"ignore env": {{
		"	if p.walk().keyEnv {\n		return p.Env\n",
		"	if false {\n		return p.Env\n",
	}},
	"keep env": {{
		"	if w.keyEnv {\n		p.Env = r.env\n",
		"	if false {\n		p.Env = r.env\n",
	}},
	"any list": {{
		"r.state != skipRecorded || w.build && !r.list {",
		"r.state != skipRecorded {",
	}},
	"early store": skipRunMutations["early store"],
}

// runSiteRuns builds the parsers of the sources with the embedded library changed by mutation (none
// if "") and its sites turning hot at hotSpan (at its threshold if 0), parses the inputs of each
// source and returns the results and, when spend is set, the least budget of each parse.
func runSiteRuns(t *testing.T, goBin string, sources []string, inputs [][]string, mutation string, hotSpan int, spend bool) (spends [][]uint64, results []string) {
	t.Helper()
	dir := t.TempDir()
	write := func(name string, content []byte) {
		path := filepath.Join(dir, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", []byte(codegentest.GoMod("runlin")))
	patches := runSiteMutations[mutation]
	if mutation != "" && patches == nil {
		t.Fatalf("no mutation %s", mutation)
	}
	if hotSpan > 0 {
		patches = append(patches[:len(patches):len(patches)], [2]string{"var RunHotSpan = 64\n", "var RunHotSpan = " + strconv.Itoa(hotSpan) + "\n"})
	}
	var imports, entries strings.Builder
	for i, src := range sources {
		g, err := parseTest(src)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		pkg := fmt.Sprintf("g%d", i)
		files, err := codegentest.Generate(t, g, codegen.GenOptions{Module: "runlin/" + pkg, Package: pkg, Start: "main", Lib: codegen.LibEmbed, LimitControls: spend})
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		for _, f := range files {
			content := string(f.Content)
			if strings.HasSuffix(f.Path, "libgo/skip_runs.go") {
				for _, r := range patches {
					if !strings.Contains(content, r[0]) {
						t.Fatalf("mutation %s: %q not in the library", mutation, r[0])
					}
					content = strings.Replace(content, r[0], r[1], 1)
				}
			}
			write(pkg+"/"+f.Path, []byte(content))
		}
		fmt.Fprintf(&imports, "\t%q\n", "runlin/"+pkg)
		limits := "nil"
		if spend {
			limits = fmt.Sprintf("[]%[1]s.ParseOption{%[1]s.WithLimits(%[1]s.Limits{Compute: budget})}", pkg)
		}
		fmt.Fprintf(&entries, `	{%#v, func(s string, budget uint64) (bool, string) {
		n, err := %[2]s.Parse(s, %[3]s...)
		j, _ := json.Marshal(n)
		return errors.Is(err, %[2]s.ErrLimit), string(j) + " " + fmt.Sprint(err)
	}},
`, inputs[i], pkg, limits)
	}
	write("main.go", []byte(`package main

import (
	"encoding/json"
	"errors"
	"fmt"

`+imports.String()+`)

var entries = []struct {
	inputs []string
	parse  func(string, uint64) (bool, string)
}{
`+entries.String()+`}

const spend = `+strconv.FormatBool(spend)+`

func main() {
	for _, e := range entries {
		for _, s := range e.inputs {
			lo, hi := uint64(1), uint64(1)
			for spend {
				if limited, _ := e.parse(s, hi); !limited {
					break
				}
				lo, hi = hi+1, hi*2
			}
			for lo < hi {
				mid := (lo + hi) / 2
				if limited, _ := e.parse(s, mid); limited {
					lo = mid + 1
				} else {
					hi = mid
				}
			}
			_, r := e.parse(s, 1<<40)
			fmt.Printf("%d %q\n", lo, r)
		}
	}
}
`))
	cmd := exec.Command(goBin, "run", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod -trimpath", "GOTOOLCHAIN=local")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	for _, in := range inputs {
		var row []uint64
		for range in {
			if len(lines) == 0 {
				t.Fatalf("missing output lines:\n%s", out)
			}
			n, result, ok := strings.Cut(lines[0], " ")
			v, err := strconv.ParseUint(n, 10, 64)
			if !ok || err != nil {
				t.Fatalf("malformed line %q", lines[0])
			}
			row = append(row, v)
			results = append(results, result)
			lines = lines[1:]
		}
		spends = append(spends, row)
	}
	return spends, results
}

// TestRunSitesGeneratedLinear checks that grammars which restart walks of a run site from every
// position spend a budget linear in the input in generated parsers, that sites that never turn hot
// make them quadratic and give the same results, and records the spends of a site whose walks reach
// records in the middle of a list, which copy the rest of the list and stay quadratic.
func TestRunSitesGeneratedLinear(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testRunSitesGeneratedLinear)
}

func testRunSitesGeneratedLinear(t *testing.T) {
	goBin := skipRunsGoBin(t)
	codegentest.NeedsBudgetControls(t)
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
	spends, results := runSiteRuns(t, goBin, sources, inputs, "", 0, true)
	t.Logf("spends at %v: %v", sizes, spends)
	for i, row := range spends[:linear] {
		if _, hi := slopes(row); hi > 1.2 {
			t.Errorf("%q: spends %v grow faster than linearly", sources[i], row)
		}
	}
	cold, plain := runSiteRuns(t, goBin, sources, inputs, "never hot", 0, true)
	t.Logf("spends of sites that never turn hot: %v", cold)
	for i, row := range cold[:linear] {
		if lo, _ := slopes(row); lo < 1.7 {
			t.Errorf("%q, sites that never turn hot: spends %v are not quadratic", sources[i], row)
		}
	}
	for i := range results {
		if results[i] != plain[i] {
			t.Errorf("input %d: records change the result\n got  %s\n want %s", i, results[i], plain[i])
		}
	}
}

// TestRunSitesGeneratedNegativeControls checks that the comparisons of the records detect a key
// without the environment, a hit that leaves the environment as it was, a list made of the record
// of a value-free walk and a record stored before its walk ends, on the inputs that reach them.
// (Records stored early hold no list, so walks that build values never reuse them: the control of
// those runs on walks without values.)
func TestRunSitesGeneratedNegativeControls(t *testing.T) {
	t.Parallel()
	goBin := skipRunsGoBin(t)
	for _, tc := range []struct{ mutation, grammar string }{
		{"ignore env", "two environments at one position"},
		{"keep env", "bindings that a record restores"},
		{"any list", "walks of a value-free twin"},
		{"any list", "lookahead walks before lists"},
		{"early store", "value-free walks"},
	} {
		// The ends of the value-free walks of r show in the tree, where "!", "?" or "," matches; the
		// walk before a "," is the third from its start, which takes a record.
		valueFree := struct{ Name, Src, Alphabet string }{"value-free walks", "rule main = (r \"!\" / r \"?\" / r \",\" / [ab])* $$\nrule r = (\"ab\" / \"b\")* => 1", "ab!?,"}
		for _, g := range append(codegentest.RunValueGrammars[:len(codegentest.RunValueGrammars):len(codegentest.RunValueGrammars)], valueFree) {
			if g.Name != tc.grammar {
				continue
			}
			inputs := [][]string{codegentest.RunValueInputs[g.Name]}
			if inputs[0] == nil {
				inputs[0] = codegentest.SkipRunInputs(g.Alphabet, 40, rand.New(rand.NewSource(1)))
			}
			_, want := runSiteRuns(t, goBin, []string{g.Src}, inputs, "", 1, false)
			_, got := runSiteRuns(t, goBin, []string{g.Src}, inputs, tc.mutation, 1, false)
			if strings.Join(got, "\n") == strings.Join(want, "\n") {
				t.Errorf("mutation %s: no difference on the inputs of %s", tc.mutation, g.Name)
			}
		}
	}
}
