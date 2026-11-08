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

const termChargesGolden = "testdata/term_charges.golden"

var termChargesGoldenFile = gentest.GoldenFile{
	Path: termChargesGolden, CaseOf: gentest.GoldenCaseBefore,
	Header: "# The charges of the generated term differential cases (gentest.TermCases) on every target: the units the\n" +
		"# engine's closure backend, bytecode VM and iterative VM, the node and the typed generated Go parsers and the\n" +
		"# generated TypeScript parser charge for each input, and the outcome (ok, or the error) each reports. The targets\n" +
		"# charge terms in their own ways today, so the columns differ; the golden freezes each, so that a change of a\n" +
		"# term's charges on one target is a stated change. The terms leave no struct field unset.\n" + gentest.GoldenRules,
}

// termChargeCases returns the term cases of the golden, which the compiler accepts.
func termChargeCases(t *testing.T) []gentest.TermCase {
	t.Helper()
	var out []gentest.TermCase
	for _, c := range gentest.TermCases(1, 40, false) {
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
	gentest.NeedsBudgetControls(t)
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	terms := termChargeCases(t)
	cases := make([]gentest.Case, len(terms))
	for i, c := range terms {
		cases[i] = c.Case
		cases[i].Name = fmt.Sprintf("term %d", i)
	}
	got := map[string]string{}
	key := func(i int, input, target string) string {
		return fmt.Sprintf("termcharges/term %d: %s | %s %s", i, terms[i].Term, input, target)
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
		totals, problems := budgetTotalsWith(t, rt, "termcharges", cases, "main", nil, run.opts, run.parse)
		for _, p := range problems {
			t.Error(p)
		}
		for k, v := range totals {
			m := regexp.MustCompile(`^termcharges/term (\d+) \| (\d+) (\w+)$`).FindStringSubmatch(k)
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
	termChargesGoldenFile.Check(t, "termcharges/", got)
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
