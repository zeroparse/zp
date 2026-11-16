package gogen_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
	"zeroparse.dev/zp/internal/engine"
)

// The generated TypeScript parsers report the calls of a parse with the events of the engine and the
// generated Go parsers, in the same JSON shape, and add them to profiles of the same shape. The test here
// compares them with the engine as TestEngineObservationsAgreeWithGenerated compares the generated Go
// parsers, on Parse, Recognize and the parse of a Document, in both position units.
//
// TODO(examined): generated TypeScript does not yet report examined ends: its events have Examined 0 and
// its profiles no examined input. Until it does, the comparison leaves out the examined ends (Examined of
// the events; examined, wasted and longestFail of the profiles), and a call that only the TypeScript
// parser makes counts as failing within its first character whenever it fails. The edit entry, whose
// relation (sameCallExamined) compares only examined ends, waits for them too.

// tsKnownDivergences are the cases whose TypeScript calls differ from the engine's, with the reason.
// TODO(left-recursion growth): remove the entry once the TypeScript growth of mutually left-recursive
// leaders takes the steps that the engine and generated Go take; TestTSObservationsAgreeWithEngine
// fails when a listed case no longer differs, so that the entry goes with the fix.
var tsKnownDivergences = map[string]string{
	"mutual leaders": "when a takes its grown seed, b grows again from a jump past the step to b@0 with end 1: the call b@0 reports evals 2, the engine's and generated Go's 3",
}

// tsAgreeConsumer runs the generated TypeScript parsers g<i>.ts on the inputs (base64) in the file of
// its first argument and prints, for each input, unit and entry, one JSON report in the shape of
// agreeReport. Each parse is observed by two trace functions and two profiles, given as arrays with a
// null entry that must be ignored; the same parse, observed again, has the trace filter Filter and the
// depth limit MAXDEPTH. The document entry gives the first trace function and profile, and the trace
// filter "main" with the depth limit 1, as the defaults of the Document, and the others in the call of
// its parse, which add to those defaults and replace the filter and the limit.
const tsAgreeConsumer = `import { readFileSync } from "node:fs";

const inputs = JSON.parse(readFileSync(process.argv[2], "utf8"));
const maxDepth = MAXDEPTH;
const untimed = (profile) => {
  const s = profile.snapshot();
  s.time = 0;
  for (const r of s.rules) { r.time = 0; r.selfTime = 0; }
  return s;
};
const message = (err) => err === null ? "<nil>" : String(err.message);
for (let i = 0; i < inputs.length; i++) {
  const m = await import("./g" + i + ".ts");
  const entries = [
    ["parse", (x, defaults, call) => m.parseWithOptions(x, { ...defaults, ...call }).error, true],
    ["recognize", (x, defaults, call) => m.recognizeWithOptions(x, { ...defaults, ...call }), true],
  ];
  if (typeof m.createDocument === "function") {
    entries.push(["document", (x, defaults, call) => {
      const { document, error } = m.createDocument(x, defaults);
      return error !== null ? error : document.parse(call).error;
    }, false]);
  }
  for (let j = 0; j < inputs[i].length; j++) {
    const x = new Uint8Array(Buffer.from(inputs[i][j], "base64"));
    for (const unit of [m.CodePoints, m.Bytes]) {
      for (const [entry, run, single] of entries) {
        const events = [], second = [], prof = new m.Profile(), prof2 = new m.Profile();
        const f1 = (e) => { events.push(e); }, f2 = (e) => { second.push(e); };
        const err = single
          ? run(x, { unit, trace: [f1, null, f2], profile: [prof, null, prof2] }, {})
          : run(x, { unit, trace: f1, profile: prof }, { trace: [f2], profile: prof2 });
        const filter = ["main", "no such rule"];
        const first = events.find((e) => e.depth === 2);
        if (first !== undefined) filter[0] = first.rule;
        const filtered = [], filteredProf = new m.Profile();
        const f = (e) => { filtered.push(e); };
        if (single) run(x, { unit, trace: f, profile: filteredProf, traceRules: filter, traceMaxDepth: maxDepth }, {});
        else run(x, { unit, trace: f, profile: filteredProf, traceRules: ["main"], traceMaxDepth: 1 }, { traceRules: filter, traceMaxDepth: maxDepth });
        console.log(JSON.stringify({
          Case: i, Input: j, Bytes: unit === m.Bytes, Entry: entry,
          Events: events, Profile: untimed(prof), Second: second, Profile2: untimed(prof2),
          Filter: filter, Filtered: filtered, FilteredProfile: untimed(filteredProf), Err: message(err),
        }));
      }
    }
  }
}
`

// runTSAgreeProbes generates the TypeScript parsers of the cases with observability, the recognizer and
// Document where it can, changed as codegentest.TSLibPatchEnv says, and returns the reports of
// tsAgreeConsumer.
func runTSAgreeProbes(t *testing.T, node string, cases []codegentest.Case) []agreeReport {
	t.Helper()
	dir := t.TempDir()
	write := func(name string, content []byte) {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("package.json", []byte(`{"type": "module"}`+"\n"))
	var inputs [][][]byte
	for i, c := range cases {
		g, err := parseTest(c.Src)
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		code, err := codegentest.GenerateTSBundle(g, codegen.GenOptions{Start: "main", Recognize: true, Observability: true, Document: true})
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		write(fmt.Sprintf("g%d.ts", i), codegentest.PatchTSLib(t, code))
		var in [][]byte
		for _, s := range c.Inputs {
			in = append(in, []byte(s))
		}
		inputs = append(inputs, in)
	}
	data, _ := json.Marshal(inputs)
	write("inputs.json", data)
	write("main.mjs", []byte(strings.ReplaceAll(tsAgreeConsumer, "MAXDEPTH", fmt.Sprint(agreeMaxDepth))))
	cmd := exec.Command(node, "--stack-size=65500", "main.mjs", "inputs.json")
	cmd.Dir = dir
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, codegentest.Clip(stderr.String(), 0))
	}
	var reports []agreeReport
	for _, l := range strings.Split(strings.TrimSuffix(string(out), "\n"), "\n") {
		var r agreeReport
		if err := json.Unmarshal([]byte(l), &r); err != nil {
			t.Fatalf("node: malformed report %q: %v", codegentest.Clip(l, 0), err)
		}
		reports = append(reports, r)
	}
	return reports
}

// withoutExamined returns a copy of evs with the examined ends left out (see TODO(examined)).
func withoutExamined(evs []core.TraceEvent) []core.TraceEvent {
	out := make([]core.TraceEvent, len(evs))
	for i, e := range evs {
		e.Examined = 0
		out[i] = e
	}
	return out
}

// profileWithoutExamined returns a copy of s with the counts of the examined input left out (see
// TODO(examined)).
func profileWithoutExamined(s core.ProfileSnapshot) core.ProfileSnapshot {
	s.Examined = 0
	rules := make([]core.RuleProfile, len(s.Rules))
	for i, r := range s.Rules {
		r.Examined, r.Wasted, r.LongestFail, r.LongestFailAt = 0, 0, 0, core.CallSite{}
		rules[i] = r
	}
	s.Rules = rules
	return s
}

// tsAgreementMismatches checks each report of the TypeScript parsers against the same parse on each
// engine backend with callsAgree and profilesAgree, and the options of both producers with
// optionsHonored, and returns the differences and what it compared, by entry.
func tsAgreementMismatches(t *testing.T, cases []codegentest.Case, reports []agreeReport) (map[string][]string, map[string]*agreementStats) {
	t.Helper()
	diffs, stats := map[string][]string{}, map[string]*agreementStats{}
	progs := map[int]*engine.Program{}
	for _, r := range reports {
		c := cases[r.Case]
		in := c.Inputs[r.Input]
		prog := progs[r.Case]
		if prog == nil {
			prog = mustCompile(t, c.Src)
			progs[r.Case] = prog
		}
		unit := core.CodePoints
		if r.Bytes {
			unit = core.Bytes
		}
		st := stats[r.Entry]
		if st == nil {
			st = &agreementStats{}
			stats[r.Entry] = st
		}
		st.reports++
		where := fmt.Sprintf("%s %q bytes=%v %s", c.Name, in, r.Bytes, r.Entry)
		if msg := optionsHonored("typescript", r.Events, r.Profile, r.Second, r.Profile2, r.Filter, r.Filtered, r.FilteredProfile); msg != "" {
			diffs[r.Entry] = append(diffs[r.Entry], where+": "+msg)
		}
		for _, b := range []engine.Backend{engine.Closure, engine.Bytecode, engine.BytecodeIterative} {
			events, snap, err := engineObservation(t, prog, r, in, b, nil, 0)
			where := fmt.Sprintf("%s %v", where, b)
			if (err == nil) != (r.Err == "<nil>") {
				diffs[r.Entry] = append(diffs[r.Entry], fmt.Sprintf("%s: engine error %v, typescript %s", where, err, r.Err))
				continue
			}
			filtered, _, _ := engineObservation(t, prog, r, in, b, r.Filter, agreeMaxDepth)
			st.filtered += len(filtered)
			if len(filtered) > 0 && len(filtered) < len(events) {
				st.partial++
			}
			aside, msg := callsAgreeOn(events, r.Events, in, unit, false)
			if msg == "" {
				aside.a, aside.b = withoutExamined(aside.a), withoutExamined(aside.b)
				msg = profilesAgree(profileWithoutExamined(snap), profileWithoutExamined(r.Profile), aside)
			}
			if msg == "" {
				if _, m := callsAgreeOn(filtered, r.Filtered, in, unit, false); m != "" {
					msg = "filtered: " + m
				}
			}
			if msg != "" {
				if _, known := tsKnownDivergences[c.Name]; known {
					if st.known == nil {
						st.known = map[string]int{}
					}
					st.known[c.Name]++
					continue
				}
				diffs[r.Entry] = append(diffs[r.Entry], where+": "+msg)
				continue
			}
			st.compared += aside.compared
			st.paired += aside.paired
			st.unpaired += aside.unpaired
			for _, e := range aside.a {
				if e.Kind == core.TraceEnter {
					st.engineOnly++
				}
			}
			for _, e := range aside.b {
				if e.Kind == core.TraceEnter {
					st.generatedOnly++
				}
			}
		}
	}
	return diffs, stats
}

// TestTSObservationsAgreeWithEngine checks the events and profiles of the generated TypeScript parsers
// against those of every engine backend (callsAgree, profilesAgree) on Parse, Recognize and the parse of
// a Document, in both units, and that they honor several trace functions and profiles, given as arrays
// or as the defaults of a Document and its parse, and the trace filter and depth limit, given in a
// parse or replacing the defaults of a Document (optionsHonored).
func TestTSObservationsAgreeWithEngine(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("runs generated TypeScript consumers")
	}
	node := codegentest.RequireNode(t)
	codegentest.NeedsBudgetControls(t)
	cases := examinedCases(t)
	diffs, stats := tsAgreementMismatches(t, cases, runTSAgreeProbes(t, node, cases))
	for name := range tsKnownDivergences {
		present, known := false, 0
		for _, c := range cases {
			present = present || c.Name == name
		}
		for _, st := range stats {
			known += st.known[name]
		}
		if present && known == 0 {
			t.Errorf("known divergence %q no longer differs: remove it from tsKnownDivergences", name)
		}
	}
	for _, entry := range []string{"parse", "recognize", "document"} {
		st := stats[entry]
		if st == nil || st.compared == 0 {
			t.Errorf("no event of %s compared", entry)
			st = &agreementStats{}
		}
		t.Logf("%s: %d reports, %d events compared; %d skipped events paired, %d unpaired; %d calls of the engine only, %d of the TypeScript parser only",
			entry, st.reports, st.compared, st.paired, st.unpaired, st.engineOnly, st.generatedOnly)
		t.Logf("%s: %d filtered engine events; %d parses filtered in part", entry, st.filtered, st.partial)
		if st.partial == 0 {
			t.Errorf("no parse of %s was filtered in part", entry)
		}
		for name, n := range st.known {
			t.Logf("%s: %d observations of the known divergence %q", entry, n, name)
		}
		for i, msg := range diffs[entry] {
			if i == 10 {
				t.Errorf("%d more differences of %s", len(diffs[entry])-i, entry)
				break
			}
			t.Error(msg)
		}
	}
}
