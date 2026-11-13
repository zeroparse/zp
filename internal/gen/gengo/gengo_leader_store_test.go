package gengo_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
)

// TestGeneratedLeaderStore checks that a growth of a leader in a generated Go parser stores its
// result inside a lookahead and outside one, as the engine does (Evaluations of Rules in
// spec/runtime/parsing.md): Parse, ParseValue and Recognize, without and with limit controls,
// evaluate the leader e of gentest.LeaderStoreCases as often as the growths that the memo allows
// take steps.
func TestGeneratedLeaderStore(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testGeneratedLeaderStore)
}

func testGeneratedLeaderStore(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated code")
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	if _, err := os.Stat(goBin); err != nil {
		t.Skip("go command not found")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gentest.GoMod("gentest")), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := gentest.LeaderStoreCases()
	var imports, calls strings.Builder
	for i, c := range cases {
		in := strconv.Quote(c.Input)
		// g has no limit controls; l has them, under the default limits.
		for _, pkg := range []string{fmt.Sprintf("g%d", i), fmt.Sprintf("l%d", i)} {
			gentest.GeneratePackage(t, dir, pkg, mustParse(c.Src), gen.GenOptions{Start: "main", Types: true, Recognize: true, Observability: true, LimitControls: pkg[0] == 'l'})
			fmt.Fprintf(&imports, "\t%q\n", "gentest/"+pkg)
			for _, op := range []string{"_, _ = %s.Parse(%s, zprt.WithProfile(&p))", "_, _ = %s.ParseValue(%s, zprt.WithProfile(&p))", "_ = %s.Recognize(%s, zprt.WithProfile(&p))"} {
				fmt.Fprintf(&calls, "\t{ var p zprt.Profile; "+op+"; fmt.Print(evals(p), \" \") }\n", pkg, in)
			}
		}
		calls.WriteString("\tfmt.Println()\n")
	}
	main := `package main

import (
	"fmt"

	zprt "zeroparse.dev/zp/runtime"
` + imports.String() + `)

func evals(p zprt.Profile) int {
	for _, r := range p.Snapshot().Rules {
		if r.Rule == "e" {
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
		want := strings.Fields(strings.Repeat(fmt.Sprintf("%d ", c.Evals), 6))
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("%s: e evaluated %v times (Parse, ParseValue, Recognize; then with limit controls), want %v", c.Name, got, want)
		}
	}
}
