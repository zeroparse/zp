package tsgen_test

import (
	"testing"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
)

// A budget of zero selects the default of max(2^20, 1024 times the input length) in the position unit,
// for every entry point, and Unlimited disables the limit. The grammar scans the rest of the input from
// every position of a scan from every position, so its cost grows with the cube of the input length; the
// call of q keeps the inner scan from being a scan site, whose records would answer it again.
func TestGeneratedTSDefaultBudget(t *testing.T) {
	t.Parallel()
	g, err := parseTest(`rule main = (. &mid)* !.
rule mid = (. &(.* q))*
rule q = ""`)
	if err != nil {
		t.Fatal(err)
	}
	out := gentest.RunTSGrammarScriptWith(t, g, `import * as p from "./parser.ts";
declare const process: { exit(code: number): never };
function fail(why: string): never { console.log(why); return process.exit(0); }
const limitOf = (e: unknown): number => e instanceof p.ParseInterruptionError && e.kind === "compute-limit" ? e.limit : -1;
const want = 1 << 20;
const short = "a".repeat(300);
if (limitOf(p.parse(short).error) !== want) fail("parse default " + p.parse(short).error);
if (limitOf(p.parseRule("", short).error) !== want) fail("parseRule default");
if (limitOf(p.recognize(short)) !== want) fail("recognize default");
if (limitOf(p.parseValue(short).error) !== want) fail("parseValue default");
if (limitOf(p.parseWithOptions(short, {}).error) !== want) fail("parseWithOptions default");
if (limitOf(p.parseWithOptions(short, { limits: { compute: 0 } }).error) !== want) fail("zero budget default");
if (limitOf(p.parseWithOptions(short, { limits: { compute: 1000 } }).error) !== 1000) fail("explicit budget");
const long = "a".repeat(5000);
if (limitOf(p.parseWithOptions(long, { unit: p.Bytes }).error) !== 5000 * 1024) fail("default scales with the input: " + limitOf(p.parseWithOptions(long, { unit: p.Bytes }).error));
const wide = "\u00e9".repeat(2000);
if (limitOf(p.parseWithOptions(wide, { unit: p.Bytes }).error) !== 4000 * 1024) fail("bytes of a string");
if (limitOf(p.parseWithOptions(wide, { unit: p.CodePoints }).error) !== 2000 * 1024) fail("code points of a string");
if (limitOf(p.parseWithOptions(new TextEncoder().encode(wide), { unit: p.CodePoints }).error) !== 2000 * 1024) fail("code points of bytes");
if (p.Unlimited !== Number.MAX_SAFE_INTEGER) fail("Unlimited");
console.log("ok");
`, gen.GenOptions{Start: "main", LimitControls: true, Types: true, Recognize: true})
	if out != "ok\n" {
		t.Fatalf("got %q", out)
	}
}

// Unlimited needs no limit: the parser runs to the end, and a parser generated without budget controls
// accepts it too.
func TestGeneratedTSUnlimitedBudget(t *testing.T) {
	t.Parallel()
	g, err := parseTest(`rule main = "a"+ !.`)
	if err != nil {
		t.Fatal(err)
	}
	for _, controls := range []bool{true, false} {
		out := gentest.RunTSGrammarScriptWith(t, g, `import * as p from "./parser.ts";
const options = { limits: { compute: Number.MAX_SAFE_INTEGER } } as p.ParseOptions;
const r = p.parseWithOptions("a".repeat(100), options);
if (r.error !== null || r.node === null) throw new Error("unlimited parse failed: " + r.error);
console.log("ok");
`, gen.GenOptions{Start: "main", LimitControls: controls, Types: true, Recognize: true})
		if out != "ok\n" {
			t.Fatalf("controls=%v: got %q", controls, out)
		}
	}
}

// maxInput bounds the length of the input in the position unit for every entry point: zero and
// Unlimited set no bound, an input of exactly maxInput units parses, and a longer one fails before any
// parsing with a ParseInterruptionError of kind "input-limit" and its length. A parser generated without
// budget controls rejects a finite bound.
func TestGeneratedTSMaxInput(t *testing.T) {
	t.Parallel()
	g, err := parseTest(`rule main = [a-zé]* !.`)
	if err != nil {
		t.Fatal(err)
	}
	out := gentest.RunTSGrammarScriptWith(t, g, `import * as p from "./parser.ts";
declare const process: { exit(code: number): never };
function fail(why: string): never { console.log(why); return process.exit(0); }
const text = "a".repeat(300) + "é";
for (const [unit, n] of [[p.CodePoints, 301], [p.Bytes, 302]] as const) {
  for (const maxInput of [0, p.Unlimited, n, n + 1]) {
    const r = p.parseWithOptions(text, { unit, limits: { input: maxInput } });
    if (r.error !== null) fail("maxInput " + maxInput + ": " + r.error);
    if (p.recognizeWithOptions(text, { unit, limits: { input: maxInput } }) !== null) fail("recognize " + maxInput);
    if (p.parseValueWithOptions(text, { unit, limits: { input: maxInput } }).error !== null) fail("parseValue " + maxInput);
  }
  for (const [name, error] of [
    ["parse", p.parseWithOptions(text, { unit, limits: { input: n - 1 } }).error],
    ["parseRule", p.parseRuleWithOptions("", text, { unit, limits: { input: n - 1 } }).error],
    ["recognize", p.recognizeWithOptions(text, { unit, limits: { input: n - 1 } })],
    ["parseValue", p.parseValueWithOptions(text, { unit, limits: { input: n - 1 } }).error],
    ["bytes", p.parseWithOptions(new TextEncoder().encode(text), { unit, limits: { input: n - 1 } }).error],
  ] as const) {
    if (!(error instanceof p.ParseInterruptionError) || error.kind !== "input-limit" || error.limit !== n - 1 || error.requested !== n || error.used !== 0) fail(name + " unit " + unit + ": " + error);
    if (error.message !== "input-limit exceeded: limit " + (n - 1) + ", length " + n) fail("message " + error.message);
  }
}
const invalid = new Uint8Array(100).fill(0x80);
const bad = p.parseWithOptions(invalid, { unit: p.CodePoints, limits: { input: 99 } }).error;
if (!(bad instanceof p.ParseInterruptionError) || bad.kind !== "input-limit" || bad.requested !== 100) fail("invalid UTF-8: " + bad);
const cancelled = { aborted: true } as p.CancellationSignal;
const first = p.parseWithOptions(text, { signal: cancelled, limits: { input: 1 } }).error;
if (!(first instanceof p.ParseInterruptionError) || first.kind !== "input-limit") fail("input limit comes before the signal: " + first);
if (p.parseWithOptions("abc", { limits: { input: -1 } }).error === null) fail("negative maxInput");
console.log("ok");
`, gen.GenOptions{Start: "main", LimitControls: true, Types: true, Recognize: true})
	if out != "ok\n" {
		t.Fatalf("got %q", out)
	}
	// Without limit controls a finite input limit is checked by length; a finite compute limit is
	// an error.
	out = gentest.RunTSGrammarScriptWith(t, g, `import * as p from "./parser.ts";
console.log(p.parseWithOptions("abc", { limits: { input: 3 } }).error === null ? "accepted" : "rejected");
const over = p.parseWithOptions("abc", { limits: { input: 2 } }).error;
console.log(over instanceof p.ParseInterruptionError && over.kind === "input-limit" ? over.message : String(over));
console.log(String(p.parseWithOptions("abc", { limits: { compute: 3 } } as p.ParseOptions).error));
`, gen.GenOptions{Start: "main", LimitControls: false, Types: true, Recognize: true})
	if want := "accepted\ninput-limit exceeded: limit 2, length 3\nError: this parser was generated without limit controls (generate with -limits to use signal and the compute and memory limits)\n"; out != want {
		t.Fatalf("without budget controls: got %q", out)
	}
}
