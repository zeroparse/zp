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

// memberScript runs the reads of reads.json on the member reads of the TypeScript runtime (Actx.member,
// over the printed method termMember) and prints the canonical result of each (conform.MemberCanon), in
// order. The fixture is that of the member table: its struct types come from structs.json.
const memberScript = `import { readFileSync } from "node:fs";
import * as p from "./parser.ts";
const CONTROLLED = false;
const P = Object.create(p.Parser.prototype);
P.control = CONTROLLED ? new p.BudgetControl({ signal: null, budget: 0 } as unknown as p.ResolvedParseOptions) : null;
P.created = [];
P.kidStack = [];
const structs: [string, [string, string][]][] = JSON.parse(readFileSync("structs.json", "utf8"));
P.g = { structFields: new Map(structs.map(([name, fields]) => [name, p.structFieldsOf(fields)])) };
const c = new p.Actx(P);
const node = new p.Node("tuple", "", 2, 5, "", p.noNodes, p.noFields, false, false);
const seven = new p.Node(p.typeBox, "", 0, 0, "", p.noNodes, p.noFields, false, false);
seven.box = 7;
p.setField(node, "y", seven, null);
const values: Record<string, p.Value> = {
  "nil": null, "1": 1, "node": node, "S{}": c.newStruct("S"), 'S{A: 3, B: "x"}': c.newStruct("S", "A", 3, "B", "x"), "[]": c.list(),
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
const reads: [string, string][] = JSON.parse(readFileSync("reads.json", "utf8"));
console.log(JSON.stringify(reads.map(([v, m]) => {
  try {
    return canon(c.member(values[v]!, m));
  } catch (x) {
    return x instanceof p.EvalError ? "error: " + x.msg : "exception: " + String(x);
  }
})));
`

// runTSMembers runs the member table on the TypeScript runtime after mutate changed its source, with a
// budget control if controlled, and returns the result of each read by its value and member.
func runTSMembers(t *testing.T, controlled bool, mutate func(string) string) map[[2]string]string {
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
	var structs []any
	for _, s := range conform.MemberStructs {
		structs = append(structs, []any{s.Name, s.Fields})
	}
	var reads [][2]string
	for _, c := range conform.MemberCases {
		reads = append(reads, [2]string{c.Value, c.Member})
	}
	sj, _ := json.Marshal(structs)
	rj, _ := json.Marshal(reads)
	dir := t.TempDir()
	script := strings.Replace(memberScript, "const CONTROLLED = false;", "const CONTROLLED = "+strconv.FormatBool(controlled)+";", 1)
	for name, content := range map[string]string{"package.json": `{"type": "module"}`, "parser.ts": src, "main.ts": script,
		"structs.json": string(sj), "reads.json": string(rj)} {
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
	if len(results) != len(reads) {
		t.Fatalf("%d results for %d reads", len(results), len(reads))
	}
	got := map[[2]string]string{}
	for i, r := range reads {
		got[r] = results[i]
	}
	return got
}

// TestValueMemberTableTypeScript runs the member table (conform.MemberCases) on the TypeScript runtime.
func TestValueMemberTableTypeScript(t *testing.T) {
	t.Parallel()
	for _, controlled := range []bool{false, true} {
		got := runTSMembers(t, controlled, nil)
		if err := conform.RunMembers("typescript controlled="+strconv.FormatBool(controlled), func(v, m string) string { return got[[2]string{v, m}] }); err != nil {
			t.Error(err)
		}
	}
}

// TestPrintedMemberMutantsTypeScript is the proof that the member table holds the printed code of the
// member reads of the TypeScript runtime: with the source of each mutant, it fails.
func TestPrintedMemberMutantsTypeScript(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the tests on mutated sources")
	}
	t.Parallel()
	mutants := []struct{ name, sig, old, new string }{
		{"Start reads the value", "termMember(c: Actx", "if (kind === 1) {", "if (kind === 0) {"},
		{"an unset field of a struct is nil", "termMember(c: Actx", "if (!st) {", "if (true) {"},
		{"a struct has captures", "termMember(c: Actx", "if (st) {", "if (false) {"},
		{"a set field of a struct reads as its zero", "termMember(c: Actx", "if (set) {", "if (set && !st) {"},
		{"a capture keeps its box", "termMember(c: Actx", "return [unboxValue(v), null];", "return [v, null];"},
		{"End is a field", "memberKind(name: string)", "return 2; // End", "return 0; // End"},
		{"a capture is read by its whole name", "memberName(kind: number", "name.slice(1)", "name"},
	}
	for _, m := range mutants {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			got := runTSMembers(t, false, func(src string) string {
				out, ok := mutateTSFunc(src, m.sig, m.old, m.new, false)
				if !ok {
					t.Fatalf("the runtime has no %s with %q", m.sig, m.old)
				}
				return out
			})
			if conform.RunMembers(m.name, func(v, mem string) string { return got[[2]string{v, mem}] }) == nil {
				t.Error("the member table did not notice")
			}
		})
	}
}
