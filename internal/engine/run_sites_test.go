package engine

import (
	"errors"
	"math/rand"
	"strconv"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/engine/core"
	"zeroparse.dev/zp/internal/gentest"
	"zeroparse.dev/zp/internal/limits"
)

// runSiteGrammars are grammars with run sites (compile/run_sites.go): repetitions outside skip
// expressions whose parse builds no value (the start rule has an action, so the elements' values
// are not needed), each with the characters of its random inputs. They exercise the parts of what a
// record holds, as the skip-run grammars do, and nested walks.
var runSiteGrammars = []struct{ Name, Src, Alphabet string }{
	{"repetition restarted by an alternative", `rule main = (a "y" / "q" a / [x])* $$ => 1
rule a = x+ => 1
rule x = "x"`, "xyqz"},
	{"recursion over a scan", `rule main = t a? $$ => 1
rule t = a t "!" / a "?" t / [x!?] t / ""
rule a = x+ => 1
rule x = "x"`, "x!z"},
	{"nested walks", `rule main = (g "y" / g "z" / [abc,])* $$ => 1
rule g = (w ","?)+ => 1
rule w = [a-b]+ "c"? => 1`, "abc,yzq"},
	{"minimum and empty elements", `rule main = (a "!" / [xy])* $$ => 1
rule a = (x? y?)#repeat(2,) => 1
rule x = "x"
rule y = "y"`, "xy!z"},
	{"lookahead in the element", `rule main = (a "!" / [xy])* $$ => 1
rule a = (!"xx" x)+ => 1
rule x = "x" / "y"`, "xy!z"},
	{"expectations", `rule main = (a "!" / "q" a "?" / [xyz])* $$ => 1
rule a = (x / "y" z)+ => 1
rule x = "x"
rule z = "z"`, "xyz!?qw"},
	{"silent walks", `rule main = (!(a "!") "x" / a "!" / [,y])* $$ => 1
rule a = (x ","?)+ => 1
rule x = "x" "y"?`, "xy,!z"},
	{"line starts", `rule main = (l "!" / [ab\n])* $$ => 1
rule l = (^ "a" / x)+ => 1
rule x = "b" / "\n"`, "ab\n!z"},
}

// runValueGrammars are grammars whose run sites build values, capture and read variables in their
// elements, and recover errors in them or in the rules they call; they have no action at the top,
// so the values of the start rule are the parse trees.
var runValueGrammars = []struct{ Name, Src, Alphabet string }{
	{"values of restarted lists", `rule main = (a "y" / "q" a / [x])* $$
rule a = x+
rule x = "x"`, "xyqz"},
	{"nested lists", `rule main = (g "y" / g "z" / [abc,])* $$
rule g = (w ","?)+
rule w = [a-b]+ "c"?`, "abc,yzq"},
	{"minimum and empty elements", `rule main = (a "!" / [xy])* $$
rule a = (x? y?)#repeat(2,)
rule x = "x"
rule y = "y"`, "xy!z"},
	{"captures in the element", `rule main = (a "!" / "q" a "?" / [xyz])* $$
rule a = (c:x d:y? => $c)+
rule x = "x"
rule y = "y"`, "xyz!?qw"},
	{"actions in the element", `rule main = (a "!" / [xy])* $$
rule a = (v:x => $v)+
rule x = "x" "y"?`, "xy!z"},
	{"variables in predicates", `var n int
rule main = (a "!" / "q" a / [xz])* $$
rule a = ({ n < 3 } x { n := n + 1 })+
rule x = "x"`, "xz!q"},
	{"two environments at one position", `var n int
rule main = ({ n := 1 } r "!" / { n := 5 } r "?" / [xyzwq])* $$
rule r = a "y" / a "w" / a "z" / a "q"
rule a = "x" ("x" { n < 3 })*`, "xyzwq!?"},
	{"walks that merge with others in the middle", `rule main = (r "!" / r "?" / r "w" / [abz])* $$
rule r = ("ab" / "a" / "b")*`, "abz!?w"},
	{"bindings that a record restores", `var n int
rule main = (r "!" / r "?" / r "w" / [xyzwq])* $$
rule r = ("x" { n := n + 1 })* { n == 3 }`, "xyzwq!?"},
	{"errors that a record replays", `rule main = (a "?" / "x" a "?" / "x" "x" a "!" / [xz!?])* $$
rule a = e+
rule e = ("x" "z") #recover(skip=[x])`, "xz!?"},
	{"recovery in the element", `rule main = (a "!" / "q" a "?" / [xyz])* $$
rule a = (x #recover(skip=[y]+))+
rule x = "x" "z"`, "xzy!?q"},
	{"recovery in a called rule", `rule main = (l "!" / [abz])* $$
rule l = (e)+
rule e = ("a" "b" #recover(skip=[z]+))`, "abz!"},
	{"lookahead and silent walks", `rule main = (!(a "!") "x" / a "!" / [,y])* $$
rule a = (x ","?)+
rule x = "x" "y"?`, "xy,!z"},
	{"a site free of the environment under rebindings", `var n int
rule main = ({ n := 1 } r "!" / { n := 2 } r "?" / r "#" / [xyzw!?#])* $$
rule r = ("x" / "yz" / b)*
rule b = "w" { n := 3 }`, "xyzw!?#"},
	{"a called rule that reads a variable", `var n int
rule main = ({ n := 1 } r "!" / { n := 5 } r "?" / [xyz!?])* $$
rule r = e*
rule e = "x" { n < 3 } / "y"`, "xyz!?"},
}

// runValueInputs are inputs, besides random ones, that reach what a grammar of runValueGrammars is
// about: here a walk that a record of another environment would answer wrongly.
var runValueInputs = map[string][]string{
	"two environments at one position":    {"xxxq!", "xxxq?", "xxxq?xxxq!", "xxxxxq?!"},
	"bindings that a record restores":     {"xxxw", "xxw", "xxxxw", "xxxw!xxw"},
	"errors that a record replays":        {"xxxxxx!", "xxzxxx!", "xxxxzxx!xxxxx!", "xxxxxxx?"},
	"a called rule that reads a variable": {"xxxx?", "xxxx!", "xyxx?", "xxxx?xxx!"},
}

// hotSpans are the thresholds at which the differential tests of run sites run: the one of the
// engine, at which the short inputs of the tests stay cold, and short ones, at which sites turn hot
// in the middle of a parse and the records answer the walks after it.
var hotSpans = []int{core.RunHotSpan, 2, 1}

// atHotSpans runs f at each of hotSpans, and checks that the sites turn hot at the short ones.
func atHotSpans(t *testing.T, f func(t *testing.T)) {
	t.Helper()
	defer func(n, m int) { core.RunHotSpan, core.ScanHotSpan = n, m }(core.RunHotSpan, core.ScanHotSpan)
	for _, span := range hotSpans {
		core.RunHotSpan, core.ScanHotSpan = span, span
		turns := core.RunHotTurns.Load()
		t.Run("span"+strconv.Itoa(span), f)
		if span <= 2 && core.RunHotTurns.Load() == turns {
			t.Errorf("no run site turned hot at span %d", span)
		}
	}
}

// assertTurnedHot checks that a run site turned hot since turns, when the threshold is short.
func assertTurnedHot(t *testing.T, name string, turns int64) {
	t.Helper()
	if core.RunHotSpan <= 2 && core.RunHotTurns.Load() == turns {
		t.Errorf("%s: no run site turned hot", name)
	}
}

// TestRunSitesMatchPlainRepetitions checks that the repetitions of run sites that build values
// give the same trees and errors as plain repetitions (a program compiled without run sites) and
// as walks without records, on every backend.
func TestRunSitesMatchPlainRepetitions(t *testing.T) { atHotSpans(t, runSitesMatchPlainRepetitions) }

func runSitesMatchPlainRepetitions(t *testing.T) {
	rng := rand.New(rand.NewSource(113))
	for _, g := range runValueGrammars {
		prog := mustCompile(t, g.Src)
		if prog.NumRunSites == 0 {
			t.Errorf("%s: no run sites", g.Name)
		}
		compile.NoRunSites = true
		plain := mustCompile(t, g.Src)
		compile.NoRunSites = false
		inputs := append(gentest.SkipRunInputs(g.Alphabet, 300, rng), runValueInputs[g.Name]...)
		turns := core.RunHotTurns.Load()
		for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
			if diffs := skipRunMismatches(t, prog, inputs, b); len(diffs) > 0 {
				t.Errorf("%s, %s: %d parses differ without records, first:\n%s", g.Name, b, len(diffs), diffs[0])
			}
			for _, in := range inputs {
				for _, exp := range []bool{true, false} {
					o := ParseOptions{Backend: b, Expectations: exp}
					n, err := prog.ParseWith("main", in, o)
					wn, werr := plain.ParseWith("main", in, o)
					if got, want := skipDump(t, n, err), skipDump(t, wn, werr); got != want {
						t.Fatalf("%s, %s: input %q\n got  %s\n want %s", g.Name, b, in, got, want)
					}
				}
			}
		}
		assertTurnedHot(t, g.Name, turns)
	}
}

// TestRunSitesOfLoadedPrograms checks that a program loaded from a compiled grammar keys the
// records of its run sites as the compiled program does: parses and Document parses of the grammars
// of runValueGrammars, many of whose sites are keyed by the environment, give the results of plain
// repetitions on the bytecode backends, which run loaded programs.
func TestRunSitesOfLoadedPrograms(t *testing.T) { atHotSpans(t, runSitesOfLoadedPrograms) }

func runSitesOfLoadedPrograms(t *testing.T) {
	rng := rand.New(rand.NewSource(115))
	for _, g := range runValueGrammars {
		compile.NoRunSites = true
		plain := mustCompile(t, g.Src)
		compile.NoRunSites = false
		inputs := append(gentest.SkipRunInputs(g.Alphabet, 300, rng), runValueInputs[g.Name]...)
		turns := core.RunHotTurns.Load()
		for _, mode := range []CompiledMode{CompiledParse, CompiledDocument} {
			data, err := mustCompile(t, g.Src).Marshal("", mode)
			if err != nil {
				t.Fatalf("%s: %v", g.Name, err)
			}
			loaded, _, err := LoadProgram(data)
			if err != nil {
				t.Fatalf("%s: %v", g.Name, err)
			}
			for _, b := range []Backend{Bytecode, BytecodeIterative} {
				for _, in := range inputs {
					o := ParseOptions{Backend: b, Expectations: true}
					wn, werr := plain.ParseWith("main", in, o)
					want := skipDump(t, wn, werr)
					if mode == CompiledDocument {
						d, err := loaded.NewDocumentWith("main", in, o)
						if err != nil {
							t.Fatalf("%s: %v", g.Name, err)
						}
						n, err := d.Parse()
						if got := skipDump(t, n, err); got != want {
							t.Fatalf("%s, %s, document of a loaded program: input %q\n got  %s\n want %s", g.Name, b, in, got, want)
						}
						continue
					}
					n, err := loaded.ParseWith("main", in, o)
					if got := skipDump(t, n, err); got != want {
						t.Fatalf("%s, %s, loaded program: input %q\n got  %s\n want %s", g.Name, b, in, got, want)
					}
				}
			}
		}
		assertTurnedHot(t, g.Name, turns)
	}
}

// TestRunSitesMatchParsesWithoutRecords checks that the records of run sites change no result:
// values and syntax errors with their expectations, on every backend. It toggles package state, so
// it does not run in parallel.
func TestRunSitesMatchParsesWithoutRecords(t *testing.T) { atHotSpans(t, runSitesMatchWithoutRecords) }

func runSitesMatchWithoutRecords(t *testing.T) {
	rng := rand.New(rand.NewSource(112))
	for _, g := range runSiteGrammars {
		prog := mustCompile(t, g.Src)
		inputs := gentest.SkipRunInputs(g.Alphabet, 300, rng)
		turns := core.RunHotTurns.Load()
		for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
			if diffs := skipRunMismatches(t, prog, inputs, b); len(diffs) > 0 {
				t.Errorf("%s, %s: %d parses differ, first:\n%s", g.Name, b, len(diffs), diffs[0])
			}
		}
		assertTurnedHot(t, g.Name, turns)
	}
	for _, c := range gentest.GenCorpus(t) {
		g, err := parseTest(c.Src)
		if err != nil {
			t.Fatalf("corpus %s: %v", c.Name, err)
		}
		prog, err := Compile(g, compile.Options{SourceInfo: true})
		if err != nil {
			t.Fatalf("corpus %s: %v", c.Name, err)
		}
		for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
			if diffs := skipRunMismatches(t, prog, c.Inputs, b); len(diffs) > 0 {
				t.Errorf("corpus %s, %s: %d parses differ, first:\n%s", c.Name, b, len(diffs), diffs[0])
			}
		}
	}
}

// TestRunSitesAreMarked checks that the grammars of TestRunSitesMatchParsesWithoutRecords have run
// sites, so that the comparison does not pass for want of any.
func TestRunSitesAreMarked(t *testing.T) {
	for _, g := range runSiteGrammars {
		prog := mustCompile(t, g.Src)
		if n := prog.NumRunSites; n == 0 {
			t.Errorf("%s: no run sites", g.Name)
		}
	}
}

// TestRunSitesAreRestartable checks which repetitions are marked: a repetition that no iteration
// reaches is reached once by a derivation and is not a site, and one in the element of another
// repetition, in a rule that element calls, or in a rule that calls itself is.
func TestRunSitesAreRestartable(t *testing.T) {
	for _, c := range []struct {
		name, src string
		sites     int
	}{
		{"a list parsed once", "rule main = item* $$\nrule item = \"a\"", 0},
		{"a list in a called rule", "rule main = list $$\nrule list = \"(\" item* \")\"\nrule item = \"a\"", 0},
		{"a list in a bounded repetition", "rule main = (\"(\" item* \")\") #repeat(0, 1) $$\nrule item = \"a\"", 0},
		{"a list in the element of a repetition", "rule main = (\"(\" item* \")\")* $$\nrule item = \"a\"", 1},
		{"a repetition inside a repetition", "rule main = (item+)* $$\nrule item = \"a\" \"b\"", 1},
		{"a list in a rule that an iteration calls", "rule main = group* $$\nrule group = \"(\" item* \")\"\nrule item = \"a\"", 1},
		{"a list in a right recursion", "rule main = t $$\nrule t = \"(\" item* \")\" t / \"\"\nrule item = \"a\"", 1},
	} {
		if n := mustCompile(t, c.src).NumRunSites; n != c.sites {
			t.Errorf("%s: %d run sites, want %d", c.name, n, c.sites)
		}
	}
}

// TestRunSitesReuseRecords checks that the records of run sites are reused on every backend: a
// parse that restarts a scan at each position of the input fits a budget that is linear in the
// input with records, and exceeds it without them.
func TestRunSitesReuseRecords(t *testing.T) {
	if !core.BudgetControlsEnabled {
		t.Skip("budget controls are unavailable in the reference build")
	}
	defer func() { core.SkipRecordsOff = false }()
	in := strings.Repeat("x", 400) + "z"
	for _, g := range runSiteGrammars[:2] {
		prog := mustCompile(t, g.Src)
		for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
			o := ParseOptions{Backend: b, Limits: limits.Limits{Compute: 200_000}}
			core.SkipRecordsOff = false
			if _, err := prog.ParseWith("main", in, o); errors.Is(err, limits.ErrLimit) {
				t.Errorf("%s, %s: the parse with records exceeds the budget", g.Name, b)
			}
			core.SkipRecordsOff = true
			if _, err := prog.ParseWith("main", in, o); !errors.Is(err, limits.ErrLimit) {
				t.Errorf("%s, %s: the parse without records fits the budget", g.Name, b)
			}
		}
	}
}

// TestRunSitesBoundValueControls checks that the controls whose repetitions build values (no
// action in the grammar), among them one with two callers of a repetition and one whose walks start
// in a new environment at each position, fit a budget that is linear in the input on every backend,
// and exceed it with plain repetitions.
func TestRunSitesBoundValueControls(t *testing.T) {
	if !core.BudgetControlsEnabled {
		t.Skip("budget controls are unavailable in the reference build")
	}
	cases := []struct{ Name, Src, Input string }{
		{"E1", "rule main = (a / \"a\")* $$\nrule a = \"a\"* \"c\"\n", strings.Repeat("a", 1000)},
		{"E7", "rule main = (a / \"a\")* $$\nrule a = (b / \"a\")* \"c\"\nrule b = \"a\"* \"d\"\n", strings.Repeat("a", 100)},
		{"rebindings around a site free of the environment", "var m int\nrule main = (a / \"x\")* [xyz]* $$\nrule a = { m := 0 } w \"!\" / { m := 1 } w \"?\" / { m := 2 } w \"#\"\nrule w = (\"x\" / \"yy\")*\n", strings.Repeat("x", 500) + strings.Repeat("yy", 500)},
		{"two callers", "@start\nrule main = (\"(\" b \")!\" / \"(\" b \")?\" / \"(\")+ \"(\"*\nrule b = (\"(\" c \")!\" / \"(\")* $$\nrule c = \"(\" c / \"x\"\n", strings.Repeat("(", 1000)},
	}
	for _, c := range cases {
		prog := mustCompile(t, c.Src)
		compile.NoRunSites = true
		plain := mustCompile(t, c.Src)
		compile.NoRunSites = false
		// A loaded program keeps which sites are free of the environment.
		data, err := prog.Marshal("", CompiledParse)
		if err != nil {
			t.Fatal(err)
		}
		loaded, _, err := LoadProgram(data)
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
			o := ParseOptions{Backend: b, Limits: limits.Limits{Compute: 150 * uint64(len(c.Input))}}
			if _, err := prog.ParseWith("main", c.Input, o); errors.Is(err, limits.ErrLimit) {
				t.Errorf("%s, %s: the parse with records exceeds the budget", c.Name, b)
			}
			if _, err := loaded.ParseWith("main", c.Input, o); b != Closure && errors.Is(err, limits.ErrLimit) {
				t.Errorf("%s, %s: the parse of the loaded program exceeds the budget", c.Name, b)
			}
			// (The closure backend does not charge the iterations of plain loops over literals, so
			// only the bytecode backends show that the control is quadratic without records; the
			// controls of internal/complexitytest measure it on all three.)
			if _, err := plain.ParseWith("main", c.Input, o); b != Closure && !errors.Is(err, limits.ErrLimit) {
				t.Errorf("%s, %s: the parse with plain repetitions fits the budget", c.Name, b)
			}
		}
	}
}

// TestRunSitesNegativeControls checks that the comparisons of the hot path detect a key without the
// environment and a hit that leaves the environment as it was, on the inputs that reach them.
func TestRunSitesNegativeControls(t *testing.T) {
	defer func(n int) { core.RunHotSpan = n; core.SkipRecordMutation = 0 }(core.RunHotSpan)
	core.RunHotSpan = 1
	for _, tc := range []struct {
		mutation int
		grammar  string
	}{
		{core.SkipIgnoreEnv, "two environments at one position"},
		{core.SkipKeepEnv, "bindings that a record restores"},
	} {
		for _, g := range runValueGrammars {
			if g.Name != tc.grammar {
				continue
			}
			prog := mustCompile(t, g.Src)
			core.SkipRecordMutation = tc.mutation
			for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
				if diffs := skipRunMismatches(t, prog, runValueInputs[g.Name], b); len(diffs) == 0 {
					t.Errorf("mutation %d, %s: no difference on the inputs of %s", tc.mutation, b, g.Name)
				}
			}
			core.SkipRecordMutation = 0
		}
	}
	// The sites that the compiler keys by the environment must be keyed by it: a program compiled
	// with every site free of it, or with the variables of called rules left out, gives other
	// results than a parse without records.
	defer func() { compile.EnvFreeMutation = 0 }()
	for _, tc := range []struct {
		mutation int
		grammar  string
	}{
		{compile.EnvFreeAll, "two environments at one position"},
		{compile.EnvFreeAll, "bindings that a record restores"},
		{compile.EnvFreeCalleesIgnore, "a called rule that reads a variable"},
	} {
		for _, g := range runValueGrammars {
			if g.Name != tc.grammar {
				continue
			}
			// (The mutation stays on while the backends build their programs.)
			compile.EnvFreeMutation = tc.mutation
			prog := mustCompile(t, g.Src)
			inputs := append(gentest.SkipRunInputs(g.Alphabet, 300, rand.New(rand.NewSource(114))), runValueInputs[g.Name]...)
			for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
				if diffs := skipRunMismatches(t, prog, inputs, b); len(diffs) == 0 {
					t.Errorf("compile mutation %d, %s: no difference on the inputs of %s", tc.mutation, b, g.Name)
				}
			}
			compile.EnvFreeMutation = 0
		}
	}
}

// TestRunSitesKeyedByEnvironment checks which sites the compiler keys by the environment: those
// whose element reads or binds a variable, or calls a rule that reads one, directly or indirectly.
// A binding in a called rule is undone when it returns and keys nothing, and a skip-run site never
// reads or binds one.
func TestRunSitesKeyedByEnvironment(t *testing.T) {
	for _, c := range []struct {
		name, src string
		keyed     bool
	}{
		{"no variables", "rule main = (r \"!\" / [xy])* $$\nrule r = (\"x\" / \"yy\")*", false},
		{"a rebinding outside the site", "var m int\nrule main = ({ m := 1 } r \"!\" / [xy])* $$\nrule r = (\"x\" / \"yy\")*", false},
		{"a predicate that reads in the element", "var m int\nrule main = (r \"!\" / [xy])* $$\nrule r = (\"x\" { m == 0 } / \"yy\")*", true},
		{"a binding in the element", "var m int\nrule main = (r \"!\" / [xy])* $$\nrule r = (\"x\" { m := 1 } / \"yy\")*", true},
		{"a called rule that reads", "var m int\nrule main = (r \"!\" / [xy])* $$\nrule r = e*\nrule e = \"x\" { m == 0 } / \"yy\"", true},
		{"a rule called indirectly that reads", "var m int\nrule main = (r \"!\" / [xy])* $$\nrule r = e*\nrule e = f / \"yy\"\nrule f = \"x\" { m == 0 }", true},
		{"a called rule that binds", "var m int\nrule main = (r \"!\" / [xy])* $$\nrule r = e*\nrule e = \"x\" { m := 1 } / \"yy\"", false},
	} {
		prog := mustCompile(t, c.src)
		if prog.NumRunSites != 1 {
			t.Fatalf("%s: %d run sites, want 1", c.name, prog.NumRunSites)
		}
		site := prog.NumSkipSites + 1
		if got := !prog.EnvFree(site); got != c.keyed {
			t.Errorf("%s: keyed by the environment %v, want %v", c.name, got, c.keyed)
		}
	}
	prog := mustCompile(t, "var m int\nrule main = (({ m := 1 } \"a\" \"b\") #recover(skip=[^;]* \";\"))* $$")
	for site := 1; site <= prog.NumSkipSites; site++ {
		if !prog.EnvFree(site) {
			t.Errorf("skip-run site %d is keyed by the environment", site)
		}
	}
	if prog.NumSkipSites == 0 {
		t.Errorf("no skip-run site")
	}
}

// TestRunSitesTurnHotAtTheEngineThreshold checks, at the threshold of the engine, that a site that
// starts cold and turns hot in the middle of a long parse gives the results of a parse without run
// sites, with and without values, on every backend.
func TestRunSitesTurnHotAtTheEngineThreshold(t *testing.T) {
	if core.RunHotSpan != 64 {
		t.Fatalf("threshold %d", core.RunHotSpan)
	}
	in := strings.Repeat("x", 300) + "z" + strings.Repeat("x", 200) + "!"
	for _, g := range append(runSiteGrammars[:2:2], runValueGrammars[0]) {
		prog := mustCompile(t, g.Src)
		compile.NoRunSites = true
		plain := mustCompile(t, g.Src)
		compile.NoRunSites = false
		for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
			for _, exp := range []bool{true, false} {
				o := ParseOptions{Backend: b, Expectations: exp}
				turns := core.RunHotTurns.Load()
				n, err := prog.ParseWith("main", in, o)
				wn, werr := plain.ParseWith("main", in, o)
				if got, want := skipDump(t, n, err), skipDump(t, wn, werr); got != want {
					t.Fatalf("%s, %s: results differ\n got  %s\n want %s", g.Name, b, got, want)
				}
				if core.RunHotTurns.Load() == turns {
					t.Errorf("%s, %s: no run site turned hot", g.Name, b)
				}
			}
		}
	}
}

// TestRunSitesDepthLimit checks that a parse with a depth limit fails or succeeds with and without
// run records on a hot site, on every backend, and gives the same result.
func TestRunSitesDepthLimit(t *testing.T) {
	src := "@start\nrule main = (\"(\" b \")!\" / \"(\")+ \"(\"*\nrule b = (\"(\" c \")!\" / \"(\")* $$\nrule c = \"(\" c / \"x\"\n"
	in := strings.Repeat("(", 300)
	prog := mustCompile(t, src)
	compile.NoRunSites = true
	plain := mustCompile(t, src)
	compile.NoRunSites = false
	turns := core.RunHotTurns.Load()
	for _, depth := range []int{2, 5, 50, 299, 300, 301, 1000} {
		for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
			o := ParseOptions{Backend: b, Limits: limits.Limits{Depth: uint64(depth)}}
			n, err := prog.ParseWith("main", in, o)
			wn, werr := plain.ParseWith("main", in, o)
			if got, want := skipDump(t, n, err), skipDump(t, wn, werr); got != want {
				t.Errorf("depth %d, %s:\n got  %s\n want %s", depth, b, got, want)
			}
			if limited := errors.Is(err, limits.ErrLimit); limited != (depth <= 50) {
				t.Errorf("depth %d, %s: limit error %v, err %v", depth, b, limited, err)
			}
		}
	}
	if core.RunHotTurns.Load() == turns {
		t.Error("no run site turned hot")
	}
}
