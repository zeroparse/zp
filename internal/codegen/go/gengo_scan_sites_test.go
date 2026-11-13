package gogen_test

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/gentest"
)

// TestScanSitesGeneratedMatchEngine checks that generated parsers, whose repetitions of one
// character record and reuse their walks at scan sites, give the engine's results on the scan-site
// grammars: values and syntax errors with their expectations, from the Node and typed entry points.
func TestScanSitesGeneratedMatchEngine(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testScanSitesGeneratedMatchEngine)
}

func testScanSitesGeneratedMatchEngine(t *testing.T) {
	goBin := skipRunsGoBin(t)
	cases := gentest.ScanSiteCases(40)
	gentest.GeneratedParsersCorpus(t, goBin, cases, codegen.GenOptions{})
	gentest.GeneratedTypesCorpus(t, goBin, cases, codegen.GenOptions{})
}

// TestScanSitesGeneratedStream checks a generated stream whose elements restart walks of scan
// sites over input long enough for the stream to drop records at its commits.
func TestScanSitesGeneratedStream(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testScanSitesGeneratedStream)
}

func testScanSitesGeneratedStream(t *testing.T) {
	goBin := skipRunsGoBin(t)
	unit := strings.Repeat("a", 40) + "?" + strings.Repeat("b", 20) + "!" + strings.Repeat(" ", 30) + "w"
	cases := []gentest.Case{{Name: "scan sites", Src: `rule main = (r "!" / r "?" / s "w" / [ab ])* #stream $$
rule r = [ab]*
rule s = xs:[ ]+ => text($xs)`, Inputs: []string{strings.Repeat(unit, 50), strings.Repeat(unit, 50) + "c"}}}
	testGeneratedStreams(t, goBin, cases, codegen.GenOptions{})
}

// scanSiteMutations are the changes of the embedded library for the negative controls: walks that
// never take a record, and records stored at the position after the start of their walk.
var scanSiteMutations = map[string][][2]string{
	"never reuse": {{
		"	rec, ok := p.scanTables[site][start]\n",
		"	rec, ok := p.scanTables[site][start]\n\tok = false\n",
	}},
	"shifted start": {{
		"	t[start] = rec\n",
		"	t[start+1] = rec\n",
	}},
}

// scanSiteSpends builds the parsers of the sources with the embedded library changed by mutation
// (none if "") and returns, for each source, the least budget of the parse of each of its inputs
// and the results of the parses.
func scanSiteSpends(t *testing.T, goBin string, sources []string, inputs [][]string, mutation string) (spends [][]uint64, results []string) {
	t.Helper()
	dir := t.TempDir()
	write := func(name string, content []byte) {
		path := filepath.Join(dir, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", []byte(gentest.GoMod("scanlin")))
	var imports, entries strings.Builder
	for i, src := range sources {
		g, err := parseTest(src)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		pkg := fmt.Sprintf("g%d", i)
		files, err := gentest.Generate(t, g, codegen.GenOptions{Module: "scanlin/" + pkg, Package: pkg, Start: "main", Lib: codegen.LibEmbed, LimitControls: true})
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		for _, f := range files {
			content := string(f.Content)
			if strings.HasSuffix(f.Path, "libgo/scan_sites.go") && mutation != "" {
				for _, r := range scanSiteMutations[mutation] {
					if !strings.Contains(content, r[0]) {
						t.Fatalf("mutation %s: %q not in the library", mutation, r[0])
					}
					content = strings.Replace(content, r[0], r[1], 1)
				}
			}
			write(pkg+"/"+f.Path, []byte(content))
		}
		fmt.Fprintf(&imports, "\t%q\n", "scanlin/"+pkg)
		fmt.Fprintf(&entries, `	{%#v, func(s string, budget uint64) (bool, string) {
		n, err := %[2]s.Parse(s, %[2]s.WithLimits(%[2]s.Limits{Compute: budget}))
		j, _ := json.Marshal(n)
		return errors.Is(err, %[2]s.ErrLimit), string(j) + " " + fmt.Sprint(err)
	}},
`, inputs[i], pkg)
	}
	write("main.go", []byte(`package main

import (
	"encoding/json"
	"errors"
	"fmt"

`+imports.String()+`)

var entries = []struct {
	inputs []string
	parse  func(string, uint64) (bool, string)
}{
`+entries.String()+`}

func main() {
	for _, e := range entries {
		for _, s := range e.inputs {
			lo, hi := uint64(1), uint64(1)
			for {
				if limited, _ := e.parse(s, hi); !limited {
					break
				}
				lo, hi = hi+1, hi*2
			}
			for lo < hi {
				mid := (lo + hi) / 2
				if limited, _ := e.parse(s, mid); limited {
					lo = mid + 1
				} else {
					hi = mid
				}
			}
			_, r := e.parse(s, 1<<40)
			fmt.Printf("%d %q\n", lo, r)
		}
	}
}
`))
	cmd := exec.Command(goBin, "run", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod -trimpath", "GOTOOLCHAIN=local")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	for _, in := range inputs {
		var row []uint64
		for range in {
			if len(lines) == 0 {
				t.Fatalf("missing output lines:\n%s", out)
			}
			spend, result, ok := strings.Cut(lines[0], " ")
			v, err := strconv.ParseUint(spend, 10, 64)
			if !ok || err != nil {
				t.Fatalf("malformed line %q", lines[0])
			}
			row = append(row, v)
			results = append(results, result)
			lines = lines[1:]
		}
		spends = append(spends, row)
	}
	return spends, results
}

// TestScanSitesGeneratedLinear checks that grammars which restart a walk of a scan site at one
// position once for each level of a nest, by a rule called at the same position and by the calling
// rule itself, spend a budget linear in the input in generated parsers, that walks that never take
// a record make them quadratic and give the same results, and that records stored at the wrong
// start change the results.
func TestScanSitesGeneratedLinear(t *testing.T) {
	t.Parallel()
	gentest.EachMode(t, testScanSitesGeneratedLinear)
}

func testScanSitesGeneratedLinear(t *testing.T) {
	goBin := skipRunsGoBin(t)
	gentest.NeedsBudgetControls(t)
	sources := []string{
		"rule main = e \" \" ws $$\nrule e = \"(\" e? (ws \")\")?\nrule ws = [ ]*",
		"rule main = e \" \" ws $$\nrule e = \"(\" e? (ws \")\")?\nrule ws = [ ]* #atomic",
		"rule main = e $$\nrule e = \"(\" (e ws \"x\" / e) / \".\"\nrule ws = c:[ ]*",
	}
	sizes := []int{200, 400, 800}
	var inputs [][]string
	for i := range sources {
		var row []string
		for _, n := range sizes {
			if i == 2 {
				row = append(row, strings.Repeat("(", n)+"."+strings.Repeat(" ", n))
			} else {
				row = append(row, strings.Repeat("(", n)+strings.Repeat(" ", n))
			}
		}
		inputs = append(inputs, row)
	}
	slopes := func(row []uint64) (lo, hi float64) {
		lo, hi = math.Inf(1), math.Inf(-1)
		for i := 1; i < len(row); i++ {
			s := math.Log2(float64(row[i]) / float64(row[i-1]))
			lo, hi = min(lo, s), max(hi, s)
		}
		return lo, hi
	}
	spends, results := scanSiteSpends(t, goBin, sources, inputs, "")
	t.Logf("spends at %v: %v", sizes, spends)
	for i, row := range spends {
		if _, hi := slopes(row); hi > 1.2 {
			t.Errorf("%q: spends %v grow faster than linearly", sources[i], row)
		}
	}
	spends, plain := scanSiteSpends(t, goBin, sources, inputs, "never reuse")
	t.Logf("spends of walks that never reuse: %v", spends)
	for i, row := range spends {
		if lo, _ := slopes(row); lo < 1.7 {
			t.Errorf("%q, walks that never reuse: spends %v are not quadratic", sources[i], row)
		}
	}
	for i := range results {
		if results[i] != plain[i] {
			t.Errorf("input %d: records change the result\n got  %s\n want %s", i, results[i], plain[i])
		}
	}
	_, shifted := scanSiteSpends(t, goBin, sources, inputs, "shifted start")
	differ := false
	for i := range results {
		differ = differ || results[i] != shifted[i]
	}
	if !differ {
		t.Errorf("records stored at the wrong start change no result")
	}
}
