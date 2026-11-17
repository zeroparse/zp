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
// A stream stores every first call (FirstCall returns at once for a stream), so its count is held between
// the count when every first call stores and the count when it defers; a Document parse memoizes every
// call, so its first parse evaluates the rule exactly as often as when every first call stores.
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
	const noStream = "\tif p.Stream != nil {\n\t\treturn false // streams memoize on the first call, as the engine does\n\t}\n"
	const document = "\tv, ok := p.callMemo(r, min)\n\treturn v, ok\n}\n\n// callUnmemoized"
	type edit struct{ old, new string }
	mutants := []struct {
		name    string
		edits   []edit
		failing string
	}{
		{"a first call per lookahead state", []edit{{dense,
			"\t\tpp := p.Pos\n\t\tif p.Silent > 0 {\n\t\t\tpp += 1 << 12\n\t\t}\n\t\tfirst = t.markSeen(pp, r.Seen)\n"}}, ""},
		{"a second call deferred as well, dense", []edit{{dense,
			"\t\tfirst = t.markSeen(p.Pos, r.Seen) || t.markSeen(p.Pos+1<<12, r.Seen)\n"}}, "alternating"},
		{"a second call deferred as well, sparse", []edit{{sparse,
			"\t\tfirst = markSeenPage(page, pos) || t.markSparseSeen(pos+1<<12, r.Seen)\n"}}, ", sparse"},
		{"a stream defers a first call and a second", []edit{{noStream, ""}, {dense,
			"\t\tfirst = t.markSeen(p.Pos, r.Seen) || t.markSeen(p.Pos+1<<12, r.Seen)\n"}}, ", stream"},
		{"a Document stores no result computed inside a lookahead", []edit{{document,
			"\tif p.Silent > 0 {\n\t\tv, ok := p.callUnmemoized(r, min)\n\t\treturn v, ok\n\t}\n" + document}}, "alternating"},
	}
	for _, m := range mutants {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			codegentest.EachMode(t, func(t *testing.T) {
				lib := codegentest.LibCopyWith(t, func(name, src string) string {
					for _, e := range m.edits {
						if n := strings.Count(src, e.old); name != "lib.go" && name != "call_exec.go" || n == 0 {
							continue
						} else if n != 1 {
							t.Fatalf("%s has %d copies of %q, not one", name, n, e.old)
						}
						src = strings.Replace(src, e.old, e.new, 1)
					}
					return src
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

// generatedStorePolicyMismatches builds a parser of lib's library for each case (a stream parser for
// the stream cases, a Document too for the others), runs it, and returns a description of each count
// of evaluations that the policy does not allow.
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
		i := len(cases)
		cases = append(cases, c)
		pkg := fmt.Sprintf("c%d", i)
		in := strconv.Quote(c.Input)
		if c.Stream {
			// A stream: ParseStream of a stream parser.
			codegentest.GeneratePackage(t, dir, pkg, mustParse(c.Src), codegen.GenOptions{Start: c.Start, Stream: true, Observability: true})
			fmt.Fprintf(&imports, "\t%q\n", "codegentest/"+pkg)
			fmt.Fprintf(&calls, "\t{ var p zpcore.Profile; _ = %[1]s.ParseStream(strings.NewReader(%[2]s), func(*%[1]s.Node) error { return nil }, zprt.WithProfile(&p)); fmt.Print(evals(p, %[3]q), \" \") }\n\tfmt.Println()\n", pkg, in, c.Rule)
			continue
		}
		codegentest.GeneratePackage(t, dir, pkg, mustParse(c.Src), codegen.GenOptions{Start: c.Start, Types: true, Recognize: true, Observability: true})
		// The Document of the same grammar.
		codegentest.GeneratePackage(t, dir, "d"+pkg, mustParse(c.Src), codegen.GenOptions{Start: c.Start, Document: true, Observability: true})
		fmt.Fprintf(&imports, "\t%q\n\t%q\n", "codegentest/"+pkg, "codegentest/d"+pkg)
		fmt.Fprintf(&calls, "\t{ var p zpcore.Profile; d, _ := d%s.NewDocument(%s); _, _ = d.Parse(zprt.WithProfile(&p)); fmt.Print(evals(p, %q), \" \") }\n", pkg, in, c.Rule)
		for _, op := range []string{"_, _ = %s.Parse(%s, zprt.WithProfile(&p))", "_, _ = %s.ParseValue(%s, zprt.WithProfile(&p))", "_ = %s.Recognize(%s, zprt.WithProfile(&p))"} {
			fmt.Fprintf(&calls, "\t{ var p zpcore.Profile; "+op+"; fmt.Print(evals(p, %q), \" \") }\n", pkg, in, c.Rule)
		}
		calls.WriteString("\tfmt.Println()\n")
	}
	main := `package main

import (
	"fmt"
	"strings"

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
		parse, stored := c.StorePolicyEvals(true)
		recognize, _ := c.StorePolicyEvals(false)
		got := strings.Join(strings.Fields(lines[i]), " ")
		if c.Stream {
			// A stream stores every first call, which the policy allows: no evaluation beyond those
			// that a stored result leaves, and none fewer.
			if n, err := strconv.Atoi(got); err != nil || n < stored || n > parse {
				bad = append(bad, fmt.Sprintf("%s: %s evaluated %s times in a stream, want %d to %d", c.Name, c.Rule, got, stored, parse))
			}
			continue
		}
		// A Document parse memoizes every call, so its first parse evaluates as often as a rule
		// whose first call stores (see the engine's TestMemoStorePolicyDocument).
		want := fmt.Sprintf("%d %d %d %d", stored, parse, parse, recognize)
		if got != want {
			bad = append(bad, fmt.Sprintf("%s: %s evaluated %s times (Document, Parse, ParseValue, Recognize), want %s", c.Name, c.Rule, got, want))
		}
	}
	return bad
}
