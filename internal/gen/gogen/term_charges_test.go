package gogen_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/engine"
	"zeroparse.dev/zp/internal/engine/core"
	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
	"zeroparse.dev/zp/internal/limits"
)

// termChargesGoldenFile returns the golden of the charges of the terms of the generator; what names it in
// the header.
func termChargesGoldenFile(path, what, unset string) gentest.GoldenFile {
	return gentest.GoldenFile{Path: path, CaseOf: gentest.GoldenCaseBefore,
		Header: "# The charges of the generated term differential cases (" + what + ") on every target: the units the\n" +
			"# engine's closure backend, bytecode VM and iterative VM, the node and the typed generated Go parsers and the\n" +
			"# generated TypeScript parser charge for each input, and the outcome (ok, or the error) each reports. The targets\n" +
			"# charge terms in their own ways today, so the columns differ; the golden freezes each, so that a change of a\n" +
			"# term's charges on one target is a stated change. " + unset + "\n" + gentest.GoldenRules}
}

// termChargeCases returns the term cases of the golden, which the compiler accepts.
func termChargeCases(t *testing.T, all []gentest.TermCase) []gentest.TermCase {
	t.Helper()
	var out []gentest.TermCase
	for _, c := range all {
		g, err := parseTest(c.Src)
		if err == nil {
			_, err = engine.Compile(g, compile.Options{})
		}
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		out = append(out, c)
	}
	return out
}

// outcome shortens a result to ok or the first line of its error.
func termOutcome(res string) string {
	res = strings.Trim(res, `"`)
	if res == "<nil>" || res == "" {
		return "ok"
	}
	if i := strings.Index(res, `\n`); i >= 0 {
		res = res[:i]
	}
	return res
}

// TestTermChargesAcrossTargets records what every target charges for the generated terms, and checks
// that the golden still holds. It asserts no equality between targets: their counts differ by contract
// today (spec/runtime/parsing.md), and the golden is the baseline against which later stages state the
// changes they make to them.
func TestTermChargesAcrossTargets(t *testing.T) {
	t.Parallel()
	checkTermCharges(t, "testdata/term_charges.golden", "gentest.TermCases", "The terms leave no struct field unset.", "termcharges", gentest.TermCases(1, 40, false))
}

// TestTermChargesAcrossTargetsWide pins the charges of the terms of gentest.TermCasesWide, which have
// strings beyond the Basic Multilingual Plane, other literals, tuples, lists of structured values and
// the errors of run time. The terms of seeds 5 to 12 leave struct fields unset, which read as their zero values, so
// that the golden sees the operators applied to the zero values. The targets charge them in their own
// ways, like the terms above.
func TestTermChargesAcrossTargetsWide(t *testing.T) {
	t.Parallel()
	var all []gentest.TermCase
	for seed := uint64(1); seed <= 12; seed++ {
		all = append(all, gentest.TermCasesWide(seed, 20, seed > 4)...)
	}
	checkTermCharges(t, "testdata/term_charges_wide.golden", "gentest.TermCasesWide, seeds 1 to 12", "Some terms leave struct fields unset.", "termchargeswide", all)
}

// TestTermChargesMember pins the charges of gentest.MemberTermCases, whose terms read every kind of
// member, set fields and the zero values of unset lists, tuples, optionals and structs among them, so
// that a change in how a target reads a member shows as a stated change of the units it charges.
func TestTermChargesMember(t *testing.T) {
	t.Parallel()
	checkTermCharges(t, "testdata/term_charges_member.golden", "gentest.MemberTermCases", "Some terms leave struct fields unset.", "termchargesmember", gentest.MemberTermCases())
}

// TestTermChargesConvert pins the charges of gentest.ConvertTermCases, whose terms convert string, tuple
// and list nodes and nodes of named node types to a named node type, so that a change in how a target
// converts a value shows as a stated change of the units it charges.
func TestTermChargesConvert(t *testing.T) {
	t.Parallel()
	checkTermCharges(t, "testdata/term_charges_convert.golden", "gentest.ConvertTermCases", "The terms make no struct value.", "termchargesconvert", gentest.ConvertTermCases())
}

// TestTermChargesStruct pins the charges of gentest.StructTermCases, whose terms make struct values of
// fields that are nodes, so that a change in how a target makes a struct and sets its range shows as a stated
// change of the units it charges.
func TestTermChargesStruct(t *testing.T) {
	t.Parallel()
	checkTermCharges(t, "testdata/term_charges_struct.golden", "gentest.StructTermCases", "Some terms leave struct fields unset.", "termchargesstruct", gentest.StructTermCases())
}

func checkTermCharges(t *testing.T, golden, what, unset, prefix string, all []gentest.TermCase) {
	t.Helper()
	gentest.NeedsBudgetControls(t)
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	terms := termChargeCases(t, all)
	cases := make([]gentest.Case, len(terms))
	for i, c := range terms {
		cases[i] = c.Case
		cases[i].Name = fmt.Sprintf("term %d", i)
	}
	got := map[string]string{}
	key := func(i int, input, target string) string {
		return fmt.Sprintf("%s/term %d: %s | %s %s", prefix, i, terms[i].Term, input, target)
	}
	inputIndex := func(i int, input string) int {
		for j, in := range terms[i].Inputs {
			if in == input {
				return j
			}
		}
		panic("no input")
	}
	// The engine.
	for i, c := range terms {
		g, _ := parseTest(c.Src)
		prog, err := engine.Compile(g, compile.Options{})
		if err != nil {
			t.Fatal(err)
		}
		for _, in := range c.Inputs {
			for _, b := range []struct {
				name string
				be   engine.Backend
			}{{"engine-closure", engine.Closure}, {"engine-vm", engine.Bytecode}, {"engine-iterative", engine.BytecodeIterative}} {
				var used uint64
				_, err := prog.ParseWith("main", in, engine.ParseOptions{Backend: b.be, Limits: limits.Limits{Compute: 1 << 50},
					Trace: func(e engine.TraceEvent) { used = core.TraceParser(e).Control.Used }})
				got[key(i, in, b.name)] = fmt.Sprintf("%d %s", used, termOutcome(fmt.Sprint(err)))
			}
		}
	}
	// Generated Go: the node parser's Parse and Recognize, and the typed parser's ParseAST.
	rt := gentest.RuntimeCopy()
	for _, run := range []struct {
		name  string
		opts  gen.GenOptions
		parse string
	}{{"go-node", gen.GenOptions{}, "Parse"}, {"go-typed", gen.GenOptions{Types: true}, "ParseAST"}} {
		totals, problems := budgetTotalsWith(t, rt, prefix, cases, "main", nil, run.opts, run.parse)
		for _, p := range problems {
			t.Error(p)
		}
		for k, v := range totals {
			m := regexp.MustCompile(`^` + prefix + `/term (\d+) \| (\d+) (\w+)$`).FindStringSubmatch(k)
			if m == nil {
				t.Fatalf("unexpected key %q", k)
			}
			var i, j int
			fmt.Sscan(m[1], &i)
			fmt.Sscan(m[2], &j)
			f := strings.SplitN(v, " ", 3)
			target := run.name
			if m[3] == "recognize" {
				target += "-recognize"
			}
			got[key(i, terms[i].Inputs[j], target)] = f[0] + " " + termOutcome(f[2])
		}
	}
	_ = inputIndex
	// Generated TypeScript.
	for i, c := range terms {
		g, _ := parseTest(c.Src)
		var ins []string
		for _, in := range c.Inputs {
			ins = append(ins, fmt.Sprintf("%q", in))
		}
		out := gentest.RunTSGrammarScriptWith(t, g, tsChargeScript(ins), gen.GenOptions{Start: "main", LimitControls: true})
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) != len(c.Inputs) {
			t.Fatalf("term %d: %d lines for %d inputs:\n%s", i, len(lines), len(c.Inputs), out)
		}
		for j, in := range c.Inputs {
			n, res, _ := strings.Cut(lines[j], " ")
			got[key(i, in, "ts-node")] = n + " " + termOutcome(res)
		}
	}
	termChargesGoldenFile(golden, what, unset).Check(t, prefix+"/", got)
}

// tsChargeScript is a TypeScript consumer that prints, for each input, the least compute limit under which
// the parse is not interrupted, and its outcome.
func tsChargeScript(inputs []string) string {
	return `import * as p from "./parser.ts";
const inputs: string[] = [` + strings.Join(inputs, ", ") + `];
for (const s of inputs) {
  const run = (n: number) => p.parseWithOptions(s, { limits: { compute: n } });
  const limited = (n: number) => { const e = run(n).error; return e instanceof p.ParseInterruptionError && e.kind === "compute-limit"; };
  let hi = 1;
  while (limited(hi)) hi *= 2;
  let lo = 1;
  while (lo < hi) { const mid = (lo + hi) >> 1; if (limited(mid)) lo = mid + 1; else hi = mid; }
  const e = run(lo).error;
  console.log(lo + " " + JSON.stringify(e === null ? "<nil>" : String(e.message).split("\n")[0]));
}
`
}
