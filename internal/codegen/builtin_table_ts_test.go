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

// builtinScript runs the calls of calls.json on the TypeScript library (the printed methods termLength,
// termText, termItems and termConvert) and prints the canonical result of each (conform.MemberCanon), in order. The input of the parser is input.txt.
const builtinScript = `import { readFileSync } from "node:fs";
import * as p from "./parser.ts";
` + tsCompose + `const CONTROLLED = false;
const P = Object.create(p.Parser.prototype);
P.control = CONTROLLED ? new p.BudgetControl({ signal: null, budget: 0 } as unknown as p.ResolvedParseOptions) : null;
P.created = [];
P.kidStack = [];
P.src = readFileSync("input.txt", "utf8");
P.offs = null;
P.unit = p.Bytes;
const c = new p.Actx(P);
const text = new p.Node("string", "", 2, 4, "xyz", p.noNodes, p.noFields, false, true);
const values: Record<string, p.Value> = {
  "nil": null, "1": 1, '"ab"': "ab", "true": true, "text": text,
  "node": new p.Node("tuple", "", 2, 5, "", [text, text], p.noFields, false, false),
  "[1, 2]": fxList(c, 1, 2), "[]": fxList(c), "func": ((x: p.Value) => x) as unknown as p.Value,
  "typed": p.convertNode("M", text), "box": (fxPush(c, "list", 1), P.kidStack.pop()!),
};
const canon = (v: p.Value): string => {
  if (v === null || v === undefined) return "nil";
  if (typeof v === "number" || typeof v === "bigint" || typeof v === "boolean") return String(v);
  if (typeof v === "string") return JSON.stringify(v);
  if (v instanceof p.Node) {
    if (v.type === p.typeBox) return "box " + canon(v.box);
    const elems = v.children.map((k) => canon(p.unboxNode(k)));
    if (v.type === p.typeListValue) return "[" + elems.join(", ") + "]";
    if (v.type === p.typeTupleValue) return "(" + elems.join(", ") + ")";
    return v.type;
  }
  return "not a value: " + String(v);
};
const call = (fn: string, v: p.Value): p.Value => {
  switch (fn) {
    case "len": return fxValue(P.termLength(v));
    case "text": return fxValue(P.termText(v));
    case "convert": return fxValue(P.termConvert("N", v));
  }
  return c.listNode([...fxItems(c, "concat", v)]);
};
const calls: [string, string][] = JSON.parse(readFileSync("calls.json", "utf8"));
console.log(JSON.stringify(calls.map(([fn, v]) => {
  try {
    return canon(call(fn, values[v]!));
  } catch (x) {
    return x instanceof FxError ? "error: " + x.msg : "exception: " + String(x);
  }
})));
`

// runTSBuiltins runs the built-in table on the TypeScript library after mutate changed its source, with a
// budget control if controlled, and returns the result of each call by its call and value.
func runTSBuiltins(t *testing.T, controlled bool, mutate func(string) string) map[[2]string]string {
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
	for _, c := range conform.BuiltinCases {
		calls = append(calls, [2]string{c.Call, c.Value})
	}
	cj, _ := json.Marshal(calls)
	dir := t.TempDir()
	script := strings.Replace(builtinScript, "const CONTROLLED = false;", "const CONTROLLED = "+strconv.FormatBool(controlled)+";", 1)
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

// TestValueBuiltinTableTypeScript runs the built-in table (conform.BuiltinCases) on the TypeScript library.
func TestValueBuiltinTableTypeScript(t *testing.T) {
	t.Parallel()
	for _, controlled := range []bool{false, true} {
		got := runTSBuiltins(t, controlled, nil)
		if err := conform.RunBuiltins("typescript controlled="+strconv.FormatBool(controlled), func(call, v string) string { return got[[2]string{call, v}] }); err != nil {
			t.Error(err)
		}
	}
}

// TestPrintedBuiltinMutantsTypeScript is the proof that the built-in table holds the printed code of the
// built-in functions of the TypeScript library: with the source of each mutant, it fails.
func TestPrintedBuiltinMutantsTypeScript(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the tests on mutated sources")
	}
	t.Parallel()
	mutants := []struct{ name, sig, old, new string }{
		{"nil has length 1", "termLength(x: Value)", "return [0, null];", "return [1, null];"},
		{"the length of a string node is that of its text", "termLength(x: Value)", "if (n.terminal) {", "if (false) {"},
		{"concat takes any value", "termItems(fn: string", "return [items, this.termError(12, v, fn, 0, 0)];", "return [items, null];"},
		{"the text of a string is empty", "termText(x: Value)", "return [(x as string), null];", "return [\"\", null];"},
		{"a list value converts", "termConvert(t: string", "if ((nodeShape(n) <= shapeList)) {", "if (true) {"},
		{"a value that does not convert is nil", "termConvert(t: string", "return [null, this.termError(15, x, t, 0, 0)];", "return [null, null];"},
	}
	for _, m := range mutants {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			got := runTSBuiltins(t, false, func(src string) string {
				out, ok := mutateTSFunc(src, m.sig, m.old, m.new, false)
				if !ok {
					t.Fatalf("the library has no %s with %q", m.sig, m.old)
				}
				return out
			})
			if conform.RunBuiltins(m.name, func(call, v string) string { return got[[2]string{call, v}] }) == nil {
				t.Error("the built-in table did not notice")
			}
		})
	}
}
