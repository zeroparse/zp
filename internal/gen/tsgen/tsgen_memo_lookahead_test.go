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

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
)

// TestGeneratedTSMemoLookaheadState checks that generated TypeScript parsers bound the evaluations
// of calls that alternate between lookahead states, as the engine does: a parse and a recognition,
// without and with limit controls, evaluate the rule a of gentest.MemoLookaheadCases as often as
// the engine does, whether or not its result has recovered errors.
func TestGeneratedTSMemoLookaheadState(t *testing.T) {
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
		"  const r = profile.snapshot().rules.find((r) => r.rule === \"a\");\n" +
		"  return r === undefined ? -1 : r.evals;\n" +
		"};\n")
	cases := gentest.MemoLookaheadCases()
	for i, c := range cases {
		in := strconv.Quote(c.Input)
		// g has no limit controls; l has them, under the default limits.
		var line []string
		for _, pkg := range []string{fmt.Sprintf("g%d", i), fmt.Sprintf("l%d", i)} {
			code, err := gentest.GenerateTSBundle(mustParse(c.Src), gen.GenOptions{Start: "s", Recognize: true, Observability: true, LimitControls: pkg[0] == 'l'})
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
	for i, c := range cases {
		got := strings.Fields(lines[i])
		want := strings.Fields(strings.Repeat(fmt.Sprintf("%d %d ", c.Evals, c.Recognized), 2))
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("%s: a evaluated %v times (parse, recognition; then with limit controls), want %v", c.Name, got, want)
		}
	}
}
