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

// structScript makes the structs of cases.json on the TypeScript runtime (fxStruct of tsCompose, over the printed
// methods termCover and termMade) and prints the range of each, and whether the context recorded it, in
// order. The input of the parser is input.txt.
const structScript = `import { readFileSync } from "node:fs";
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
c.start = RULE_START;
c.end = RULE_END;
const text = new p.Node("string", "", 2, 4, "xyz", p.noNodes, p.noFields, false, true);
const values: Record<string, p.Value> = {
  "nil": null, "1": 1, '"ab"': "ab", "text": text,
  "node": new p.Node("tuple", "", 2, 5, "", [text, text], p.noFields, false, false),
  "late": new p.Node("string", "", 7, 9, "q", p.noNodes, p.noFields, false, true),
};
const cases: string[][] = JSON.parse(readFileSync("cases.json", "utf8"));
console.log(JSON.stringify(cases.map((fields) => {
  try {
    const before = P.created.length;
    const kv: p.Value[] = [];
    fields.forEach((f, i) => kv.push("F" + i, values[f]!));
    const n = fxStruct(c, "S", ...kv) as p.Node;
    const recorded = P.created.length === before + 1 && P.created[before] === n;
    return n.start + "-" + n.end + (recorded ? "RECORDED" : "");
  } catch (x) {
    return x instanceof FxError ? "error: " + x.msg : "exception: " + String(x);
  }
})));
`

// runTSStructs runs the struct table on the TypeScript runtime after mutate changed its source, with a
// budget control if controlled, and returns the result of each case by its fields.
func runTSStructs(t *testing.T, controlled bool, mutate func(string) string) map[string]string {
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
	var cases [][]string
	for _, c := range conform.StructCases {
		cases = append(cases, append([]string{}, c.Fields...))
	}
	cj, _ := json.Marshal(cases)
	script := strings.NewReplacer("const CONTROLLED = false;", "const CONTROLLED = "+strconv.FormatBool(controlled)+";",
		"RULE_START", strconv.Itoa(conform.StructRuleStart), "RULE_END", strconv.Itoa(conform.StructRuleEnd),
		"RECORDED", conform.StructRecorded).Replace(structScript)
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
		got[strings.Join(c, ",")] = results[i]
	}
	return got
}

// TestValueStructTableTypeScript runs the struct table (conform.StructCases) on the TypeScript runtime.
func TestValueStructTableTypeScript(t *testing.T) {
	t.Parallel()
	for _, controlled := range []bool{false, true} {
		got := runTSStructs(t, controlled, nil)
		if err := conform.RunStructs("typescript controlled="+strconv.FormatBool(controlled), func(fields []string) string { return got[strings.Join(fields, ",")] }); err != nil {
			t.Error(err)
		}
	}
}

// TestPrintedStructMutantsTypeScript is the proof that the struct table holds the printed code of the
// range of a struct in the TypeScript runtime: with the source of each mutant, it fails.
func TestPrintedStructMutantsTypeScript(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the tests on mutated sources")
	}
	t.Parallel()
	mutants := []struct{ name, sig, old, new string }{
		{"a node field value does not cover", "termCover(n: Node", "if (!ok) {", "if (true) {"},
		{"the start of the range is the latest", "termCover(n: Node", "cs < ns", "cs > ns"},
		{"the end of the range is the earliest", "termCover(n: Node", "ce > ne", "ce < ne"},
		{"a later node replaces the range", "termCover(n: Node", "first || cs < ns", "true"},
		{"a struct without a node keeps no range", "termMade(c: Actx", "if (first) {", "if (false) {"},
		{"a struct is not recorded", "termMade(c: Actx", "this.created.push(n);", ""},
	}
	for _, m := range mutants {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			got := runTSStructs(t, false, func(src string) string {
				out, ok := mutateTSFunc(src, m.sig, m.old, m.new, false)
				if !ok {
					t.Fatalf("the runtime has no %s with %q", m.sig, m.old)
				}
				return out
			})
			if conform.RunStructs(m.name, func(fields []string) string { return got[strings.Join(fields, ",")] }) == nil {
				t.Error("the struct table did not notice")
			}
		})
	}
}
