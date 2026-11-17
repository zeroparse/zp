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
// first may leave no result in the memo, so a parse and a recognition evaluate the rule of each
// case of codegentest.StorePolicyCases as often as the policy allows. Generated TypeScript has no streams (stream generation is only
// supported for Go) and no Documents, so neither entry exists to test there.
func TestGeneratedTSStorePolicy(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("runs generated TypeScript")
	}
	node := codegentest.RequireNode(t)
	for _, limits := range []bool{false, true} {
		t.Run(fmt.Sprintf("limits=%v", limits), func(t *testing.T) {
			t.Parallel()
			if bad := tsStorePolicyMismatches(t, node, limits, nil); len(bad) > 0 {
				t.Error(strings.Join(bad, "\n"))
			}
		})
	}
}

// TestGeneratedTSStorePolicyMutantsFail is the negative control of TestGeneratedTSStorePolicy: a
// parser whose firstCall defers a result once more, per lookahead state or per second call, fails
// it, in the dense seen table and in the sparse one.
func TestGeneratedTSStorePolicyMutantsFail(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("runs generated TypeScript")
	}
	node := codegentest.RequireNode(t)
	const dense = "this.markSeen(this.pos, s);"
	const sparse = "this.markSparseSeen(this.pos, s)\n"
	mutants := []struct{ name, old, new, failing string }{
		{"a first call per lookahead state", dense, "this.markSeen(this.pos + (this.silent > 0 ? 4096 : 0), s);", ""},
		{"a second call deferred as well, dense", dense, "this.markSeen(this.pos, s) || this.markSeen(this.pos + 4096, s);", ""},
		{"a second call deferred as well, sparse", sparse, "(this.markSparseSeen(this.pos, s) || this.markSparseSeen(this.pos + 4096, s))\n", ", sparse"},
	}
	for _, m := range mutants {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			bad := tsStorePolicyMismatches(t, node, false, func(code string) string {
				if n := strings.Count(code, m.old); n != 1 {
					t.Fatalf("the parser has %d copies of %q, not one", n, m.old)
				}
				return strings.Replace(code, m.old, m.new, 1)
			})
			if len(bad) == 0 {
				t.Fatal("the test passed with the mutant")
			}
			found := false
			for _, b := range bad {
				found = found || strings.Contains(b, m.failing)
			}
			if !found {
				t.Fatalf("no case %q failed:\n%s", m.failing, strings.Join(bad, "\n"))
			}
			t.Logf("%d cases failed with the mutant, for example:\n%s", len(bad), bad[0])
		})
	}
}

// tsStorePolicyMismatches generates a parser for each case that is not a stream, with limit
// controls if limits, after changing its source with mutate (if not nil), runs them in node, and
// returns a description of each count of evaluations that the policy does not allow.
func tsStorePolicyMismatches(t *testing.T, node string, limits bool, mutate func(string) string) []string {
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
		code, err := codegentest.GenerateTSBundle(mustParse(c.Src), codegen.GenOptions{Start: c.Start, Recognize: true, Observability: true, LimitControls: limits})
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
		fmt.Fprintf(&main, "console.log(evals(%[1]s, %[2]q, (o) => %[1]s.parseWithOptions(%[3]s, o)), evals(%[1]s, %[2]q, (o) => %[1]s.recognizeWithOptions(%[3]s, o)));\n", pkg, c.Rule, in)
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
	var bad []string
	for i, c := range cases {
		parse, _ := c.StorePolicyEvals(true)
		recognize, _ := c.StorePolicyEvals(false)
		got, want := strings.Join(strings.Fields(lines[i]), " "), fmt.Sprintf("%d %d", parse, recognize)
		if got != want {
			bad = append(bad, fmt.Sprintf("%s: %s evaluated %s times (parse, recognition), want %s", c.Name, c.Rule, got, want))
		}
	}
	return bad
}
