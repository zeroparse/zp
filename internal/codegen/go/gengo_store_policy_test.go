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

// TestGeneratedStorePolicy checks the store policy of "Evaluations of Rules" in
// spec/runtime/parsing.md in generated Go parsers, as the engine's TestMemoStorePolicy does: of the
// evaluations of a memoized rule at a position outside every context of growths, only the first may
// leave no result in the memo, so Parse, ParseValue and Recognize evaluate the rule of each case of
// codegentest.StorePolicyCases as often as the policy allows.
func TestGeneratedStorePolicy(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, func(t *testing.T) {
		if bad := generatedStorePolicyMismatches(t, codegentest.LibCopy()); len(bad) > 0 {
			t.Error(strings.Join(bad, "\n"))
		}
	})
}

// TestGeneratedStorePolicyMutantsFail is the negative control of TestGeneratedStorePolicy: a library
// whose FirstCall defers a result once more, per lookahead state or per second call, fails it, in
// the dense seen table and in the sparse one.
func TestGeneratedStorePolicyMutantsFail(t *testing.T) {
	t.Parallel()
	const dense = "\t\tfirst = t.markSeen(p.Pos, r.Seen)\n"
	const sparse = "\t\tfirst = markSeenPage(page, pos)\n"
	mutants := []struct{ name, old, new, failing string }{
		{"a first call per lookahead state", dense,
			"\t\tpp := p.Pos\n\t\tif p.Silent > 0 {\n\t\t\tpp += 1 << 12\n\t\t}\n\t\tfirst = t.markSeen(pp, r.Seen)\n", ""},
		{"a second call deferred as well, dense", dense,
			"\t\tfirst = t.markSeen(p.Pos, r.Seen) || t.markSeen(p.Pos+1<<12, r.Seen)\n", ""},
		{"a second call deferred as well, sparse", sparse,
			"\t\tfirst = markSeenPage(page, pos) || t.markSparseSeen(pos+1<<12, r.Seen)\n", ", sparse"},
	}
	for _, m := range mutants {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			codegentest.EachMode(t, func(t *testing.T) {
				lib := codegentest.LibCopyWith(t, func(name, src string) string {
					if name != "lib.go" {
						return src
					}
					if n := strings.Count(src, m.old); n != 1 {
						t.Fatalf("lib.go has %d copies of %q, not one", n, m.old)
					}
					return strings.Replace(src, m.old, m.new, 1)
				}, nil)
				bad := generatedStorePolicyMismatches(t, lib)
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
		})
	}
}

// generatedStorePolicyMismatches builds a package of lib's library for each case that is not a
// stream (generated Go has no stream in these parsers: see the engine's test), runs it, and returns
// a description of each count of evaluations that the policy does not allow.
func generatedStorePolicyMismatches(t *testing.T, lib string) []string {
	t.Helper()
	if testing.Short() {
		t.Skip("builds generated code")
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	if _, err := os.Stat(goBin); err != nil {
		t.Skip("go command not found")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(codegentest.GoModAt("codegentest", lib)), 0o644); err != nil {
		t.Fatal(err)
	}
	var cases []codegentest.StorePolicyCase
	var imports, calls strings.Builder
	for _, c := range codegentest.StorePolicyCases() {
		if c.Stream {
			continue
		}
		i := len(cases)
		cases = append(cases, c)
		pkg := fmt.Sprintf("c%d", i)
		codegentest.GeneratePackage(t, dir, pkg, mustParse(c.Src), codegen.GenOptions{Start: c.Start, Types: true, Recognize: true, Observability: true})
		fmt.Fprintf(&imports, "\t%q\n", "codegentest/"+pkg)
		in := strconv.Quote(c.Input)
		for _, op := range []string{"_, _ = %s.Parse(%s, zprt.WithProfile(&p))", "_, _ = %s.ParseValue(%s, zprt.WithProfile(&p))", "_ = %s.Recognize(%s, zprt.WithProfile(&p))"} {
			fmt.Fprintf(&calls, "\t{ var p zpcore.Profile; "+op+"; fmt.Print(evals(p, %q), \" \") }\n", pkg, in, c.Rule)
		}
		calls.WriteString("\tfmt.Println()\n")
	}
	main := `package main

import (
	"fmt"

	zprt "zeroparse.dev/zp/codegen/go/lib"
	zpcore "zeroparse.dev/zp/core"
` + imports.String() + `)

func evals(p zpcore.Profile, rule string) int {
	for _, r := range p.Snapshot().Rules {
		if r.Rule == rule {
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
	var bad []string
	for i, c := range cases {
		parse, _ := c.StorePolicyEvals(true)
		recognize, _ := c.StorePolicyEvals(false)
		got := strings.Join(strings.Fields(lines[i]), " ")
		want := fmt.Sprintf("%d %d %d", parse, parse, recognize)
		if got != want {
			bad = append(bad, fmt.Sprintf("%s: %s evaluated %s times (Parse, ParseValue, Recognize), want %s", c.Name, c.Rule, got, want))
		}
	}
	return bad
}
