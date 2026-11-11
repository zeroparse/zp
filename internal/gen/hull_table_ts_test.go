package gen_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
	"zeroparse.dev/zp/internal/ir/exec/conform"
)

// hullScript makes the tuples and lists of cases.json on the TypeScript runtime (Actx.tuple and
// Actx.newList, over the printed method termHull), prints the range of each, and the units that the
// composition charges over the elements again, in order. The input of the parser is input.txt.
const hullScript = `import { readFileSync } from "node:fs";
import * as p from "./parser.ts";
const CONTROLLED = false;
const P = Object.create(p.Parser.prototype);
P.control = CONTROLLED ? new p.BudgetControl({ signal: null, budget: 1e15 } as unknown as p.ResolvedParseOptions) : null;
P.created = [];
P.kidStack = [];
P.src = readFileSync("input.txt", "utf8");
P.offs = null;
P.unit = p.Bytes;
const c = new p.Actx(P);
c.start = RULE_START;
c.end = RULE_END;
const text = new p.Node("string", "", 2, 4, "xyz", p.noNodes, p.noFields, false, true);
const values: Record<string, p.Value> = {
  "nil": null, "1": 1, '"ab"': "ab", "text": text,
  "node": new p.Node("tuple", "", 2, 5, "", [text, text], p.noFields, false, false),
  "late": new p.Node("string", "", 7, 9, "q", p.noNodes, p.noFields, false, true),
};
const used = (): number => (CONTROLLED ? (P.control as any).budgetUsed : 0);
// pushed returns the elements that list makes of vs.
const pushed = (vs: p.Value[]): (p.Node | null)[] => {
  const base = P.kidStack.length;
  for (const v of vs) c.push("list", v);
  return P.kidStack.splice(base);
};
const cases: [string, string[]][] = JSON.parse(readFileSync("cases.json", "utf8"));
console.log(JSON.stringify(cases.map(([kind, elems]) => {
  try {
    const vs = elems.map((e) => values[e]!);
    const n = (kind === "tuple" ? c.tuple(...vs) : c.newList(pushed(vs))) as p.Node;
    const rng = n.start + "-" + n.end;
    const before = used();
    P.termHull(c, kind === "tuple", n.children);
    return rng + " " + (used() - before);
  } catch (x) {
    return x instanceof p.EvalError ? "error: " + x.msg : "exception: " + String(x);
  }
})));
`

func hullKey(kind string, elems []string) string { return kind + ":" + strings.Join(elems, ",") }

// runTSHulls runs the hull table on the TypeScript runtime after mutate changed its source, with a
// budget control if controlled, and returns the result of each case by its kind and elements.
func runTSHulls(t *testing.T, controlled bool, mutate func(string) string) map[string]string {
	t.Helper()
	node := gentest.RequireNode(t)
	g, err := parseTest("@start\nrule main = \"a\"\n")
	if err != nil {
		t.Fatal(err)
	}
	code, err := gentest.GenerateTSBundle(g, gen.GenOptions{Start: "main"})
	if err != nil {
		t.Fatal(err)
	}
	src := string(code)
	if mutate != nil {
		src = mutate(src)
	}
	var cases [][2]any
	for _, kind := range conform.HullKinds {
		for _, c := range conform.HullCases {
			cases = append(cases, [2]any{kind, append([]string{}, c.Elems...)})
		}
	}
	cj, _ := json.Marshal(cases)
	script := strings.NewReplacer("const CONTROLLED = false;", "const CONTROLLED = "+strconv.FormatBool(controlled)+";",
		"RULE_START", strconv.Itoa(conform.StructRuleStart), "RULE_END", strconv.Itoa(conform.StructRuleEnd),
	).Replace(hullScript)
	dir := t.TempDir()
	for name, content := range map[string]string{"package.json": `{"type": "module"}`, "parser.ts": src, "main.ts": script,
		"cases.json": string(cj), "input.txt": conform.BuiltinInput} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(node, "main.ts")
	cmd.Dir = dir
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, gentest.Clip(stderr.String(), 0))
	}
	var results []string
	if err := json.Unmarshal(out, &results); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if len(results) != len(cases) {
		t.Fatalf("%d results for %d cases", len(results), len(cases))
	}
	got := map[string]string{}
	for i, c := range cases {
		got[hullKey(c[0].(string), c[1].([]string))] = results[i]
	}
	return got
}

// TestValueHullTableTypeScript runs the hull table (conform.HullCases) on the TypeScript runtime.
func TestValueHullTableTypeScript(t *testing.T) {
	t.Parallel()
	for _, controlled := range []bool{false, true} {
		got := runTSHulls(t, controlled, nil)
		if err := conform.RunHulls("typescript controlled="+strconv.FormatBool(controlled), controlled, func(kind string, elems []string) (string, int) {
			return splitHull(got[hullKey(kind, elems)])
		}); err != nil {
			t.Error(err)
		}
	}
}

// splitHull splits the result of a case into the range and the units.
func splitHull(r string) (string, int) {
	rng, units, _ := strings.Cut(r, " ")
	n, _ := strconv.Atoi(units)
	return rng, n
}

// TestPrintedHullMutantsTypeScript is the proof that the hull table holds the printed code of the
// range of a list or tuple in the TypeScript runtime: with the source of each mutant, it fails.
func TestPrintedHullMutantsTypeScript(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the tests on mutated sources")
	}
	t.Parallel()
	mutants := []struct{ name, sig, old, new string }{
		{"an element of a hull charges no step", "termHull(c: Actx", "this.spend();", ""},
		{"an element that adds no range widens the range", "termHull(c: Actx", "if (has) {", "if (has || true) {"},
		{"the start of a hull is the latest", "termHull(c: Actx", "es < lo", "es > lo"},
		{"the end of a hull is the earliest", "termHull(c: Actx", "ee > hi", "ee < hi"},
		{"a later element replaces the range of a hull", "termHull(c: Actx", "first || es < lo", "true"},
		{"an empty hull is not at the start of the rule", "termHull(c: Actx", "let lo = rs;", "let lo = 0;"},
	}
	for _, m := range mutants {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			got := runTSHulls(t, true, func(src string) string {
				out, ok := mutateTSFunc(src, m.sig, m.old, m.new, false)
				if !ok {
					t.Fatalf("the runtime has no %s with %q", m.sig, m.old)
				}
				return out
			})
			if conform.RunHulls(m.name, true, func(kind string, elems []string) (string, int) { return splitHull(got[hullKey(kind, elems)]) }) == nil {
				t.Error("the hull table did not notice")
			}
		})
	}
}
