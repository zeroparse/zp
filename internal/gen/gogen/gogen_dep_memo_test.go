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

// depMemoGenCases returns the grammars of gentest.DependencyMemoCases for seeds below seeds that
// generate Go parsers.
func depMemoGenCases(t *testing.T, seeds int) []gentest.Case {
	t.Helper()
	var cases []gentest.Case
	for _, c := range gentest.DependencyMemoCases(seeds) {
		g, err := parseTest(c.Src)
		if err != nil {
			continue
		}
		if _, err := gentest.GenerateCode(t, g, gen.GenOptions{Package: "g", Start: "main", Recognize: true}); err != nil {
			continue
		}
		cases = append(cases, c)
	}
	if len(cases) < seeds*2 {
		t.Fatalf("only %d grammars of %d seeds generate", len(cases), seeds)
	}
	return cases
}

// TestGeneratedLeftRecursionDependencyMemo checks that the Parse, Recognize and ParseAST of
// generated Go parsers give the engine's results for grammars whose leaders grow inside the growth
// of other leaders of their cycle at one position, where the dependency memo reuses results by the
// seeds they read and the lookaheads they observed.
func TestGeneratedLeftRecursionDependencyMemo(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testGeneratedLeftRecursionDependencyMemo)
}

func testGeneratedLeftRecursionDependencyMemo(t *testing.T) {
	if testing.Short() {
		t.Skip("builds generated code")
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	if _, err := os.Stat(goBin); err != nil {
		t.Skip("go command not found")
	}
	cases := depMemoGenCases(t, 60)
	gentest.GeneratedParsersCorpus(t, goBin, cases, gen.GenOptions{})
	gentest.GeneratedTypesCorpus(t, goBin, cases, gen.GenOptions{ConvertTypes: true}, len(cases))
}

// TestGeneratedLeftRecursionDependencyMemoLinear checks that the Parse and ParseAST of generated Go
// parsers of gentest.DependencyMemoShapes evaluate a number of calls linear in the length of the
// input: doubling the input at most a little more than doubles the evaluations. They also parse an
// input of length 10,000; with limit controls, within the default budget, which grows linearly with
// the input and counts work that evaluations do not show, such as the scan of the jumps recorded
// for a seed. The shapes of gentest.ContextStoreShapes, whose constant is large, are compared at
// gentest.ContextStoreSizes instead, without the long input.
func TestGeneratedLeftRecursionDependencyMemoLinear(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testGeneratedLeftRecursionDependencyMemoLinear)
}

func testGeneratedLeftRecursionDependencyMemoLinear(t *testing.T) {
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
	shapes := gentest.SampledShapes()
	costly := len(shapes)
	shapes = append(shapes, gentest.ContextStoreShapes...)
	var imports, calls strings.Builder
	for i, s := range shapes {
		pkg := fmt.Sprintf("g%d", i)
		gentest.GeneratePackage(t, dir, pkg, mustParse("@start\nrule main = "+s.Rules), gen.GenOptions{Start: "main", Types: true, Observability: true})
		fmt.Fprintf(&imports, "\t%q\n", "gentest/"+pkg)
		sizes, long := []int{80, 160}, 10000
		if i >= costly {
			sizes, long = gentest.ContextStoreSizes[:], 0
		}
		for _, n := range sizes {
			in := strconv.Quote(s.Input(n))
			fmt.Fprintf(&calls, "\t{ var p zprt.Profile; if _, err := %s.Parse(%s, zprt.WithProfile(&p)); err != nil { panic(err) }; fmt.Print(p.Snapshot().Evals, \" \") }\n", pkg, in)
			fmt.Fprintf(&calls, "\t{ var p zprt.Profile; if _, err := %s.ParseAST(%s, zprt.WithProfile(&p)); err != nil { panic(err) }; fmt.Print(p.Snapshot().Evals, \" \") }\n", pkg, in)
		}
		if long == 0 {
			calls.WriteString("\tfmt.Print(true, \" \", true)\n")
		} else {
			in := strconv.Quote(s.Input(long))
			fmt.Fprintf(&calls, "\t{ _, err := %s.Parse(%s); _, errAST := %s.ParseAST(%s); fmt.Print(err == nil, \" \", errAST == nil) }\n", pkg, in, pkg, in)
		}
		calls.WriteString("\tfmt.Println()\n")
	}
	main := `package main

import (
	"fmt"

	zprt "zeroparse.dev/zp/runtime"
` + imports.String() + `)

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
	if len(lines) != len(shapes) {
		t.Fatalf("got %d lines, want %d\n%s", len(lines), len(shapes), out)
	}
	for i, s := range shapes {
		var parse80, ast80, parse160, ast160 int
		var parseLong, astLong bool
		if _, err := fmt.Sscan(lines[i], &parse80, &ast80, &parse160, &ast160, &parseLong, &astLong); err != nil {
			t.Fatalf("%s: %q: %v", s.Name, lines[i], err)
		}
		if !parseLong || !astLong {
			t.Errorf("%s at 10000 within the default budget: Parse succeeded %v, ParseAST succeeded %v", s.Name, parseLong, astLong)
		}
		for _, r := range []struct {
			name         string
			small, large int
		}{{"Parse", parse80, parse160}, {"ParseAST", ast80, ast160}} {
			if i >= costly {
				if r.small == 0 || r.large > r.small*gentest.ContextStoreGrowth {
					t.Errorf("%s %s: %d evaluations at m=%d, %d at m=%d", s.Name, r.name, r.small, gentest.ContextStoreSizes[0], r.large, gentest.ContextStoreSizes[1])
				}
			} else if r.small == 0 || r.large > r.small*5/2 {
				t.Errorf("%s %s: %d evaluations at 80, %d at 160", s.Name, r.name, r.small, r.large)
			}
		}
	}
}
