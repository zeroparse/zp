package tsgen_test

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/testing/codegentest"
)

// observeOptionsScript checks the meaning of the observation options of a generated TypeScript parser:
// a trace function or a profile, or an array of them, in which null entries are ignored and each
// receives the whole parse in order; the trace filter and depth limit select the events of the trace
// functions only; and the trace functions and profiles of the call of a Document parse add to the
// Document's defaults while its trace filter and depth limit replace them, for that parse only.
const observeOptionsScript = `import * as p from "./parser.ts";
const check = (ok, what) => { if (!ok) console.log("FAIL " + what); };
const same = (a, b) => JSON.stringify(a) === JSON.stringify(b);
const untimed = (profile) => {
  const s = profile.snapshot();
  s.time = 0;
  for (const r of s.rules) { r.time = 0; r.selfTime = 0; }
  return s;
};
const input = "ab";

// Spellings: one function, or an array with null entries; each function receives every event, in order.
const one = [];
check(p.parseWithOptions(input, { trace: (e) => { one.push(e); } }).error === null, "single trace parse");
const log = [], a = [], b = [];
check(p.parseWithOptions(input, { trace: [null, (e) => { a.push(e); log.push("a"); }, null, (e) => { b.push(e); log.push("b"); }] }).error === null, "array trace parse");
check(one.length > 4 && same(one, a) && same(a, b), "every trace function receives the whole parse");
check(log.every((x, i) => x === (i % 2 === 0 ? "a" : "b")), "trace functions receive each event in order");
const p1 = new p.Profile(), p2 = new p.Profile(), p3 = new p.Profile();
p.parseWithOptions(input, { profile: p1 });
p.parseWithOptions(input, { profile: [p2, null, p3] });
check(same(untimed(p1), untimed(p2)) && same(untimed(p2), untimed(p3)) && untimed(p1).parses === 1, "every profile receives the whole parse");
const twice = new p.Profile();
p.parseWithOptions(input, { profile: [twice, twice] });
check(untimed(twice).parses === 2 && untimed(twice).calls === 2 * untimed(p1).calls, "a profile given twice records the parse twice");

// A trace function that throws ends the parse with what it threw; the later functions do not receive the event.
const thrown = { thrown: true };
let later = 0, caught;
try {
  p.parseWithOptions(input, { trace: [(e) => { if (e.kind === "exit") throw thrown; }, () => { later++; }] });
} catch (x) { caught = x; }
check(caught === thrown, "a trace function's throw ends the parse");
check(later === one.findIndex((e) => e.kind === "exit"), "later trace functions do not receive the event of the throw");

// Invalid entries are errors of the options.
for (const [options, want] of [[{ trace: [() => {}, 1] }, "trace must be"], [{ trace: 1 }, "trace must be"], [{ profile: [p1, {}] }, "profile must be"], [{ profile: {} }, "profile must be"]]) {
  const r = p.parseWithOptions(input, options);
  check(r.error !== null && r.error.message.includes(want), "invalid option " + JSON.stringify(Object.keys(options)) + ": " + (r.error && r.error.message));
}

// The trace filter and depth limit select the events of the trace functions, never what profiles record:
// ref records a parse whose events all reach a trace function.
const ref = new p.Profile();
p.parseWithOptions(input, { trace: () => {}, profile: ref });
check(same(untimed(ref), untimed(p1)), "a profile records the same with and without trace functions");
const filtered = [], fp = new p.Profile(), deep = [], dp = new p.Profile();
p.parseWithOptions(input, { trace: (e) => { filtered.push(e); }, profile: fp, traceRules: ["no such rule"] });
p.parseWithOptions(input, { trace: (e) => { deep.push(e); }, profile: dp, traceMaxDepth: 1 });
check(filtered.length === 0 && same(untimed(fp), untimed(ref)), "the trace filter leaves profiles whole");
check(deep.length === 2 && deep.every((e) => e.depth === 1) && same(untimed(dp), untimed(ref)), "the depth limit leaves profiles whole");
const unfiltered = new p.Profile();
p.parseWithOptions(input, { profile: unfiltered, traceRules: ["no such rule"], traceMaxDepth: 1 });
check(same(untimed(unfiltered), untimed(ref)), "a filter without trace functions leaves profiles whole");

// Documents: the call's functions and profiles add to the defaults; its filter and depth limit replace
// them for that parse only. A full-text edit before each parse makes each parse make every call.
const fd = [], fc = [], pd = new p.Profile(), pc = new p.Profile();
const defaults = { trace: (e) => { fd.push(e); }, profile: pd, traceRules: ["main"], traceMaxDepth: 1 };
const { document, error } = p.createDocument(input, defaults);
check(error === null, "document");
const reparse = (options) => {
  check(document.edit(0, input.length, input) === null, "full-text edit");
  return document.parse(options).error;
};
check(reparse() === null, "default parse");
check(fd.length === 2 && fd.every((e) => e.rule === "main" && e.depth === 1) && untimed(pd).parses === 1, "the defaults apply: " + fd.length);
fd.length = 0;
check(reparse({ trace: [(e) => { fc.push(e); }], profile: pc, traceRules: [], traceMaxDepth: 0 }) === null, "parse with options");
check(same(fd, one) && same(fc, one), "the call's trace functions add to the defaults, and its filter and depth limit replace them: " + fd.length + " " + fc.length);
check(untimed(pd).parses === 2 && same(untimed(pc), untimed(p1)), "the call's profiles add to the defaults");
fd.length = 0; fc.length = 0;
check(reparse({ traceRules: ["part"] }) === null, "parse with a filter");
check(fd.length === 0 && fc.length === 0, "a filter of the call keeps the default depth limit: " + fd.length);
check(reparse({ traceMaxDepth: 2 }) === null, "parse with a depth limit");
check(same(fd, one) && fc.length === 0, "a depth limit of the call keeps the default filter: " + fd.length);
fd.length = 0;
check(reparse() === null, "default parse again");
check(fd.length === 2 && fd.every((e) => e.rule === "main" && e.depth === 1) && fc.length === 0 && untimed(pd).parses === 5 && untimed(pc).parses === 1,
  "a later parse has the defaults again: " + fd.length);
check(defaults.traceRules.length === 1 && defaults.traceMaxDepth === 1 && typeof defaults.trace === "function" && defaults.profile === pd, "the defaults object is unchanged");
console.log("done");
`

// TestGeneratedTSObservationOptions checks the meaning of the observation options of generated
// TypeScript parsers (observeOptionsScript).
func TestGeneratedTSObservationOptions(t *testing.T) {
	t.Parallel()
	g, err := parseTest(`rule main = part part
rule part = [a-z]`)
	if err != nil {
		t.Fatal(err)
	}
	// The mutants of the observation tests change the library through codegentest.TSLibPatchEnv.
	bundle := func(g *grammar.Grammar, o codegen.GenOptions) ([]byte, error) {
		code, err := codegentest.GenerateTSBundle(g, o)
		if err != nil {
			return nil, err
		}
		return codegentest.PatchTSLib(t, code), nil
	}
	out := codegentest.RunTSGrammarScriptBundle(t, g, observeOptionsScript, codegen.GenOptions{Start: "main", Observability: true, Document: true}, bundle)
	if strings.Contains(out, "FAIL") || !strings.HasSuffix(out, "done\n") {
		t.Fatalf("observation options:\n%s", out)
	}
}
