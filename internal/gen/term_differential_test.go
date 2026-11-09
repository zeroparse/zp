package gen_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/engine"
	"zeroparse.dev/zp/internal/engine/core"
	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
	"zeroparse.dev/zp/internal/ir/exec/ref"
	"zeroparse.dev/zp/internal/limits"
)

// termDifferentialCases is the number of terms of each seed. The tests that run in the process
// (the reference evaluator and the engine's backends) read termProcessSeeds seeds, and the tests that
// build generated parsers termGeneratedSeeds, which is larger under -tags zp_long; ZP_TERM_SEEDS
// sets both for a longer search. The default of the first kills the mutants of the arithmetic that
// the few seeds of a build did not reach (a floored remainder, a division that floors).
const termDifferentialCases = 60

const termProcessSeedsDefault = 200

func envSeeds(def uint64) uint64 {
	if n, err := strconv.ParseUint(os.Getenv("ZP_TERM_SEEDS"), 10, 64); err == nil && n > 0 {
		return n
	}
	return def
}

var (
	termProcessSeeds   = envSeeds(termProcessSeedsDefault)
	termBackendSeeds   = envSeeds(20)
	termGeneratedSeeds = envSeeds(termGeneratedSeedsDefault)
)

// wellTypedTermCases returns the generated term cases of the seed that the compiler accepts, and the
// number it rejected, which fails the test: the generator makes terms that are well typed by construction.
func wellTypedTermCases(t *testing.T, seed uint64, n int, unset bool) (cases []gentest.TermCase, rejected int) {
	t.Helper()
	return wellTypedCases(t, gentest.TermCases(seed, n, unset))
}

// wellTypedWideTermCases is wellTypedTermCases for the terms of gentest.TermCasesWide.
func wellTypedWideTermCases(t *testing.T, seed uint64, n int, unset bool) (cases []gentest.TermCase, rejected int) {
	t.Helper()
	return wellTypedCases(t, gentest.TermCasesWide(seed, n, unset))
}

func wellTypedCases(t *testing.T, all []gentest.TermCase) (cases []gentest.TermCase, rejected int) {
	t.Helper()
	for _, c := range all {
		g, err := parseTest(c.Src)
		if err == nil {
			_, err = engine.Compile(g, compile.Options{})
		}
		if err != nil {
			// The generator makes terms that are well typed by construction: a rejection is a fault of it,
			// in every test that reads its terms.
			t.Errorf("rejected: %v\n%s", err, c.Src)
			rejected++
			continue
		}
		cases = append(cases, c)
	}
	return cases, rejected
}

// termSeedCases returns the well-typed terms of seed: the narrow terms for the seeds up to half, the
// wide terms of gentest.TermCasesWide for the others, so that a test with 2*half seeds reads both.
func termSeedCases(t *testing.T, seed, half uint64, unset bool) (cases []gentest.TermCase, rejected int) {
	t.Helper()
	if seed > half {
		return wellTypedWideTermCases(t, seed-half, termDifferentialCases, unset)
	}
	return wellTypedTermCases(t, seed, termDifferentialCases, unset)
}

func termCaseList(cs []gentest.TermCase) []gentest.Case {
	out := make([]gentest.Case, len(cs))
	for i, c := range cs {
		out[i] = c.Case
	}
	return out
}

// TestTermEngineBackends checks that the engine's closure backend, its bytecode VM and its iterative
// VM give one value or one error for every generated term, in both position units. Their charges differ
// by design today (zp.Limits: counts depend on backend); the test only requires that every backend
// reports one and logs how far apart the backends are.
func TestTermEngineBackends(t *testing.T) {
	t.Parallel()
	for seed := uint64(1); seed <= 2*termBackendSeeds; seed++ {
		cases, _ := termSeedCases(t, seed, termBackendSeeds, true)
		for _, c := range cases {
			g, _ := parseTest(c.Src)
			prog, err := engine.Compile(g, compile.Options{})
			if err != nil {
				t.Fatal(err)
			}
			for _, in := range c.Inputs {
				for _, unit := range []engine.PositionUnit{engine.CodePoints, engine.Bytes} {
					var want string
					for _, b := range []engine.Backend{engine.Closure, engine.Bytecode, engine.BytecodeIterative} {
						var used uint64
						n, err := prog.ParseWith("main", in, engine.ParseOptions{Backend: b, Unit: unit, Limits: limits.Limits{Compute: 1 << 50},
							Trace: func(e engine.TraceEvent) { used = core.TraceParser(e).Control.Used }})
						got := gentest.ResultJSON(n, err)
						if used == 0 {
							t.Errorf("%s: backend %d reports no charges", c.Name, b)
						}
						if b == engine.Closure {
							want = got
						} else if got != want {
							t.Errorf("%s: input %q unit %d: backend %d gives\n %s\nclosure gives\n %s", c.Name, in, unit, b, got, want)
						}
					}
				}
			}
		}
	}
}

// TestTermGeneratedTargets checks that generated Go (node and typed) and generated TypeScript give the
// engine's value or error for every generated term, in both position units.
func TestTermGeneratedTargets(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testTermGeneratedTargets)
}

func testTermGeneratedTargets(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	list := func(unset bool) []gentest.Case {
		var all []gentest.TermCase
		for seed := uint64(1); seed <= 2*termGeneratedSeeds; seed++ {
			cases, _ := termSeedCases(t, seed, termGeneratedSeeds, unset)
			all = append(all, cases...)
		}
		cases := termCaseList(all)
		for i := range cases {
			cases[i].Name = fmt.Sprintf("%d %s", i, cases[i].Name)
		}
		return cases
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	cases := list(true)
	gentest.GeneratedParsersCorpus(t, goBin, cases, gen.GenOptions{})
	if node, err := exec.LookPath("node"); err == nil {
		gentest.TSParsersCorpus(t, node, cases, gen.GenOptions{})
	}
	// A typed parser reads an unset field as the zero value of its Type, not as nil, so the typed
	// comparison leaves unset fields out.
	gentest.GeneratedTypesCorpus(t, goBin, list(false), gen.GenOptions{ConvertTypes: true}, 0)
}

// TestTermDifferentialDetectsBrokenTargets is the negative control of TestTermGeneratedTargets: with an
// operator of the generated Go or TypeScript parsers swapped for another, or the iteration order of
// foldl reversed, the comparison with the engine fails on some generated term.
func TestTermDifferentialDetectsBrokenTargets(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testTermDifferentialDetectsBrokenTargets)
}

func testTermDifferentialDetectsBrokenTargets(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	cases, _ := wellTypedTermCases(t, 1, termDifferentialCases, true)
	list := termCaseList(cases)
	for i := range list {
		list[i].Name = fmt.Sprintf("%d %s", i, list[i].Name)
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	for _, m := range []struct{ name, from, to string }{
		{"plus becomes minus", `c.Binary(rt.OpAdd`, `c.Binary(rt.OpSub`},
		{"times becomes plus", `c.Binary(rt.OpMul`, `c.Binary(rt.OpAdd`},
		{"map becomes filter", `c.MapList(`, `c.FilterList(`},
	} {
		got := gentest.GeneratedParsersMismatches(t, goBin, list, gen.GenOptions{}, func(code []byte) []byte {
			return []byte(strings.ReplaceAll(string(code), m.from, m.to))
		})
		if len(got) == 0 {
			t.Errorf("Go: %s: no generated term notices", m.name)
		}
	}
	node, err := exec.LookPath("node")
	if err != nil {
		return
	}
	for _, m := range []struct{ name, from, to string }{
		{"plus becomes minus", `c.binary(0,`, `c.binary(1,`},
		{"equal becomes not equal", `c.binary(5,`, `c.binary(6,`},
		{"length becomes zero", `c.length(`, `((x) => (c.length(x), 0))(`},
	} {
		got := gentest.TSParsersMismatches(t, node, list, gen.GenOptions{}, func(code string) string {
			return strings.ReplaceAll(code, m.from, m.to)
		})
		if len(got) == 0 {
			t.Errorf("TypeScript: %s: no generated term notices", m.name)
		}
	}
}

// referenceTerm returns the term of the generated case: the action of the rule main, or the predicate
// that precedes the optional "b".
func referenceTerm(t *testing.T, c gentest.TermCase) grammar.Term {
	t.Helper()
	g, err := parseTest(c.Src)
	if err != nil {
		t.Fatal(err)
	}
	var found grammar.Term
	var walk func(e grammar.Expr)
	walk = func(e grammar.Expr) {
		switch e := e.(type) {
		case *grammar.Predicate:
			found = e.Term
		case *grammar.Seq:
			for _, x := range e.Items {
				walk(x)
			}
		case *grammar.Choice:
			for _, x := range e.Alts {
				walk(x)
			}
		case *grammar.Group:
			walk(e.Expr)
		}
	}
	for _, d := range g.File.Decls {
		if r, ok := d.(*grammar.RuleDef); ok && r.Name == "main" {
			if c.Kind == gentest.TermAction {
				return r.Action
			}
			walk(r.Expr)
		}
	}
	if found == nil {
		t.Fatalf("%s: no term", c.Name)
	}
	return found
}

// TestTermReferenceEvaluator checks the engine's closure backend against the reference evaluator of
// terms (internal/ir/exec/ref), which states the semantics of a term on naive tagged values: the same
// value or the same error for every generated term, in both position units. A predicate case compares
// whether the predicate held, by whether the input "ab" parses.
func TestTermReferenceEvaluator(t *testing.T) {
	t.Parallel()
	checked := 0
	for seed := uint64(1); seed <= 2*termProcessSeeds; seed++ {
		cases, _ := termSeedCases(t, seed, termProcessSeeds, true)
		for _, c := range cases {
			g, _ := parseTest(c.Src)
			prog, err := engine.Compile(g, compile.Options{})
			if err != nil {
				t.Fatal(err)
			}
			term := referenceTerm(t, c)
			for _, unit := range []engine.PositionUnit{engine.CodePoints, engine.Bytes} {
				w := utf8.RuneCountInString(c.Lit)
				if unit == engine.Bytes {
					w = len(c.Lit)
				}
				env := &ref.TermEnv{Captures: map[string]ref.V{"x": ref.Node(c.Lit, 0, w)}, Bytes: unit == engine.Bytes, Start: 0, End: w, Line: 1, Col: 1,
					Zeros: referenceZeros(t, prog.Static(), "R", "Inner")}
				v, msg := env.Eval(term)
				opts := engine.ParseOptions{Backend: engine.Closure, Unit: unit}
				if c.Kind == gentest.TermPredicate {
					n, err := prog.ParseWith("main", c.Lit+"b", opts)
					held := err == nil && n != nil
					if want := ref.Holds(v, msg); held != want {
						t.Errorf("%s: unit %d: the engine holds the predicate: %v, the reference evaluator: %v (%s)", c.Name, unit, held, want, msg)
					}
					checked++
					continue
				}
				n, err := prog.ParseWith("main", c.Lit, opts)
				want := map[string]any{"node": nil}
				if msg != "" {
					want["err"] = "action in main: " + msg
				} else {
					want["node"] = env.Plain(v)
				}
				wantJSON, _ := json.Marshal(want)
				got := gentest.ResultJSON(n, err)
				var gotAny, wantAny any
				if json.Unmarshal([]byte(got), &gotAny) != nil || json.Unmarshal(wantJSON, &wantAny) != nil || !reflect.DeepEqual(gotAny, wantAny) {
					t.Errorf("%s: unit %d: the engine gives\n %s\nthe reference evaluator\n %s", c.Name, unit, got, wantJSON)
				}
				checked++
			}
		}
	}
	if checked == 0 {
		t.Fatal("no case was checked")
	}
}

// referenceZeros returns the zero values of the fields of the struct types names that the grammar declares for the reference
// evaluator, decoded from the zero descriptors that the compiled program holds
// (compile.Program.StructZeroOf). A struct field holds the struct with no field set.
func referenceZeros(t testing.TB, prog *compile.Program, names ...string) map[string]map[string]ref.V {
	t.Helper()
	var decode func(d string) (ref.V, string)
	decode = func(d string) (ref.V, string) {
		switch d[0] {
		case 'i':
			return ref.Int(0), d[1:]
		case 's':
			return ref.Str(""), d[1:]
		case 'b':
			return ref.Bool(false), d[1:]
		case 'n':
			return ref.Nil, d[1:]
		case 'l':
			return ref.List(), d[1:]
		case '(':
			var items []ref.V
			d = d[1:]
			for d[0] != ')' {
				var e ref.V
				e, d = decode(d)
				items = append(items, e)
			}
			return ref.Tuple(items...), d[1:]
		case '{':
			name, rest, _ := strings.Cut(d[1:], "}")
			return ref.V{Kind: ref.KStruct, Type: name, Fields: map[string]ref.V{}}, rest
		}
		t.Fatalf("unknown zero descriptor %q", d)
		return ref.Nil, ""
	}
	zeros := map[string]map[string]ref.V{}
	for _, name := range names {
		sz := prog.StructZeroOf(name)
		if sz == nil {
			continue // the type is declared by the grammars of some generators only
		}
		zeros[name] = map[string]ref.V{}
		for _, f := range sz.Names {
			if sz.Zeros[f] != compile.NoZeroDesc {
				zeros[name][f], _ = decode(sz.Zeros[f])
			}
		}
	}
	return zeros
}
