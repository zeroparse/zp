package gogen_test

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
	"zeroparse.dev/zp/internal/engine"
)

// The examined end of a call in an observed parse is exact: every unit of input that can change the
// call's result lies below it, and nothing beyond what the call or a call it used looked at. The tests
// here compare the ends that the generated parsers report on every route an observed parse takes with
// a reference: the typed parse (ParseValue), the Node parse (Parse) and the recognizer (Recognize) with
// the engine's parse of the same input and unit, and the stream parse (ParseStream), which reads its
// input one byte at a time, with the batch parse of the same generated parser, which the engine checks.

// examinedProbeMain is the consumer of the generated packages: for each input, unit and entry it prints
// one JSON report with the exits of the observed parse and its error.
const examinedProbeMain = `package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	zprt "zeroparse.dev/zp/codegen/go/lib"
	"zeroparse.dev/zp/core"
IMPORTS)

type entry struct {
	name string
	run  func(string, ...zprt.ParseOption) error
}

var entries = [][]entry{
ENTRIES}

// oneByte returns one byte per Read, so that a stream parse loads its input as late as it can.
type oneByte struct{ s string }

func (r *oneByte) Read(b []byte) (int, error) {
	if r.s == "" {
		return 0, io.EOF
	}
	b[0], r.s = r.s[0], r.s[1:]
	return 1, nil
}

func main() {
	var inputs [][][]byte // bytes: JSON would replace invalid UTF-8 in a string
	data, _ := os.ReadFile(os.Args[1])
	if err := json.Unmarshal(data, &inputs); err != nil {
		panic(err)
	}
	for i := range entries {
		for j, in := range inputs[i] {
			for _, unit := range []core.PositionUnit{core.CodePoints, core.Bytes} {
				for _, e := range entries[i] {
					var exits []core.TraceEvent
					err := e.run(string(in), zprt.WithUnit(unit), zprt.WithTrace(func(ev core.TraceEvent) {
						if ev.Kind == core.TraceExit {
							exits = append(exits, ev)
						}
					}))
					b, _ := json.Marshal(struct {
						Case, Input int
						Bytes       bool
						Entry       string
						Exits       []core.TraceEvent
						Err         string
					}{i, j, unit == core.Bytes, e.name, exits, fmt.Sprint(err)})
					fmt.Println(string(b))
				}
			}
		}
	}
}
`

// examinedReport is one report of the consumer.
type examinedReport struct {
	Case, Input int
	Bytes       bool
	Entry       string
	Exits       []examinedExit
	Err         string
}

// examinedExit is an exit of a generated parse or of the engine's.
type examinedExit struct {
	Rule                      string
	Level, Pos, End, Examined int
	Matched                   bool
}

// examinedKey identifies the calls that a reference and a generated parse both make: a call of the same
// rule at the same position and level with the same outcome.
type examinedKey struct {
	rule            string
	level, pos, end int
	matched         bool
}

// examinedCases are the cases of the exactness tests: the representative cases of the generated corpus,
// the shapes of the observation tests, examinedExtras and the stream grammars.
func examinedCases(t *testing.T) []codegentest.Case {
	t.Helper()
	cases := append(codegentest.Representative(codegentest.GenCorpus(t)), observeShapes...)
	cases = append(cases, examinedExtras()...)
	for _, c := range streamCases(t) {
		if !strings.Contains(c.Src, "#stream") || len(c.Inputs) == 0 {
			continue
		}
		dup := false
		for _, d := range cases {
			dup = dup || d.Src == c.Src
		}
		if !dup {
			cases = append(cases, c)
		}
	}
	return cases
}

// runExaminedProbes builds the cases with observability, the typed library and the recognizer, and with
// ParseStream where the start rule has a #stream repetition, as the packages f0..fN of a module that
// imports the library from lib (codegentest.LibCopy or a copy with a mutant). mutate, if not nil, changes
// the code of every package. It returns the reports of the consumer.
func runExaminedProbes(t *testing.T, cases []codegentest.Case, lib string, mutate func([]byte) []byte) []examinedReport {
	t.Helper()
	dir := t.TempDir()
	write := func(name string, content []byte) {
		path := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", []byte(codegentest.GoModAt("examinedprobe", lib)))
	var imports, entries strings.Builder
	var inputs [][][]byte
	for i, c := range cases {
		g, err := parseTest(c.Src)
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		o := codegen.GenOptions{Start: "main", Types: true, Recognize: true, Observability: true, Stream: true}
		// The package keeps its types in a package of their own: merged into one file, the types of some
		// grammars would collide with the parser's names.
		if _, err := codegentest.GenerateCode(t, g, o); err != nil {
			if !strings.Contains(err.Error(), "needs a #stream repetition") {
				t.Fatalf("%s: %v", c.Name, err)
			}
			o.Stream = false
		}
		pkg := fmt.Sprintf("f%d", i)
		codegentest.GeneratePackageIn(t, dir, "examinedprobe", pkg, g, o)
		if mutate != nil {
			filepath.WalkDir(filepath.Join(dir, pkg), func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
					return err
				}
				code, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				return os.WriteFile(path, mutate(code), 0o644)
			})
		}
		fmt.Fprintf(&imports, "\tf%d \"examinedprobe/f%d\"\n", i, i)
		fmt.Fprintf(&entries, `	{
		{"value", func(in string, o ...zprt.ParseOption) error { _, err := f%[1]d.ParseValue(in, o...); return err }},
		{"parse", func(in string, o ...zprt.ParseOption) error { _, err := f%[1]d.Parse(in, o...); return err }},
		{"recognize", func(in string, o ...zprt.ParseOption) error { return f%[1]d.Recognize(in, o...) }},
`, i)
		if o.Stream {
			fmt.Fprintf(&entries, `		{"stream", func(in string, o ...zprt.ParseOption) error {
			return f%d.ParseStream(&oneByte{in}, func(*core.Node) error { return nil }, o...)
		}},
`, i)
		}
		entries.WriteString("\t},\n")
		var in [][]byte
		for _, s := range c.Inputs {
			in = append(in, []byte(s))
		}
		inputs = append(inputs, in)
	}
	write("main.go", []byte(strings.NewReplacer("IMPORTS", imports.String(), "ENTRIES", entries.String()).Replace(examinedProbeMain)))
	data, _ := json.Marshal(inputs)
	write("inputs.json", data)
	cmd := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "run", ".", "inputs.json")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod -trimpath", "GOTOOLCHAIN=local")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go run: %v\n%s", err, stderr.String())
	}
	var reports []examinedReport
	for _, l := range strings.Split(strings.TrimSuffix(string(out), "\n"), "\n") {
		var r examinedReport
		if err := json.Unmarshal([]byte(l), &r); err != nil {
			t.Fatalf("go run: malformed report %q: %v", l, err)
		}
		reports = append(reports, r)
	}
	return reports
}

// examinedDiffs compares the examined ends of the exits of got with those of the exits of ref for the same
// calls (examinedKey), the exits of the rule skip set aside, and returns the number of exits compared and the
// differences. A call that ref does not make is not compared; one that ref makes more than once may have any
// of its ends.
func examinedDiffs(got, ref []examinedExit, skip string) (compared int, diffs []string) {
	ends := map[examinedKey]map[int]bool{}
	for _, x := range ref {
		k := examinedKey{x.Rule, x.Level, x.Pos, x.End, x.Matched}
		if ends[k] == nil {
			ends[k] = map[int]bool{}
		}
		ends[k][x.Examined] = true
	}
	for _, x := range got {
		if x.Rule == skip {
			continue
		}
		want := ends[examinedKey{x.Rule, x.Level, x.Pos, x.End, x.Matched}]
		if want == nil {
			continue
		}
		compared++
		if !want[x.Examined] {
			diffs = append(diffs, fmt.Sprintf("examined end of %s@%d (level %d, end %d, matched %v): %d, reference %v", x.Rule, x.Pos, x.Level, x.End, x.Matched, x.Examined, want))
		}
	}
	return compared, diffs
}

// examinedMismatches checks the reports against their references (see the comment at the top of the file),
// and returns the number of exits compared and the differences, both by entry.
func examinedMismatches(t *testing.T, cases []codegentest.Case, reports []examinedReport) (compared map[string]int, diffs map[string][]string) {
	t.Helper()
	compared, diffs = map[string]int{}, map[string][]string{}
	type group struct {
		c, j  int
		bytes bool
	}
	batch := map[group][]examinedExit{}
	for _, r := range reports {
		if r.Entry == "parse" {
			batch[group{r.Case, r.Input, r.Bytes}] = r.Exits
		}
	}
	progs := map[int]*engine.Program{}
	for _, r := range reports {
		c := cases[r.Case]
		in := c.Inputs[r.Input]
		var ref []examinedExit
		skip := ""
		if r.Entry == "stream" {
			// The start rule of a stream parse delivers its elements instead of gathering them.
			ref, skip = batch[group{r.Case, r.Input, r.Bytes}], "main"
		} else {
			prog := progs[r.Case]
			if prog == nil {
				prog = mustCompile(t, c.Src)
				progs[r.Case] = prog
			}
			o := engine.ParseOptions{Expectations: true, Backend: engine.Closure, Unit: core.CodePoints, Recognize: r.Entry == "recognize"}
			if r.Bytes {
				o.Unit = core.Bytes
			}
			o.Trace = func(e engine.TraceEvent) {
				if e.Kind == engine.TraceExit {
					ref = append(ref, examinedExit{e.Rule, e.Level, e.Pos, e.End, e.Examined, e.Matched})
				}
			}
			prog.ParseWith("main", in, o)
		}
		n, d := examinedDiffs(r.Exits, ref, skip)
		compared[r.Entry] += n
		for _, msg := range d {
			diffs[r.Entry] = append(diffs[r.Entry], fmt.Sprintf("%s %q bytes=%v %s: %s", c.Name, in, r.Bytes, r.Entry, msg))
		}
	}
	return compared, diffs
}

// TestObservedExaminedEndsAreExact checks the examined ends of the observed parses of every route against
// their references, on examinedCases and examinedOnlyCases.
func TestObservedExaminedEndsAreExact(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	codegentest.EachMode(t, func(t *testing.T) {
		cases := append(examinedCases(t), examinedOnlyCases()...)
		compared, diffs := examinedMismatches(t, cases, runExaminedProbes(t, cases, codegentest.LibCopy(), nil))
		t.Logf("exits compared by entry: %v", compared)
		for _, entry := range []string{"value", "parse", "recognize", "stream"} {
			if compared[entry] == 0 {
				t.Errorf("no exit of %s compared", entry)
			}
			for i, msg := range diffs[entry] {
				if i == 10 {
					t.Errorf("%d more differences of %s", len(diffs[entry])-i, entry)
					break
				}
				t.Error(msg)
			}
		}
	})
}

// examinedExtras are shapes that reach records the library replays only on particular inputs:
// a lookahead at the start of a member called where its leader grows, whose outcome a context keeps, a
// recovery that ends its rule, whose skip walks the skip records serve, and a lookahead whose outcome is
// the same for two seeds of a growth but examines a different extent, reduced from random cycle grammar
// 304.
func examinedExtras() []codegentest.Case {
	unit := strings.Repeat("a", 40) + "c" + strings.Repeat("a", 9) + "e" + strings.Repeat("a", 20) + "d"
	return []codegentest.Case{
		{Name: "lookahead of a member at a growth start", Src: `rule main = e $$
rule e = e "+" m / m
rule m = &(e "x" t) e "x" / t
rule t = [0-9]+`, Inputs: []string{"1x2", "1x2+3", "1+2x34", "1x23x4", "1x2x3", "1+2+3x45"}},
		{Name: "recovery that ends its rule", Src: `rule main = (x "d" / [ace])* $$
rule x = "b" #recover(skip=[ac]+)`, Inputs: []string{strings.Repeat(unit, 6), "b" + strings.Repeat("a", 40) + "cd", strings.Repeat("b"+unit, 4)}},
		{Name: "growth steps of a leader taken again from the context", Src: `rule main = a $$
rule a = b "1" / b "2" / "a"
rule b = a "x" / b "y" / "b"`, Inputs: []string{"b1", "by2", "ax2", "byy1x2", "b1x1y2", "ax1yx2", "byyyy1x2", "byyyyyy2x1"}},
		{Name: "lookahead outcomes of a growth that differ only in their examined ends", Src: `rule main = r0 $$
rule r0 = r2
rule r1 = (r2 (r1 "b" "c" / r0) / &"b" r0) / "a"
rule r2 = r0 "b" / !r1? / "a"`, Inputs: []string{"a", "aac", "acb", "aaac", "acabca", "acbccc"}},
	}
}

// examinedOnlyCases are shapes whose examined ends the exactness tests check, but whose other fields
// producers do not yet report alike, so the observation agreement compares only their examined ends,
// those of calls that cannot match included:
// a growth whose last step, which does not grow, is taken from the context, reduced from random cycle
// grammar 103. Its leader takes every growth step from the context and evaluates nothing, which the
// engine reports as Memo=true and generated parsers as Memo=false
// (https://github.com/zeroparse/zp/issues/295).
func examinedOnlyCases() []codegentest.Case {
	return []codegentest.Case{
		{Name: "last growth step taken from the context", Src: `rule main = r0
rule r0 = r3 / r1 / "a"
rule r1 = !("c" / r4)* / r0
rule r3 = r0 r3 / r4
rule r4 = &"c" (r1 / r4?)`, Inputs: []string{"a", "c", "cc", "ca", "b", "ac", "cbbcbcaa"}},
	}
}
