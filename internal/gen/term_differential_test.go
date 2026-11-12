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
		// The fixed terms read every kind of member, whether or not the constructor set it.
		cases := append(termCaseList(all), termCaseList(gentest.MemberTermCases())...)
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
	// A typed parser reads a field that a constructor leaves out as the zero value of its Type, like the
	// others, so the typed comparison reads unset fields too.
	gentest.GeneratedTypesCorpus(t, goBin, list(true), gen.GenOptions{ConvertTypes: true}, 0)
}

// TestTermFixedGeneratedTargets checks the fixed term sets of gentest (conversions, structs, casts,
// list and tuple values, iteration, pushes and scalar operators) on generated Go (node and typed) and generated
// TypeScript: each gives the engine's value or error for every input, in both position units. The
// generated terms of TestTermGeneratedTargets do not reach every form these sets pin, and the charge
// goldens of the sets compare only the outcome, not the value.
func TestTermFixedGeneratedTargets(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testTermFixedGeneratedTargets)
}

func testTermFixedGeneratedTargets(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	all, _ := wellTypedCases(t, fixedTermCases())
	cases := termCaseList(all)
	for i := range cases {
		cases[i].Name = fmt.Sprintf("%d %s", i, cases[i].Name)
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	gentest.GeneratedParsersCorpus(t, goBin, cases, gen.GenOptions{})
	if node, err := exec.LookPath("node"); err == nil {
		gentest.TSParsersCorpus(t, node, cases, gen.GenOptions{})
	}
	gentest.GeneratedTypesCorpus(t, goBin, cases, gen.GenOptions{ConvertTypes: true}, 0)
}

// TestTermDifferentialDetectsBrokenTargets is the negative control of TestTermGeneratedTargets: with an
// operator of the generated Go or TypeScript parsers swapped for another, a failure of a term left unraised, the right operand of a logical operator always evaluated or an
// equality of texts negated, the comparison with the engine fails on some generated or fixed scalar term.
func TestTermDifferentialDetectsBrokenTargets(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testTermDifferentialDetectsBrokenTargets)
}

func testTermDifferentialDetectsBrokenTargets(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	cases, _ := wellTypedTermCases(t, 1, termDifferentialCases, true)
	// The fixed scalar terms fail, short-circuit and compare texts, which the generated terms rarely do.
	scalar, _ := wellTypedCases(t, gentest.ScalarTermCases())
	list := termCaseList(append(cases, scalar...))
	for i := range list {
		list[i].Name = fmt.Sprintf("%d %s", i, list[i].Name)
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	// Each mutant changes one pattern of the code, which must occur in the generated code. Every term of
	// these cases is printed from Exec IR, which calls the compositions with the constant of the operator.
	for _, m := range []struct{ name, from, to string }{
		{"lowered: plus becomes minus", `c.P.TermBinary(rt.OpAdd,`, `c.P.TermBinary(rt.OpSub,`},
		{"lowered: times becomes plus", `c.P.TermBinary(rt.OpMul,`, `c.P.TermBinary(rt.OpAdd,`},
		{"lowered: quotient becomes product", `c.P.TermBinary(rt.OpQuo,`, `c.P.TermBinary(rt.OpMul,`},
		{"lowered: remainder becomes quotient", `c.P.TermBinary(rt.OpRem,`, `c.P.TermBinary(rt.OpQuo,`},
		{"lowered: less becomes less or equal", `c.P.TermBinary(rt.OpLt,`, `c.P.TermBinary(rt.OpLe,`},
		{"lowered: equal becomes not equal", `c.P.TermBinary(rt.OpEq,`, `c.P.TermBinary(rt.OpNe,`},
		{"lowered: negation becomes not", `c.P.TermUnary(rt.OpNeg,`, `c.P.TermUnary(rt.OpNot,`},
		{"lowered: not becomes negation", `c.P.TermUnary(rt.OpNot,`, `c.P.TermUnary(rt.OpNeg,`},
		{"lowered: end is read as start", `rt.MemberEnd,`, `rt.MemberStart,`},
		{"lowered: a failure is not raised", `panic(tf`, `_ = (tf`},
		{"lowered: a logical operator always evaluates its right operand", `if !tb`, `if true || !tb`},
		{"lowered: a logical operator always evaluates its right operand (||)", `if tb`, `if true || tb`},
		{"lowered: a text equality is negated", `any(c.P.TextEqual(`, `any(!c.P.TextEqual(`},
	} {
		hits := 0
		got := gentest.GeneratedParsersMismatches(t, goBin, list, gen.GenOptions{}, func(code []byte) []byte {
			s := string(code)
			if n := strings.Count(s, m.from); n > 0 {
				hits++
				s = strings.ReplaceAll(s, m.from, m.to)
			}
			return []byte(s)
		})
		t.Logf("Go: %s: parsers changed %d of %d cases, mismatches %d", m.name, hits, len(list), len(got))
		if hits == 0 {
			t.Errorf("Go: %s: the generated code has no %q", m.name, m.from)
		} else if len(got) == 0 {
			t.Errorf("Go: %s: no generated term notices", m.name)
		}
	}
	node, err := exec.LookPath("node")
	if err != nil {
		return
	}
	for _, m := range []struct{ name, from, to string }{
		{"lowered: plus becomes minus", `c.p.termBinary(0,`, `c.p.termBinary(1,`},
		{"lowered: times becomes plus", `c.p.termBinary(2,`, `c.p.termBinary(0,`},
		{"lowered: quotient becomes product", `c.p.termBinary(3,`, `c.p.termBinary(2,`},
		{"lowered: remainder becomes quotient", `c.p.termBinary(4,`, `c.p.termBinary(3,`},
		{"lowered: less becomes less or equal", `c.p.termBinary(7,`, `c.p.termBinary(8,`},
		{"lowered: equal becomes not equal", `c.p.termBinary(5,`, `c.p.termBinary(6,`},
		{"lowered: negation becomes not", `c.p.termUnary(0,`, `c.p.termUnary(1,`},
		{"lowered: not becomes negation", `c.p.termUnary(1,`, `c.p.termUnary(0,`},
		{"lowered: length becomes zero", `c.p.termLength(`, `((x: Value): [Value, Value] => (c.p.termLength(x), [0, null]))(`},
		{"lowered: a failure is not raised", `evalError(tf`, `void (tf`},
		{"lowered: a logical operator always evaluates its right operand", `if (!tb`, `if (true || !tb`},
		{"lowered: a logical operator always evaluates its right operand (||)", `if (tb`, `if (true || tb`},
	} {
		hits := 0
		got := gentest.TSParsersMismatches(t, node, list, gen.GenOptions{}, func(code string) string {
			if strings.Contains(code, m.from) {
				hits++
				code = strings.ReplaceAll(code, m.from, m.to)
			}
			return code
		})
		t.Logf("TypeScript: %s: parsers changed %d of %d cases, mismatches %d", m.name, hits, len(list), len(got))
		if hits == 0 {
			t.Errorf("TypeScript: %s: the generated code has no %q", m.name, m.from)
		} else if len(got) == 0 {
			t.Errorf("TypeScript: %s: no generated term notices", m.name)
		}
	}
}

// TestTermConstructorsDetectBrokenTargets is the negative control of TestTermFixedGeneratedTargets for
// the constructors printed from Exec IR (tuples, lists, concat and structs) on generated Go (node and
// typed) and TypeScript parsers: each mutant changes one pattern of the lowered code or, for typed Go, of
// the struct constructors it calls, which must occur, and the comparison with the engine (for typed Go,
// also with the parser that converts its nodes) on the fixed struct, hull, push and cast terms must then
// fail. The control changes nothing and must find no mismatch.
func TestTermConstructorsDetectBrokenTargets(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testTermConstructorsDetectBrokenTargets)
}

func testTermConstructorsDetectBrokenTargets(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	var all []gentest.TermCase
	for _, set := range [][]gentest.TermCase{gentest.StructTermCases(), gentest.HullTermCases(), gentest.PushTermCases(),
		gentest.CastTermCases()} {
		all = append(all, set...)
	}
	cases, _ := wellTypedCases(t, all)
	list := termCaseList(cases)
	for i := range list {
		list[i].Name = fmt.Sprintf("%d %s", i, list[i].Name)
	}
	type mutant struct{ name, from, to string }
	check := func(target string, m mutant, hits int, got []string) {
		t.Logf("%s: %s: parsers changed %d of %d cases, mismatches %d", target, m.name, hits, len(list), len(got))
		switch {
		case hits == 0:
			t.Errorf("%s: %s: the generated code has no %q", target, m.name, m.from)
		case m.from == m.to && len(got) != 0:
			t.Errorf("%s: %s: %d mismatches without a change: %v", target, m.name, len(got), got)
		case m.from != m.to && len(got) == 0:
			t.Errorf("%s: %s: no generated term notices", target, m.name)
		}
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	for _, m := range []mutant{
		{"control: unchanged", `c.P.TermHull(c, `, `c.P.TermHull(c, `},
		{"a tuple becomes a list", `c.P.TermHull(c, true, `, `c.P.TermHull(c, false, `},
		{"a list becomes a tuple", `c.P.TermHull(c, false, `, `c.P.TermHull(c, true, `},
		{"concat pushes no elements", `c.P.TermPushItems(c, "concat", `, `c.P.TermPushItems(c, "concat", any(nil)); _ = (`},
		{"an element is pushed as nil", `c.P.TermPush(c, "list", `, `c.P.TermPush(c, "list", any(nil)); _ = (`},
		{"a field is set to nil", `c.TermSetField(`, `c.TermSetFieldNil(`},
		{"a struct ignores the ranges of its fields", `c.P.TermCover(`, `termCoverFirst(c.P, `},
	} {
		hits := 0
		got := gentest.GeneratedParsersMismatches(t, goBin, list, gen.GenOptions{}, func(code []byte) []byte {
			s := string(code)
			if strings.Contains(s, m.from) {
				hits++
				s = strings.ReplaceAll(s, m.from, m.to)
				if strings.Contains(m.to, "TermSetFieldNil") {
					s += "\nfunc tsfNil(c *rt.ActCtx, n *rt.Node, name string, _ any) { c.TermSetField(n, name, nil) }\n"
					s = strings.ReplaceAll(s, "c.TermSetFieldNil(", "tsfNil(c, ")
				}
				if strings.HasPrefix(m.to, "termCoverFirst(") {
					// Each field is covered as if it were the first, so the range is that of the last node.
					s += "\nfunc termCoverFirst(p *rt.Parser, n *rt.Node, v any, _ bool) bool { return p.TermCover(n, v, true) }\n"
				}
			}
			return []byte(s)
		})
		check("Go", m, hits, got)
	}
	// Typed Go: the typed parsers against the parsers that convert their nodes and against the engine.
	for _, m := range []mutant{
		{"control: unchanged", `c.P.TermHull(c, `, `c.P.TermHull(c, `},
		{"a tuple becomes a list", `c.P.TermHull(c, true, `, `c.P.TermHull(c, false, `},
		{"a list becomes a tuple", `c.P.TermHull(c, false, `, `c.P.TermHull(c, true, `},
		{"concat pushes no elements", `c.P.TermPushItems(c, "concat", `, `c.P.TermPushItems(c, "concat", any(nil)); _ = (`},
		{"an element is pushed as nil", `c.P.TermPush(c, "list", `, `c.P.TermPush(c, "list", any(nil)); _ = (`},
		// The constructor takes its first two fields in the other order, as a call that passed them
		// out of declaration order would.
		{"two fields of a struct are swapped", `final bool, f0 any, f1 any`, `final bool, f1 any, f0 any`},
		{"a struct ignores the ranges of its fields", `return c.MadeOf(v, `, `return c.MadeOf(v) // `},
	} {
		hits := 0
		got := gentest.GeneratedTypesMismatches(t, goBin, list, gen.GenOptions{ConvertTypes: true}, func(code []byte) []byte {
			s := string(code)
			if strings.Contains(s, m.from) {
				hits++
				s = strings.ReplaceAll(s, m.from, m.to)
			}
			return []byte(s)
		}, 0)
		check("typed Go", m, hits, got)
	}
	node, err := exec.LookPath("node")
	if err != nil {
		return
	}
	for _, m := range []mutant{
		{"control: unchanged", `c.p.termHull(c, `, `c.p.termHull(c, `},
		{"a tuple becomes a list", `c.p.termHull(c, true, `, `c.p.termHull(c, false, `},
		{"a list becomes a tuple", `c.p.termHull(c, false, `, `c.p.termHull(c, true, `},
		{"concat pushes no elements", `c.p.termPushItems(c, "concat", `, `c.p.termPushItems(c, "concat", null); void (`},
		{"an element is pushed as nil", `c.p.termPush(c, "list", `, `c.p.termPush(c, "list", null); void (`},
		{"a field is set to nil", `c.termSetField(`, `((n: Node, f: string, _v: Value) => c.termSetField(n, f, null))(`},
		{"a struct ignores the ranges of its fields", `c.p.termCover(`, `((n: Node, v: Value, _f: boolean) => c.p.termCover(n, v, true))(`},
	} {
		hits := 0
		got := gentest.TSParsersMismatches(t, node, list, gen.GenOptions{}, func(code string) string {
			if strings.Contains(code, m.from) {
				hits++
				code = strings.ReplaceAll(code, m.from, m.to)
			}
			return code
		})
		check("TypeScript", m, hits, got)
	}
}

// TestTermIterationsDetectBrokenTargets is the negative control of TestTermFixedGeneratedTargets for the
// loops of map, filter, foldl and foldr printed from Exec IR on generated Go (node and typed) and TypeScript
// parsers: each mutant changes one pattern of the lowered loops, which must occur, and the comparison with
// the engine on the fixed iteration terms must then fail. The control changes nothing and must find no
// mismatch.
func TestTermIterationsDetectBrokenTargets(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testTermIterationsDetectBrokenTargets)
}

// iterMutantHelpers are the functions that the mutants of TestTermIterationsDetectBrokenTargets call in
// generated Go, for the node and the typed parser alike.
const iterMutantHelpers = `
func itemAtFirst[S any, P interface{ TermItemAt(S, int, bool) any }](p P, items S, i int, _ bool) any {
	return p.TermItemAt(items, i, false)
}
func itemAtLast[S any, P interface{ TermItemAt(S, int, bool) any }](p P, items S, i int, _ bool) any {
	return p.TermItemAt(items, i, true)
}
func keepAll[P interface{ TermFilterKeep(any) (bool, any) }](p P, v any) (bool, any) {
	_, f := p.TermFilterKeep(v)
	return true, f
}
`

func testTermIterationsDetectBrokenTargets(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	cases, _ := wellTypedCases(t, gentest.IterTermCases())
	list := termCaseList(cases)
	for i := range list {
		list[i].Name = fmt.Sprintf("%d %s", i, list[i].Name)
	}
	type mutant struct{ name, from, to string }
	check := func(target string, m mutant, hits int, got []string) {
		t.Logf("%s: %s: parsers changed %d of %d cases, mismatches %d", target, m.name, hits, len(list), len(got))
		switch {
		case hits == 0:
			t.Errorf("%s: %s: the generated code has no %q", target, m.name, m.from)
		case m.from == m.to && len(got) != 0:
			t.Errorf("%s: %s: %d mismatches without a change: %v", target, m.name, len(got), got)
		case m.from != m.to && len(got) == 0:
			t.Errorf("%s: %s: no generated term notices", target, m.name)
		}
	}
	goMutants := []mutant{
		{"control: unchanged", `c.P.TermItemAt(`, `c.P.TermItemAt(`},
		{"the folds take the elements from the first", `c.P.TermItemAt(`, `itemAtFirst(c.P, `},
		{"the iterations take the elements from the last", `c.P.TermItemAt(`, `itemAtLast(c.P, `},
		{"filter keeps every element", `c.P.TermFilterKeep(`, `keepAll(c.P, `},
		{"map pushes nil for every element", `c.P.TermPush(c, "map", `, `c.P.TermPush(c, "map", any(nil)); _ = (`},
		{"map takes no elements", `c.P.TermItems("map", `, `c.P.TermItems("map", any(nil)); _ = (`},
		{"foldl takes no elements", `c.P.TermItems("foldl", `, `c.P.TermItems("foldl", any(nil)); _ = (`},
	}
	mutate := func(m mutant, hits *int) func(code []byte) []byte {
		return func(code []byte) []byte {
			s := string(code)
			if strings.Contains(s, m.from) {
				*hits++
				s = strings.ReplaceAll(s, m.from, m.to) + iterMutantHelpers
			}
			return []byte(s)
		}
	}
	// The map of a projected repetition makes a list of no elements. A typed parser holds node and typed
	// actions, which print the call as any(c.NewList(...)) and (c.NewList(...)), of the elements in a local
	// named ti.
	noList := mutant{"the map of a projected repetition makes no elements", `c.NewList(ti`, `newListNone(c, ti`}
	mutateGo := func(m mutant, hits *int) func(code []byte) []byte {
		if m != noList {
			return mutate(m, hits)
		}
		return func(code []byte) []byte {
			s := string(code)
			if strings.Contains(s, m.from) {
				*hits++
				s = strings.ReplaceAll(s, "any("+m.from, "any("+m.to)
				s = strings.ReplaceAll(s, m.from, "newListNoneT(c, ti")
				s += "\nfunc newListNone(c *rt.ActCtx, _ []*rt.Node) *rt.Node { return c.NewList(nil) }\n" +
					"func newListNoneT(c *rt.TActCtx, _ []any) any { return c.NewList(nil) }\n"
			}
			return []byte(s)
		}
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	for _, m := range append(goMutants, noList) {
		hits := 0
		got := gentest.GeneratedParsersMismatches(t, goBin, list, gen.GenOptions{}, mutateGo(m, &hits))
		check("Go", m, hits, got)
	}
	for _, m := range append(goMutants, noList) {
		hits := 0
		got := gentest.GeneratedTypesMismatches(t, goBin, list, gen.GenOptions{ConvertTypes: true}, mutateGo(m, &hits), 0)
		check("typed Go", m, hits, got)
	}
	node, err := exec.LookPath("node")
	if err != nil {
		return
	}
	for _, m := range []mutant{
		{"control: unchanged", `c.p.termItemAt(`, `c.p.termItemAt(`},
		{"the folds take the elements from the first", `c.p.termItemAt(`, `((it: readonly (Node | null)[], i: number, _r: boolean) => c.p.termItemAt(it, i, false))(`},
		{"the iterations take the elements from the last", `c.p.termItemAt(`, `((it: readonly (Node | null)[], i: number, _r: boolean) => c.p.termItemAt(it, i, true))(`},
		{"filter keeps every element", `c.p.termFilterKeep(`, `((v: Value): [boolean, Value] => [true, c.p.termFilterKeep(v)[1]])(`},
		{"map pushes nil for every element", `c.p.termPush(c, "map", `, `c.p.termPush(c, "map", null); void (`},
		{"map takes no elements", `c.p.termItems("map", `, `c.p.termItems("map", null); void (`},
		{"foldl takes no elements", `c.p.termItems("foldl", `, `c.p.termItems("foldl", null); void (`},
		{"the map of a projected repetition makes no elements", `c.newList(`, `c.newList([]); void (`},
	} {
		hits := 0
		got := gentest.TSParsersMismatches(t, node, list, gen.GenOptions{}, func(code string) string {
			if strings.Contains(code, m.from) {
				hits++
				code = strings.ReplaceAll(code, m.from, m.to)
			}
			return code
		})
		check("TypeScript", m, hits, got)
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
				env := &ref.TermEnv{Captures: map[string]ref.V{"x": ref.Node(ref.StringNode, c.Lit, 0, w, nil, nil)}, Bytes: unit == engine.Bytes,
					Start: 0, End: w, Line: 1, Col: 1, Zeros: referenceZeros(t, prog.Static(), "R", "Inner", "Leaf")}
				in := c.Lit
				if c.Kind == gentest.TermPredicate {
					in += "b"
				}
				checkReference(t, c, prog, term, env, unit, in)
				checked++
			}
		}
	}
	if checked == 0 {
		t.Fatal("no case was checked")
	}
}

// checkReference compares the engine's closure backend on the input in with the reference evaluator
// in env: the value or the error of an action, and for a predicate, which precedes an optional "b",
// whether the predicate held, by whether an input that ends in "b" parses.
func checkReference(t *testing.T, c gentest.TermCase, prog *engine.Program, term grammar.Term, env *ref.TermEnv, unit engine.PositionUnit, in string) {
	t.Helper()
	v, msg := env.Eval(term)
	n, err := prog.ParseWith("main", in, engine.ParseOptions{Backend: engine.Closure, Unit: unit})
	if c.Kind == gentest.TermPredicate {
		held := err == nil && n != nil
		if want := !strings.HasSuffix(in, "b") || ref.Holds(v, msg); held != want {
			t.Errorf("%s: input %q unit %d: the engine parses: %v, the reference evaluator: %v (%s)", c.Name, in, unit, held, want, msg)
		}
		return
	}
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
		t.Errorf("%s: input %q unit %d: the engine gives\n %s\nthe reference evaluator\n %s", c.Name, in, unit, got, wantJSON)
	}
}

// fixedTermCases returns the fixed term sets of gentest other than the members, which
// TestTermGeneratedTargets reads: conversions, structs, casts, list and tuple values, iteration, pushes
// and scalar operators.
func fixedTermCases() []gentest.TermCase {
	var all []gentest.TermCase
	for _, set := range [][]gentest.TermCase{gentest.ConvertTermCases(), gentest.StructTermCases(), gentest.CastTermCases(),
		gentest.HullTermCases(), gentest.IterTermCases(), gentest.PushTermCases(), gentest.ScalarTermCases()} {
		all = append(all, set...)
	}
	return all
}

// TestTermFixedReference checks the engine's closure backend against the reference evaluator on the
// fixed term sets and the members, in both position units: the same value or error for every input of an action, and
// whether a predicate held. The captures come from the hand-written tables of the cases, so the
// reference evaluator reads the nodes that the rules define, not the engine's. With
// TestTermFixedGeneratedTargets, which compares the generated targets with the engine on the same
// cases, every target gives the reference value.
func TestTermFixedReference(t *testing.T) {
	t.Parallel()
	checked := 0
	for _, c := range append(gentest.MemberTermCases(), fixedTermCases()...) {
		g, err := parseTest(c.Src)
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		prog, err := engine.Compile(g, compile.Options{})
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		var structs []string
		for _, d := range g.File.Decls {
			if td, ok := d.(*grammar.TypeDef); ok {
				if _, ok := td.Type.(*grammar.StructType); ok {
					structs = append(structs, td.Name)
				}
			}
		}
		zeros := referenceZeros(t, prog.Static(), structs...)
		term := referenceTerm(t, c)
		for _, in := range c.Inputs {
			for _, unit := range []engine.PositionUnit{engine.CodePoints, engine.Bytes} {
				bytes := unit == engine.Bytes
				end := utf8.RuneCountInString(in)
				if bytes {
					end = len(in)
				}
				env := &ref.TermEnv{Captures: referenceCaptures(c.Captures(in), in, bytes), Bytes: bytes, Start: 0, End: end, Line: 1, Col: 1, Zeros: zeros}
				checkReference(t, c, prog, term, env, unit, in)
				checked++
			}
		}
	}
	if checked == 0 {
		t.Fatal("no case was checked")
	}
}

// referenceCaptures turns the captures of a fixed case, in characters of the input in, into nodes of the
// reference evaluator in the position unit; a capture that did not match is left out.
func referenceCaptures(caps map[string]gentest.Capture, in string, bytes bool) map[string]ref.V {
	runes := []rune(in)
	offset := func(i int) int {
		if bytes {
			return len(string(runes[:i]))
		}
		return i
	}
	var node func(c gentest.Capture) ref.V
	node = func(c gentest.Capture) ref.V {
		shape := map[gentest.CaptureShape]ref.Shape{gentest.StringCapture: ref.StringNode, gentest.TupleCapture: ref.TupleNode,
			gentest.ListCapture: ref.ListNode}
		if c.Shape == gentest.Unmatched {
			return ref.Nil
		}
		var items []ref.V
		for _, e := range c.Elems {
			items = append(items, node(e))
		}
		return ref.Node(shape[c.Shape], string(runes[c.From:c.To]), offset(c.From), offset(c.To), items, referenceCaptures(c.Caps, in, bytes))
	}
	out := map[string]ref.V{}
	for name, c := range caps {
		if c.Shape != gentest.Unmatched {
			out[name] = node(c)
		}
	}
	return out
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
