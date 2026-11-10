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

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
)

// TestGeneratedTwinMemo checks the memo of value-free twins in a generated Go parser, as
// TestTwinMemo does for the engine (Evaluations of Rules in spec/runtime/parsing.md): a rule and its
// twin have memo entries of their own, and a twin is memoized exactly when its rule is, even where
// the twin is called from one place only. Parse and ParseAST, without and with limit controls,
// evaluate the body of r, the twin and the rule together, as often as each grammar of gentest says.
// Recognize builds no value that no predicate reads, so r has no twin there: the calls of r and -r
// evaluate one rule with one memo entry.
func TestGeneratedTwinMemo(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testGeneratedTwinMemo)
}

func testGeneratedTwinMemo(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated code")
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	if _, err := os.Stat(goBin); err != nil {
		t.Skip("go command not found")
	}
	cases := []struct {
		name, src        string
		evals, recognize int
	}{
		// Twice the twin and twice the rule: an entry of the twin answers no call of the rule. A
		// recognition evaluates r twice in all.
		{"own entries", gentest.TwinOwnEntriesSrc, 4, 2},
		// Twice the twin at 1, whose second evaluation stores the result that answers its third
		// call, and once the rule at 0.
		{"one place", gentest.TwinOnePlaceSrc, 3, 3},
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gentest.GoMod("gentest")), 0o644); err != nil {
		t.Fatal(err)
	}
	var imports, calls strings.Builder
	in := strconv.Quote("abcy")
	for i, c := range cases {
		// g has no limit controls; l has them, under the default limits.
		for _, pkg := range []string{fmt.Sprintf("g%d", i), fmt.Sprintf("l%d", i)} {
			gentest.GeneratePackage(t, dir, pkg, mustParse(c.src), gen.GenOptions{Start: "main", Types: true, Recognize: true, Observability: true, LimitControls: pkg[0] == 'l'})
			fmt.Fprintf(&imports, "\t%q\n", "gentest/"+pkg)
			for _, op := range []string{"_, _ = %s.Parse(%s, zprt.WithProfile(&p))", "_, _ = %s.ParseAST(%s, zprt.WithProfile(&p))", "_ = %s.Recognize(%s, zprt.WithProfile(&p))"} {
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

// evals sums the body evaluations of r and of its twin, which the profile reports under its name.
func evals(p zprt.Profile) int {
	n := 0
	for _, r := range p.Snapshot().Rules {
		if r.Rule == "r" {
			n += r.Evals
		}
	}
	return n
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
		want := strings.Fields(strings.Repeat(fmt.Sprintf("%d %d %d ", c.evals, c.evals, c.recognize), 2))
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("%s: r evaluated %v times (Parse, ParseAST, Recognize; then with limit controls), want %v", c.name, got, want)
		}
	}
}
