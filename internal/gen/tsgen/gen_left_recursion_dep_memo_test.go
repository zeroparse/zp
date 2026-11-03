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

// depMemoTSCases returns the grammars of gentest.DependencyMemoCases for seeds below seeds that
// generate TypeScript parsers.
func depMemoTSCases(t *testing.T, seeds int) []gentest.Case {
	t.Helper()
	var cases []gentest.Case
	for _, c := range gentest.DependencyMemoCases(seeds) {
		g, err := parseTest(c.Src)
		if err != nil {
			continue
		}
		if _, err := gentest.GenerateTSBundle(g, gen.GenOptions{Start: "main", Recognize: true}); err != nil {
			continue
		}
		cases = append(cases, c)
	}
	if len(cases) < seeds*2 {
		t.Fatalf("only %d grammars of %d seeds generate", len(cases), seeds)
	}
	return cases
}

// TestGeneratedTSLeftRecursionDependencyMemo checks that generated TypeScript parsers give the
// engine's results for grammars whose leaders grow inside the growth of other leaders of their
// cycle at one position, where the dependency memo reuses results by the seeds they read and the
// lookaheads they observed; with source information, the values also tell apart step results that
// differ only in the rules that produced them.
func TestGeneratedTSLeftRecursionDependencyMemo(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("runs generated TypeScript")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found")
	}
	gentest.TSParsersCorpus(t, node, depMemoTSCases(t, 60), gen.GenOptions{})
	gentest.TSParsersCorpus(t, node, depMemoTSCases(t, 20), gen.GenOptions{SourceInfo: true})
}

// TestGeneratedTSLeftRecursionDependencyMemoLinear checks that generated TypeScript parsers of
// gentest.DependencyMemoShapes evaluate a number of calls linear in the length of the input:
// doubling the input at most a little more than doubles the evaluations. Generated with budget
// controls, they parse an input of length 10,000 within the default budget, which grows linearly
// with the input and counts work that evaluations do not show, such as the scan of the jumps
// recorded for a seed.
func TestGeneratedTSLeftRecursionDependencyMemoLinear(t *testing.T) {
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
	for i, s := range gentest.DependencyMemoShapes {
		code, err := gentest.GenerateTSBundle(mustParse("@start\nrule main = "+s.Rules), gen.GenOptions{Start: "main", Observability: true})
		if err != nil {
			t.Fatalf("%s: %v", s.Name, err)
		}
		write(fmt.Sprintf("g%d.ts", i), code)
		fmt.Fprintf(&main, "import * as g%d from \"./g%[1]d.ts\";\n", i)
		code, err = gentest.GenerateTSBundle(mustParse("@start\nrule main = "+s.Rules), gen.GenOptions{Start: "main", LimitControls: true})
		if err != nil {
			t.Fatalf("%s: %v", s.Name, err)
		}
		write(fmt.Sprintf("c%d.ts", i), code)
		fmt.Fprintf(&main, "import * as c%d from \"./c%[1]d.ts\";\n", i)
	}
	main.WriteString("const evals = (m, input) => {\n" +
		"  const profile = new m.Profile();\n" +
		"  const r = m.parseWithOptions(input, { profile });\n" +
		"  if (r.error !== null) throw r.error;\n" +
		"  return profile.snapshot().evals;\n" +
		"};\n" +
		"const withinBudget = (m, input) => m.parseWithOptions(input, {}).error === null;\n")
	for i, s := range gentest.DependencyMemoShapes {
		fmt.Fprintf(&main, "console.log(evals(g%[1]d, %[2]s), evals(g%[1]d, %[3]s), withinBudget(c%[1]d, %[4]s));\n", i, strconv.Quote(s.Input(80)), strconv.Quote(s.Input(160)), strconv.Quote(s.Input(10000)))
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
	if len(lines) != len(gentest.DependencyMemoShapes) {
		t.Fatalf("got %d lines, want %d\n%s", len(lines), len(gentest.DependencyMemoShapes), out)
	}
	for i, s := range gentest.DependencyMemoShapes {
		var small, large int
		var long bool
		if _, err := fmt.Sscan(lines[i], &small, &large, &long); err != nil {
			t.Fatalf("%s: %q: %v", s.Name, lines[i], err)
		}
		if !long {
			t.Errorf("%s: no parse at 10000 within the default budget", s.Name)
		}
		if small == 0 || large > small*5/2 {
			t.Errorf("%s: %d evaluations at 80, %d at 160", s.Name, small, large)
		}
	}
}
