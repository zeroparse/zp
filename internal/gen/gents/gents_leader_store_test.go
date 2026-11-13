package gents_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
)

// TestGeneratedTSLeaderStore checks that a growth of a leader in a generated TypeScript parser
// stores its result inside a lookahead and outside one, as the engine and the generated Go parsers
// do (Evaluations of Rules in spec/runtime/parsing.md): in a parse and a recognition, without and
// with limit controls, the calls of the leader e of gentest.LeaderStoreCases at 0 from main grow
// e or are answered by the memo as a memo that stores every growth answers them, which the memo
// flag of each call in the trace shows. The store of the TypeScript runtime is printed from the
// Core IR of the call protocol, so a change to the store there must fail this test.
func TestGeneratedTSLeaderStore(t *testing.T) {
	t.Parallel()
	cases := gentest.LeaderStoreCases()
	var tc []tsRunCase
	for _, c := range cases {
		tc = append(tc, tsRunCase{c.Name, c.Src, c.Input})
	}
	const calls = `(m, run) => {
  const stack = [], out = [];
  run({ trace: (e) => {
    if (e.kind === "enter") {
      stack.push({ rule: e.rule, pos: e.pos, fromMain: stack.length > 0 && stack[stack.length - 1].rule === "main" });
    } else if (e.kind === "exit") {
      const f = stack.pop();
      if (f.rule === "e" && f.pos === 0 && f.fromMain) out.push(e.memo ? "memo" : "grow");
    }
  } });
  return out.join(" ");
}`
	got := runTS(t, tc, calls)
	for i, c := range cases {
		for j, form := range tsRunForms {
			if got[i][j] != c.Calls {
				t.Errorf("%s, %s: calls of e at 0 from main were %q, want %q", c.Name, form, got[i][j], c.Calls)
			}
		}
	}
}

// TestGeneratedTSTwinMemo checks the memo of value-free twins in a generated TypeScript parser, as
// TestGeneratedTwinMemo does for Go: a rule and its twin have memo entries of their own, and a twin
// is memoized exactly when its rule is, even where the twin is called from one place only. A
// recognition builds values only for the rules that a call inside a capture that a predicate can
// read reaches, which r is not, so r has no twin there.
func TestGeneratedTSTwinMemo(t *testing.T) {
	t.Parallel()
	cases := []struct {
		tsRunCase
		evals, recognized int
	}{
		{tsRunCase{"own entries", gentest.TwinOwnEntriesSrc, "abcy"}, 4, 2},
		{tsRunCase{"one place", gentest.TwinOnePlaceSrc, "abcy"}, 3, 3},
		{tsRunCase{"scoped capture", gentest.TwinScopedCaptureSrc, "aa5"}, 4, 2},
		{tsRunCase{"lambda parameter", gentest.TwinLambdaParamSrc, "aa5"}, 4, 2},
	}
	var tc []tsRunCase
	for _, c := range cases {
		tc = append(tc, c.tsRunCase)
	}
	const evals = `(m, run) => {
  const profile = new m.Profile();
  run({ profile });
  return String(profile.snapshot().rules.filter((r) => r.rule === "r").reduce((n, r) => n + r.evals, 0));
}`
	got := runTS(t, tc, evals)
	for i, c := range cases {
		for j, form := range tsRunForms {
			want := c.evals
			if strings.Contains(form, "recognition") {
				want = c.recognized
			}
			if got[i][j] != strconv.Itoa(want) {
				t.Errorf("%s, %s: r evaluated %s times, want %d", c.name, form, got[i][j], want)
			}
		}
	}
}

// tsRunCase is a grammar with start rule main and an input.
type tsRunCase struct {
	name, src, input string
}

// tsRunForms names the runs that runTS makes of each case, in order.
var tsRunForms = []string{"parse", "recognition", "parse with limit controls", "recognition with limit controls"}

// runTS generates each case without and with limit controls, and returns for each case the
// strings that probe, a JavaScript function of the generated module and a function that runs it
// with parse options, returns for the runs of tsRunForms.
func runTS(t *testing.T, cases []tsRunCase, probe string) [][]string {
	t.Helper()
	if testing.Short() {
		t.Skip("runs generated TypeScript")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found")
	}
	dir := t.TempDir()
	write := func(name string, content []byte) {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("package.json", []byte(`{"type":"module"}`))
	var main strings.Builder
	fmt.Fprintf(&main, "const probe = %s;\n", probe)
	for i, c := range cases {
		in := strconv.Quote(c.input)
		// g has no limit controls; l has them, under the default limits.
		var runs []string
		for _, pkg := range []string{fmt.Sprintf("g%d", i), fmt.Sprintf("l%d", i)} {
			code, err := gentest.GenerateTSBundle(mustParse(c.src), gen.GenOptions{Start: "main", Recognize: true, Observability: true, LimitControls: pkg[0] == 'l'})
			if err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
			write(pkg+".ts", code)
			fmt.Fprintf(&main, "import * as %s from \"./%[1]s.ts\";\n", pkg)
			runs = append(runs, fmt.Sprintf("probe(%s, (o) => %[1]s.parseWithOptions(%[2]s, o))", pkg, in),
				fmt.Sprintf("probe(%s, (o) => %[1]s.recognizeWithOptions(%[2]s, o))", pkg, in))
		}
		fmt.Fprintf(&main, "console.log(JSON.stringify([%s]));\n", strings.Join(runs, ", "))
	}
	write("main.mjs", []byte(main.String()))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, "main.mjs")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(lines) != len(cases) {
		t.Fatalf("got %d lines, want %d\n%s", len(lines), len(cases), out)
	}
	got := make([][]string, len(cases))
	for i, line := range lines {
		if err := json.Unmarshal([]byte(line), &got[i]); err != nil || len(got[i]) != len(tsRunForms) {
			t.Fatalf("%s: unexpected output %q: %v", cases[i].name, line, err)
		}
	}
	return got
}

// TestGeneratedTSLambdaParameter checks, in a generated TypeScript parser, that a recognition in
// which a capture named x records nothing, since the only x that a predicate names is a lambda
// parameter, has the outcome of a parse on inputs where the lambda runs over a list that is not
// empty, as TestRecognizeLambdaParameter does for the engine.
func TestGeneratedTSLambdaParameter(t *testing.T) {
	t.Parallel()
	var cases []tsRunCase
	for _, in := range []string{"aaz6", "aazz6", "aa6", "aa5"} {
		cases = append(cases, tsRunCase{in, gentest.TwinLambdaParamSrc, in})
	}
	// A parse returns a result with an error field; a recognition returns the error or null.
	const outcome = `(m, run) => {
  const r = run({});
  return (r === null || r.error === null) ? "ok" : "error";
}`
	got := runTS(t, cases, outcome)
	for i, c := range cases {
		want := "error"
		if strings.HasSuffix(c.input, "z6") {
			want = "ok"
		}
		for j, form := range tsRunForms {
			if got[i][j] != want {
				t.Errorf("%q, %s: %s, want %s", c.input, form, got[i][j], want)
			}
		}
	}
}
