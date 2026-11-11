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

// iterScript runs the calls of calls.json on the TypeScript runtime (rtFold, Actx.mapList and filterList, over
// the printed methods termItemAt and termFilterKeep) and prints the canonical result of each
// (conform.MemberCanon) and the units it charged, in order. The input of the parser is input.txt.
const iterScript = `import { readFileSync } from "node:fs";
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
const values: Record<string, p.Value> = {
  "nil": null, "1": 1, '"ab"': "ab", "true": true, "[1, 2]": c.list(1, 2), "[]": c.list(),
};
const canon = (v: p.Value): string => {
  if (v === null || v === undefined) return "nil";
  if (typeof v === "number" || typeof v === "bigint" || typeof v === "boolean") return String(v);
  if (typeof v === "string") return JSON.stringify(v);
  if (v instanceof p.Node) {
    if (v.type === p.typeBox) return "box " + canon(v.box);
    const elems = v.children.map((k) => canon(p.unboxNode(k)));
    if (v.type === p.typeListValue) return "[" + elems.join(", ") + "]";
    return v.type;
  }
  return "not a value: " + String(v);
};
const call = (fn: string, v: p.Value): p.Value => {
  switch (fn) {
    case "foldl": case "foldr": return p.rtFold(c, fn, fn === "foldr", null, v, (_acc, item) => item);
    case "map": return c.mapList(v, (item) => item);
    case "filter": return c.filterList(v, (item) => item === 1);
  }
  return c.filterList(v, (item) => item);
};
const used = (): number => (CONTROLLED ? (P.control as any).budgetUsed : 0);
const calls: [string, string][] = JSON.parse(readFileSync("calls.json", "utf8"));
console.log(JSON.stringify(calls.map(([fn, v]) => {
  const before = used();
  try {
    return canon(call(fn, values[v]!)) + "\t" + (used() - before);
  } catch (x) {
    return (x instanceof p.EvalError ? "error: " + x.msg : "exception: " + String(x)) + "\t" + (used() - before);
  }
})));
`

// runTSIters runs the iteration table on the TypeScript runtime after mutate changed its source, with a
// budget control if controlled, and returns the result of each call by its call and value.
func runTSIters(t *testing.T, controlled bool, mutate func(string) string) map[[2]string]string {
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
	var calls [][2]string
	for _, c := range conform.IterCases {
		calls = append(calls, [2]string{c.Call, c.Value})
	}
	cj, _ := json.Marshal(calls)
	dir := t.TempDir()
	script := strings.Replace(iterScript, "const CONTROLLED = false;", "const CONTROLLED = "+strconv.FormatBool(controlled)+";", 1)
	for name, content := range map[string]string{"package.json": `{"type": "module"}`, "parser.ts": src, "main.ts": script,
		"calls.json": string(cj), "input.txt": conform.BuiltinInput} {
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
	if len(results) != len(calls) {
		t.Fatalf("%d results for %d calls", len(results), len(calls))
	}
	got := map[[2]string]string{}
	for i, c := range calls {
		got[c] = results[i]
	}
	return got
}

// iterResult splits the result of a call into the canonical value and the units.
func iterResult(got map[[2]string]string, call, v string) (string, int) {
	r, units, _ := strings.Cut(got[[2]string{call, v}], "\t")
	n, _ := strconv.Atoi(units)
	return r, n
}

// TestValueIterTableTypeScript runs the iteration table (conform.IterCases) on the TypeScript runtime.
func TestValueIterTableTypeScript(t *testing.T) {
	t.Parallel()
	for _, controlled := range []bool{false, true} {
		got := runTSIters(t, controlled, nil)
		if err := conform.RunIters("typescript controlled="+strconv.FormatBool(controlled), controlled, func(call, v string) (string, int) {
			return iterResult(got, call, v)
		}); err != nil {
			t.Error(err)
		}
	}
}

// TestPrintedIterMutantsTypeScript is the proof that the iteration table holds the printed code of the
// iterating built-in functions of the TypeScript runtime: with the source of each mutant, it fails.
func TestPrintedIterMutantsTypeScript(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the tests on mutated sources")
	}
	t.Parallel()
	mutants := []struct{ name, sig, old, new string }{
		{"a fold step charges nothing", "rtFold(c: Actx,", "c.spend();", ""},
		{"foldr passes the elements from the first", "termItemAt(items:", "if (right) {", "if (false) {"},
		{"the folds pass the elements in each other's order", "termItemAt(items:", "if (right) {", "if (!right) {"},
		{"an iteration passes a box", "termItemAt(items:", "return unboxValue(items[i]!);", "return items[i]!;"},
		{"filter takes a result that is not a bool", "termFilterKeep(v: Value)", "if (valKind(v) !== 3) {", "if (valKind(v) === 0) {"},
		{"filter drops what it keeps", "termFilterKeep(v: Value)", "return [(v as boolean), null];", "return [!(v as boolean), null];"},
	}
	for _, m := range mutants {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			got := runTSIters(t, true, func(src string) string {
				out, ok := mutateTSFunc(src, m.sig, m.old, m.new, false)
				if !ok {
					t.Fatalf("the runtime has no %s with %q", m.sig, m.old)
				}
				return out
			})
			if conform.RunIters(m.name, true, func(call, v string) (string, int) { return iterResult(got, call, v) }) == nil {
				t.Error("the iteration table did not notice")
			}
		})
	}
}
