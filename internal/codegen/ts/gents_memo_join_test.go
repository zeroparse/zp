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
	"zeroparse.dev/zp/internal/testing/codegentest"
)

// TestGeneratedTSMemoJoin checks that a store in a generated TypeScript parser never narrows the
// calls that the entry of a key answers, as in the engine: a parse and a recognition, without and
// with limit controls, evaluate the rule k of each run of codegentest.MemoJoinCases as often as the
// engine does.
func TestGeneratedTSMemoJoin(t *testing.T) {
	t.Parallel()
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
	main.WriteString("const evals = (m, run) => {\n" +
		"  const profile = new m.Profile();\n" +
		"  run({ profile });\n" +
		"  const r = profile.snapshot().rules.find((r) => r.rule === \"k\");\n" +
		"  return r === undefined ? -1 : r.evals;\n" +
		"};\n")
	type run struct {
		c   codegentest.MemoJoinCase
		run codegentest.MemoJoinRun
	}
	var runs []run
	for _, c := range codegentest.MemoJoinCases() {
		for _, r := range c.Runs {
			i := len(runs)
			runs = append(runs, run{c, r})
			in := strconv.Quote(r.Input)
			// g has no limit controls; l has them, under the default limits.
			var line []string
			for _, pkg := range []string{fmt.Sprintf("g%d", i), fmt.Sprintf("l%d", i)} {
				code, err := codegentest.GenerateTSBundle(mustParse(r.Src), codegen.GenOptions{Start: "s", Recognize: true, Observability: true, LimitControls: pkg[0] == 'l'})
				if err != nil {
					t.Fatalf("%s: %v", c.Name, err)
				}
				write(pkg+".ts", code)
				fmt.Fprintf(&main, "import * as %s from \"./%[1]s.ts\";\n", pkg)
				line = append(line, fmt.Sprintf("evals(%s, (o) => %[1]s.parseWithOptions(%[2]s, o))", pkg, in),
					fmt.Sprintf("evals(%s, (o) => %[1]s.recognizeWithOptions(%[2]s, o))", pkg, in))
			}
			fmt.Fprintf(&main, "console.log(%s);\n", strings.Join(line, ", "))
		}
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
	if len(lines) != len(runs) {
		t.Fatalf("got %d lines, want %d\n%s", len(lines), len(runs), out)
	}
	for i, r := range runs {
		got := strings.Fields(lines[i])
		want := strings.Fields(strings.Repeat(fmt.Sprintf("%d %d ", r.c.Evals, r.c.Recognized), 2))
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("%s (%d units of input): k evaluated %v times (parse, recognition; then with limit controls), want %v", r.c.Name, len(r.run.Input), got, want)
		}
	}
}
