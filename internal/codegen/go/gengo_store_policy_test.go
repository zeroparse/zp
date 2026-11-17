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
// leave no result in the memo. So Parse, ParseValue, Recognize, ParseStream and the first parse of a
// Document (untyped and typed), for each case of codegentest.StorePolicyCases, evaluate the rule at
// least as often as a memo storing every first call and at most as often as one deferring each.
func TestGeneratedStorePolicy(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, func(t *testing.T) {
		if bad := storePolicyEnvelope(generatedStorePolicyCounts(t, codegentest.LibCopy())); len(bad) > 0 {
			t.Error(strings.Join(bad, "\n"))
		}
	})
}

// TestGeneratedStorePolicyReferenceCounts pins what the generated Go library does within the policy,
// which the policy does not require and TestGeneratedStorePolicy does not: an ordinary parse defers
// the first call of a rule at a position until repeated calls exceed a sixteenth of its calls, so it
// evaluates the rule exactly as often as the model of codegentest.StorePolicyCase says; a stream and
// a Document parse memoize every first call. A change of that choice that the policy allows updates
// this test.
func TestGeneratedStorePolicyReferenceCounts(t *testing.T) {
	t.Parallel()
	codegentest.EachMode(t, func(t *testing.T) {
		var bad []string
		for _, n := range generatedStorePolicyCounts(t, codegentest.LibCopy()) {
			bad = append(bad, n.pinMismatches()...)
		}
		if len(bad) > 0 {
			t.Error(strings.Join(bad, "\n"))
		}
	})
}

// TestGeneratedStorePolicyMutantsFail is the negative control of TestGeneratedStorePolicy: a library
// that defers a result that the policy requires to be stored fails it, on the case that has the calls
// where the mutant differs, in the dense seen table and in the sparse one, in a stream and in a
// Document.
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
			"\t\tpp := p.Pos\n\t\tif p.Silent > 0 {\n\t\t\tpp += 1 << 12\n\t\t}\n\t\tfirst = t.markSeen(pp, r.Seen)\n"}}, "inside first"},
		{"a second call deferred as well, dense", []edit{{dense,
			"\t\tfirst = t.markSeen(p.Pos, r.Seen) || t.markSeen(p.Pos+1<<12, r.Seen)\n"}}, "outside only"},
		{"a second call deferred as well, sparse", []edit{{sparse,
			"\t\tfirst = markSeenPage(page, pos) || t.markSparseSeen(pos+1<<12, r.Seen)\n"}}, "inside first, sparse"},
		{"a stream defers a first call and a second", []edit{{noStream, ""}, {dense,
			"\t\tfirst = t.markSeen(p.Pos, r.Seen) || t.markSeen(p.Pos+1<<12, r.Seen)\n"}}, "outside only, stream"},
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
				bad := storePolicyEnvelope(generatedStorePolicyCounts(t, lib))
				if len(bad) == 0 {
					t.Fatal("the test passed with the mutant")
				}
				found := false
				for _, b := range bad {
					found = found || strings.HasPrefix(b, m.failing+": ")
				}
				if !found {
					t.Fatalf("no case %q failed:\n%s", m.failing, strings.Join(bad, "\n"))
				}
				t.Logf("%d cases failed with the mutant, for example:\n%s", len(bad), bad[0])
			})
		})
	}
}

// storeCounts are the evaluations of a case's rule in each entry of a generated Go parser: of a
// stream case, in a stream; of the others, in the first parse of the Document (untyped and typed) and
// in Parse, ParseValue and Recognize.
type storeCounts struct {
	c                                          codegentest.StorePolicyCase
	stream, doc, typedDoc, parse, value, recog int
}

// envelope returns the counts of n that are outside what the policy allows: not fewer than a memo
// storing every first call leaves, not more than one deferring each leaves.
func (n storeCounts) envelope() []string {
	var bad []string
	check := func(entry string, got int, lo, hi int) {
		if got < lo || got > hi {
			bad = append(bad, fmt.Sprintf("%s: %s evaluated %d times in %s, want %d to %d", n.c.Name, n.c.Rule, got, entry, lo, hi))
		}
	}
	deferred, stored := n.c.StorePolicyEvals(true)
	if n.c.Stream {
		check("a stream", n.stream, stored, deferred)
		return bad
	}
	recDeferred, recStored := n.c.StorePolicyEvals(false)
	check("a Document", n.doc, stored, deferred)
	check("a typed Document", n.typedDoc, stored, deferred)
	check("Parse", n.parse, stored, deferred)
	check("ParseValue", n.value, stored, deferred)
	check("Recognize", n.recog, recStored, recDeferred)
	return bad
}

// pinMismatches returns the counts of n that differ from what the library does: see
// TestGeneratedStorePolicyReferenceCounts.
func (n storeCounts) pinMismatches() []string {
	var bad []string
	check := func(entry string, got, want int) {
		if got != want {
			bad = append(bad, fmt.Sprintf("%s: %s evaluated %d times in %s, the library %d", n.c.Name, n.c.Rule, got, entry, want))
		}
	}
	deferred, stored := n.c.StorePolicyEvals(true)
	if n.c.Stream {
		check("a stream", n.stream, stored)
		return bad
	}
	recDeferred, _ := n.c.StorePolicyEvals(false)
	check("a Document", n.doc, stored)
	check("a typed Document", n.typedDoc, stored)
	check("Parse", n.parse, deferred)
	check("ParseValue", n.value, deferred)
	check("Recognize", n.recog, recDeferred)
	return bad
}

func storePolicyEnvelope(counts []storeCounts) []string {
	var bad []string
	for _, n := range counts {
		bad = append(bad, n.envelope()...)
	}
	return bad
}

// generatedStorePolicyCounts builds a parser of lib's library for each case (a stream parser for
// the stream cases, a Document and a typed Document too for the others), runs it, and returns the
// evaluations of the case's rule in each entry.
func generatedStorePolicyCounts(t *testing.T, lib string) []storeCounts {
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
			codegentest.GeneratePackage(t, dir, pkg, mustParse(c.Src), codegen.GenOptions{Start: c.Start, Stream: true, Observability: true})
			fmt.Fprintf(&imports, "\t%q\n", "codegentest/"+pkg)
			fmt.Fprintf(&calls, "\t{ var p zpcore.Profile; _ = %[1]s.ParseStream(strings.NewReader(%[2]s), func(*%[1]s.Node) error { return nil }, zprt.WithProfile(&p)); fmt.Print(evals(p, %[3]q), \" \") }\n\tfmt.Println()\n", pkg, in, c.Rule)
			continue
		}
		codegentest.GeneratePackage(t, dir, pkg, mustParse(c.Src), codegen.GenOptions{Start: c.Start, Types: true, Recognize: true, Observability: true})
		codegentest.GeneratePackage(t, dir, "d"+pkg, mustParse(c.Src), codegen.GenOptions{Start: c.Start, Document: true, Observability: true})
		codegentest.GeneratePackage(t, dir, "t"+pkg, mustParse(c.Src), codegen.GenOptions{Start: c.Start, Document: true, Types: true, Observability: true})
		fmt.Fprintf(&imports, "\t%q\n\t%q\n\t%q\n", "codegentest/"+pkg, "codegentest/d"+pkg, "codegentest/t"+pkg)
		fmt.Fprintf(&calls, "\t{ var p zpcore.Profile; d, _ := d%s.NewDocument(%s); _, _ = d.Parse(zprt.WithProfile(&p)); fmt.Print(evals(p, %q), \" \") }\n", pkg, in, c.Rule)
		fmt.Fprintf(&calls, "\t{ var p zpcore.Profile; d, _ := t%s.NewDocument(%s); _, _ = d.ParseValue(zprt.WithProfile(&p)); fmt.Print(evals(p, %q), \" \") }\n", pkg, in, c.Rule)
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
	counts := make([]storeCounts, len(cases))
	for i, c := range cases {
		f := strings.Fields(lines[i])
		var v []int
		for _, x := range f {
			n, err := strconv.Atoi(x)
			if err != nil {
				t.Fatalf("%s: %v\n%s", c.Name, err, out)
			}
			v = append(v, n)
		}
		switch {
		case c.Stream && len(v) == 1:
			counts[i] = storeCounts{c: c, stream: v[0]}
		case !c.Stream && len(v) == 5:
			counts[i] = storeCounts{c: c, doc: v[0], typedDoc: v[1], parse: v[2], value: v[3], recog: v[4]}
		default:
			t.Fatalf("%s: %d counts in %q", c.Name, len(v), lines[i])
		}
	}
	return counts
}
