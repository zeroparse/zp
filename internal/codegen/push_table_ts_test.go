package codegen_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
	"zeroparse.dev/zp/internal/ir/exec/conform"
)

// pushScript runs the calls of calls.json on the printed methods termPush and termPushItems of the TypeScript
// library and prints, in order, the canonical result of each (conform.MemberCanon), the number of elements it
// pushed and the units it charged. The input of the parser is input.txt.
const pushScript = `import { readFileSync } from "node:fs";
import * as p from "./parser.ts";
` + tsCompose + `const CONTROLLED = false;
const P = Object.create(p.Parser.prototype);
P.control = CONTROLLED ? new p.BudgetControl({ signal: null, budget: 1e15 } as unknown as p.ResolvedParseOptions) : null;
P.created = [];
P.kidStack = [];
P.src = readFileSync("input.txt", "utf8");
P.offs = null;
P.unit = p.Bytes;
const c = new p.Actx(P);
const values: Record<string, p.Value> = {
  "nil": null, "1": 1, '"ab"': "ab", "true": true, "[1, 2]": fxList(c, 1, 2), "[]": fxList(c),
  "func": ((x: p.Value) => x) as unknown as p.Value,
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
const used = (): number => (CONTROLLED ? (P.control as any).budgetUsed : 0);
const calls: [string, string][] = JSON.parse(readFileSync("calls.json", "utf8"));
console.log(JSON.stringify(calls.map(([fn, v]) => {
  const base = P.kidStack.length;
  const before = used();
  const e = fn === "push" ? P.termPush(c, "list", values[v]!) : P.termPushItems(c, "concat", values[v]!);
  const units = used() - before;
  const pushed: (p.Node | null)[] = P.kidStack.slice(base);
  P.kidStack.length = base;
  let r: string;
  if (e !== null) r = "error: " + (e as string);
  else if (fn === "push") r = pushed.length === 1 ? canon(pushed[0]) : "nil";
  else r = canon(c.listNode(pushed));
  return r + "\t" + pushed.length + "\t" + units;
})));
`

// runTSPushes runs the push table on the TypeScript library after mutate changed its source, with a budget
// control if controlled, and returns the result of each call by its call and value.
func runTSPushes(t *testing.T, controlled bool, mutate func(string) string) map[[2]string]string {
	t.Helper()
	node := codegentest.RequireNode(t)
	g, err := parseTest("@start\nrule main = \"a\"\n")
	if err != nil {
		t.Fatal(err)
	}
	code, err := codegentest.GenerateTSBundle(g, codegen.GenOptions{Start: "main"})
	if err != nil {
		t.Fatal(err)
	}
	src := string(code)
	if mutate != nil {
		src = mutate(src)
	}
	var calls [][2]string
	for _, c := range conform.PushCases {
		calls = append(calls, [2]string{c.Call, c.Value})
	}
	cj, _ := json.Marshal(calls)
	dir := t.TempDir()
	script := strings.Replace(pushScript, "const CONTROLLED = false;", "const CONTROLLED = "+strconv.FormatBool(controlled)+";", 1)
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
		t.Fatalf("node: %v\n%s", err, codegentest.Clip(stderr.String(), 0))
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

// pushResult splits the result of a call into the canonical value, the elements pushed and the units.
func pushResult(got map[[2]string]string, call, v string) (string, int, int) {
	parts := strings.Split(got[[2]string{call, v}], "\t")
	if len(parts) != 3 {
		return got[[2]string{call, v}], -1, -1
	}
	pushed, _ := strconv.Atoi(parts[1])
	units, _ := strconv.Atoi(parts[2])
	return parts[0], pushed, units
}

// TestValuePushTableTypeScript runs the push table (conform.PushCases) on the TypeScript library.
func TestValuePushTableTypeScript(t *testing.T) {
	t.Parallel()
	for _, controlled := range []bool{false, true} {
		got := runTSPushes(t, controlled, nil)
		if err := conform.RunPushes("typescript controlled="+strconv.FormatBool(controlled), controlled, func(call, v string) (string, int, int) {
			return pushResult(got, call, v)
		}); err != nil {
			t.Error(err)
		}
	}
}

// TestPrintedPushMutantsTypeScript is the proof that the push table holds the printed code of the compositions
// push and pushItems of the TypeScript library: with the source of each mutant, it fails.
func TestPrintedPushMutantsTypeScript(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the tests on mutated sources")
	}
	t.Parallel()
	mutants := []struct{ name, sig, old, new string }{
		{"a bool is not an element", "termPush(c: Actx", "k === 1 || k === 2 || k === 3", "k === 1 || k === 2"},
		{"a function is an element", "termPush(c: Actx", "return this.termError(11, v, fn, 0, 0);", "return null;"},
		{"nil pushes no element", "termPush(c: Actx", "this.kidStack.push(null);", ""},
		{"concat pushes no elements", "termPushItems(c: Actx", "c.pushAll(items);", ""},
		{"concat takes a value that is not a list", "termPushItems(c: Actx", "if (ok) {", "if (true) {"},
		{"concat charges nothing for the elements it pushes", "pushAll(items:", "this.spend();", ""},
	}
	for _, m := range mutants {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			got := runTSPushes(t, true, func(src string) string {
				out, ok := mutateTSFunc(src, m.sig, m.old, m.new, false)
				if !ok {
					t.Fatalf("the library has no %s with %q", m.sig, m.old)
				}
				return out
			})
			if conform.RunPushes(m.name, true, func(call, v string) (string, int, int) { return pushResult(got, call, v) }) == nil {
				t.Error("the push table did not notice")
			}
		})
	}
}
