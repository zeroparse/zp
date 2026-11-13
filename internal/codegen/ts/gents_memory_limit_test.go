package tsgen_test

import (
	"strconv"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/gentest"
)

// memoryLimitScript finds the count of a parse as the smallest memory limit it succeeds under, and
// checks the contract of the memory limit against it: the count is the same on every run, a limit
// one below it fails with the kind memory-limit reporting the count as used, a small limit fails
// during the parse, Unlimited sets no bound, and the count grows linearly with the input.
const memoryLimitScript = `import * as p from "./parser.ts";
declare const process: { exit(code: number): never };
function fail(why: string): never { console.log(why); return process.exit(0); }
const input = (n: number): string => INPUT;
// run returns the interruption of a parse, if any; a parse that recovers errors still completes.
const run = (s: string, memory: number): Error | null => {
  const e = p.parseWithOptions(s, { limits: { memory } }).error;
  return e instanceof p.ParseInterruptionError ? e : null;
};
const memoryError = (e: unknown): p.ParseInterruptionError | null => e instanceof p.ParseInterruptionError && e.kind === "memory-limit" ? e : null;
function count(s: string): number {
  if (run(s, 0) !== null) fail("the parse is interrupted without a memory limit: " + run(s, 0));
  let lo = 1, hi = 1 << 30;
  if (run(s, hi) !== null) fail("the count passes 2^30");
  while (lo < hi) {
    const mid = Math.floor((lo + hi) / 2);
    const e = run(s, mid);
    if (e === null) hi = mid;
    else if (memoryError(e) === null) fail("not a memory-limit failure: " + e);
    else lo = mid + 1;
  }
  return lo;
}
const s = input(1000);
const used = count(s);
if (count(s) !== used) fail("the count differs between runs");
const below = memoryError(run(s, used - 1));
if (below === null) fail("one below the count does not fail");
if (below.limit !== used - 1 || below.used !== used || below.requested !== 0) fail("limit, used, requested: " + below.limit + ", " + below.used + ", " + below.requested + "; count " + used);
if (below.message !== "memory-limit exceeded: limit " + (used - 1) + ", used " + used) fail("message: " + below.message);
const small = memoryError(run(s, 4096));
if (small === null || small.used <= 4096 || small.used >= used) fail("a small limit does not fail during the parse: " + (small?.used ?? null) + " of " + used);
if (run(s, p.Unlimited) !== null) fail("Unlimited fails");
const double = count(input(2000));
if (double < 1.6 * used || double > 2.4 * used) fail("not linear: " + used + " then " + double);
console.log("ok");
`

// A finite memory limit of a generated TypeScript parser fails a parse whose count passes it, and
// the count is deterministic and linear in the input, for a grammar that builds nodes and lists, one
// that grows left-recursive seeds, and one that recovers errors.
func TestGeneratedTSMemoryLimit(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, grammar, input string }{
		{"lists", `rule main = item* !.
rule item = word:[a-z]+ " "?`, `"abc ".repeat(n / 4)`},
		{"left recursion", `rule main = e !.
rule e = e "+" n / n
rule n = [0-9]+`, `"1" + "+12".repeat(n / 3)`},
		{"recovery", `rule main = item* !.
rule item = ("a" "!") #recover(skip=(!";" .)*) ";"`, `"a!;ab;".repeat(n / 6)`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g, err := parseTest(c.grammar)
			if err != nil {
				t.Fatal(err)
			}
			script := strings.Replace(memoryLimitScript, "INPUT", c.input, 1)
			out := gentest.RunTSGrammarScriptWith(t, g, script, codegen.GenOptions{Start: "main", LimitControls: true})
			if out != "ok\n" {
				t.Fatalf("got %q", out)
			}
		})
	}
}

// memoryGrowthScript prints by how much the memory count of a parse grows from 200 items to 400:
// the count is the least memory limit that the parse passes.
const memoryGrowthScript = `import * as p from "./parser.ts";
const ok = (s: string, memory: number): boolean => p.parseWithOptions(s, { limits: { memory } }).error === null;
function count(n: number): number {
  const s = "a ".repeat(n);
  let lo = 1, hi = 1 << 30;
  while (lo < hi) {
    const mid = Math.floor((lo + hi) / 2);
    if (ok(s, mid)) hi = mid; else lo = mid + 1;
  }
  return lo;
}
console.log(String(count(400) - count(200)));
`

// The values that actions build are counted whichever path builds them: a list, a tuple, a struct
// and a converted node each add, per item, the bytes of the size model over an action that returns
// the matched node.
func TestGeneratedTSMemoryLimitValues(t *testing.T) {
	t.Parallel()
	const decls = "type S = node[string]\ntype P = { A node[string] }\n"
	count := func(t *testing.T, action string) int {
		g, err := parseTest(decls + "rule main = item* !.\nrule item = w:[a-z] \" \" => " + action)
		if err != nil {
			t.Fatal(err)
		}
		out := gentest.RunTSGrammarScriptWith(t, g, memoryGrowthScript, codegen.GenOptions{Start: "main", LimitControls: true})
		v, err := strconv.Atoi(strings.TrimSpace(out))
		if err != nil {
			t.Fatalf("output %q", out)
		}
		return v
	}
	base := count(t, "$w")
	// The bytes per item by the size model: a Node is 112 bytes, an array of n elements 48 + 8n (32
	// when empty), and an object of k fields 24 + 8k.
	for _, c := range []struct {
		name, action string
		perItem      int
	}{
		{"list", "list($w)", 112 + 56},            // the list node and its child list
		{"tuple", "($w, $w)", 112 + 64},           // the tuple node and its child list
		{"struct", "P{A: $w}", 112 + 48 + 8 + 40}, // the node, its field array, slot and record
		{"convert", "S{$w}", 112 + 8 + 32 + 32},   // the copy of the node with its child and field lists
	} {
		t.Run(c.name, func(t *testing.T) {
			got := (count(t, c.action) - base) / 200
			if got != c.perItem {
				t.Fatalf("%s adds %d bytes per item over returning the node, want %d", c.action, got, c.perItem)
			}
		})
	}
}
