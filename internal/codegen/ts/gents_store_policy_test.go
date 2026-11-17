package tsgen_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

// TestGeneratedTSStorePolicy checks the store policy of "Evaluations of Rules" in
// spec/runtime/parsing.md in generated TypeScript parsers, as the engine's TestMemoStorePolicy does:
// of the evaluations of a memoized rule at a position outside every context of growths, only the
// first may leave no result in the memo. So a parse, a recognition and the first parse of a Document,
// for each case of codegentest.StorePolicyCases, evaluate the rule at least as often as a memo
// storing every first call and at most as often as one deferring each. Generated TypeScript has no
// streams (stream generation is only supported for Go), so the stream cases are left out.
func TestGeneratedTSStorePolicy(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("runs generated TypeScript")
	}
	node := codegentest.RequireNode(t)
	for _, limits := range []bool{false, true} {
		t.Run(fmt.Sprintf("limits=%v", limits), func(t *testing.T) {
			t.Parallel()
			if bad := tsStorePolicyEnvelope(tsStorePolicyCounts(t, node, limits, nil)); len(bad) > 0 {
				t.Error(strings.Join(bad, "\n"))
			}
		})
	}
}

// TestGeneratedTSStorePolicyReferenceCounts pins what the generated TypeScript library does within
// the policy, which the policy does not require and TestGeneratedTSStorePolicy does not: a parse
// defers the first call of a rule at a position until repeated calls exceed a sixteenth of its calls,
// so it evaluates the rule exactly as often as the model of codegentest.StorePolicyCase says; a
// Document parse memoizes every first call. A change of that choice that the policy allows updates
// this test.
func TestGeneratedTSStorePolicyReferenceCounts(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("runs generated TypeScript")
	}
	node := codegentest.RequireNode(t)
	var bad []string
	for _, n := range tsStorePolicyCounts(t, node, false, nil) {
		deferred, stored := n.c.StorePolicyEvals(true)
		recDeferred, _ := n.c.StorePolicyEvals(false)
		for _, x := range []struct {
			entry     string
			got, want int
		}{{"a Document", n.doc, stored}, {"a parse", n.parse, deferred}, {"a recognition", n.recog, recDeferred}} {
			if x.got != x.want {
				bad = append(bad, fmt.Sprintf("%s: %s evaluated %d times in %s, the library %d", n.c.Name, n.c.Rule, x.got, x.entry, x.want))
			}
		}
	}
	if len(bad) > 0 {
		t.Error(strings.Join(bad, "\n"))
	}
}

// TestGeneratedTSStorePolicyMutantsFail is the negative control of TestGeneratedTSStorePolicy: a
// parser that defers a result that the policy requires to be stored fails it, on the case that has
// the calls where the mutant differs, in the dense seen table and in the sparse one and in a
// Document.
func TestGeneratedTSStorePolicyMutantsFail(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("runs generated TypeScript")
	}
	node := codegentest.RequireNode(t)
	const dense = "this.markSeen(this.pos, s);"
	const sparse = "this.markSparseSeen(this.pos, s)\n"
	const document = "      if (!r.docMemo) {\n        const v = this.callUnmemoized(r, min);"
	mutants := []struct{ name, old, new, failing string }{
		{"a first call per lookahead state", dense, "this.markSeen(this.pos + (this.silent > 0 ? 4096 : 0), s);", "inside first"},
		{"a second call deferred as well, dense", dense, "this.markSeen(this.pos, s) || this.markSeen(this.pos + 4096, s);", "outside only"},
		{"a second call deferred as well, sparse", sparse, "(this.markSparseSeen(this.pos, s) || this.markSparseSeen(this.pos + 4096, s))\n", "inside first, sparse"},
		{"a Document stores no result computed inside a lookahead", document, "      if (!r.docMemo || this.silent > 0) {\n        const v = this.callUnmemoized(r, min);", "alternating"},
	}
	for _, m := range mutants {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			bad := tsStorePolicyEnvelope(tsStorePolicyCounts(t, node, false, func(code string) string {
				if n := strings.Count(code, m.old); n != 1 {
					t.Fatalf("the parser has %d copies of %q, not one", n, m.old)
				}
				return strings.Replace(code, m.old, m.new, 1)
			}))
			if len(bad) == 0 {
				t.Fatal("the test passed with the mutant")
			}
			found := false
			for _, b := range bad {
				found = found || strings.HasPrefix(b, m.failing+": ")
			}
			if !found {
				t.Fatalf("no case %q failed:\n%s", m.failing, strings.Join(bad, "\n"))
			}
			t.Logf("%d cases failed with the mutant, for example:\n%s", len(bad), bad[0])
		})
	}
}

// tsStoreCounts are the evaluations of a case's rule in the first parse of a Document, a parse and a
// recognition.
type tsStoreCounts struct {
	c                 codegentest.StorePolicyCase
	doc, parse, recog int
}

// tsStorePolicyEnvelope returns the counts that are outside what the policy allows: not fewer than a
// memo storing every first call leaves, not more than one deferring each leaves.
func tsStorePolicyEnvelope(counts []tsStoreCounts) []string {
	var bad []string
	for _, n := range counts {
		deferred, stored := n.c.StorePolicyEvals(true)
		recDeferred, recStored := n.c.StorePolicyEvals(false)
		for _, x := range []struct {
			entry       string
			got, lo, hi int
		}{{"a Document", n.doc, stored, deferred}, {"a parse", n.parse, stored, deferred}, {"a recognition", n.recog, recStored, recDeferred}} {
			if x.got < x.lo || x.got > x.hi {
				bad = append(bad, fmt.Sprintf("%s: %s evaluated %d times in %s, want %d to %d", n.c.Name, n.c.Rule, x.got, x.entry, x.lo, x.hi))
			}
		}
	}
	return bad
}

// tsStorePolicyCounts generates a parser for each case that is not a stream, with limit controls if
// limits, after changing its source with mutate (if not nil), runs them in node, and returns the
// evaluations of the case's rule.
func tsStorePolicyCounts(t *testing.T, node string, limits bool, mutate func(string) string) []tsStoreCounts {
	t.Helper()
	dir := t.TempDir()
	write := func(name string, content []byte) {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("package.json", []byte(`{"type":"module"}`))
	var main strings.Builder
	main.WriteString("const evals = (m, rule, run) => {\n" +
		"  const profile = new m.Profile();\n" +
		"  run({ profile });\n" +
		"  const r = profile.snapshot().rules.find((r) => r.rule === rule);\n" +
		"  return r === undefined ? -1 : r.evals;\n" +
		"};\n")
	var cases []codegentest.StorePolicyCase
	for _, c := range codegentest.StorePolicyCases() {
		if c.Stream {
			continue
		}
		i := len(cases)
		cases = append(cases, c)
		code, err := codegentest.GenerateTSBundle(mustParse(c.Src), codegen.GenOptions{Start: c.Start, Recognize: true, Observability: true, LimitControls: limits, Document: true})
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		if mutate != nil {
			code = []byte(mutate(string(code)))
		}
		pkg := fmt.Sprintf("c%d", i)
		write(pkg+".ts", code)
		in := strconv.Quote(c.Input)
		fmt.Fprintf(&main, "import * as %s from \"./%[1]s.ts\";\n", pkg)
		fmt.Fprintf(&main, "console.log(evals(%[1]s, %[2]q, (o) => %[1]s.createDocument(%[3]s).document.parse(o)), evals(%[1]s, %[2]q, (o) => %[1]s.parseWithOptions(%[3]s, o)), evals(%[1]s, %[2]q, (o) => %[1]s.recognizeWithOptions(%[3]s, o)));\n", pkg, c.Rule, in)
	}
	write("main.mjs", []byte(main.String()))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
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
	counts := make([]tsStoreCounts, len(cases))
	for i, c := range cases {
		n := tsStoreCounts{c: c}
		if _, err := fmt.Sscan(lines[i], &n.doc, &n.parse, &n.recog); err != nil {
			t.Fatalf("%s: %v in %q", c.Name, err, lines[i])
		}
		counts[i] = n
	}
	return counts
}
