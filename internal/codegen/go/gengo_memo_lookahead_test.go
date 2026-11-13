package gogen_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
)

// TestGeneratedMemoLookaheadState checks that generated Go parsers bound the evaluations of calls
// that alternate between lookahead states, as the engine does: Parse, ParseValue and Recognize,
// without and with limit controls, evaluate the rule a of codegentest.MemoLookaheadCases as often as
// the engine does, whether or not its result has recovered errors.
func TestGeneratedMemoLookaheadState(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, testGeneratedMemoLookaheadState)
}

func testGeneratedMemoLookaheadState(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated code")
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	if _, err := os.Stat(goBin); err != nil {
		t.Skip("go command not found")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(codegentest.GoMod("codegentest")), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := codegentest.MemoLookaheadCases()
	var imports, calls strings.Builder
	for i, c := range cases {
		in := strconv.Quote(c.Input)
		// g has no limit controls; l has them, under the default limits.
		for _, pkg := range []string{fmt.Sprintf("g%d", i), fmt.Sprintf("l%d", i)} {
			codegentest.GeneratePackage(t, dir, pkg, mustParse(c.Src), codegen.GenOptions{Start: "s", Types: true, Recognize: true, Observability: true, LimitControls: pkg[0] == 'l'})
			fmt.Fprintf(&imports, "\t%q\n", "codegentest/"+pkg)
			for _, op := range []string{"_, _ = %s.Parse(%s, zprt.WithProfile(&p))", "_, _ = %s.ParseValue(%s, zprt.WithProfile(&p))", "_ = %s.Recognize(%s, zprt.WithProfile(&p))"} {
				fmt.Fprintf(&calls, "\t{ var p zprt.Profile; "+op+"; fmt.Print(evals(p), \" \") }\n", pkg, in)
			}
		}
		calls.WriteString("\tfmt.Println()\n")
	}
	main := `package main

import (
	"fmt"

	zprt "zeroparse.dev/zp/codegen/go/lib"
` + imports.String() + `)

func evals(p zprt.Profile) int {
	for _, r := range p.Snapshot().Rules {
		if r.Rule == "a" {
			return r.Evals
		}
	}
	return -1
}

func main() {
` + calls.String() + `}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(goBin, "run", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod -trimpath", "GOTOOLCHAIN=local")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run: %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(lines) != len(cases) {
		t.Fatalf("got %d lines, want %d\n%s", len(lines), len(cases), out)
	}
	for i, c := range cases {
		got := strings.Fields(lines[i])
		want := strings.Fields(strings.Repeat(fmt.Sprintf("%d %d %d ", c.Evals, c.Evals, c.Recognized), 2))
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("%s: a evaluated %v times (Parse, ParseValue, Recognize; then with limit controls), want %v", c.Name, got, want)
		}
	}
}
