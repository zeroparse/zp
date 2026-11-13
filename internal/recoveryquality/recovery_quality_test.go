// Package recoveryquality measures how well #recover repairs invalid input on two ready-made languages.
//
// Each language has a corpus of invalid inputs in testdata/recovery/<language>.txt: mutations of valid inputs
// generated from the ready-made grammar (testdata/grammars/<language>), one JSON object per line. The
// recovering variant of the grammar is testdata/recovery/<language>.zp. The test parses every corpus input with
// the variant, computes the measures of docs/guide/recovery-quality.md and compares them with the table in that
// guide; go test ./internal/recoveryquality -update regenerates the corpora and the table.
package recoveryquality_test

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/engine"
	"zeroparse.dev/zp/sample"
)

var update = flag.Bool("update", false, "regenerate the corpora and the results table")

// A case is one invalid input: a valid Base changed by one mutation at byte offset Off.
type testCase struct {
	Input    string `json:"input"`
	Base     string `json:"base"`
	Off      int    `json:"offset"`
	Mutation string `json:"mutation"`
}

var languages = []struct {
	name, grammar string
	size          int
	opts          []sample.Option
}{
	{"json", "json", 300, []sample.Option{sample.WithSeed(1), sample.WithMaxLen(160), sample.WithMaxDepth(4)}},
	{"cel", "cel", 300, []sample.Option{sample.WithSeed(1), sample.WithMaxLen(120), sample.WithMaxDepth(5)}},
}

const (
	window     = 8 // bytes around the mutation in which a recovered error counts as located
	tablePath  = "../../docs/guide/recovery-quality.md"
	beginTable = "<!-- recovery-quality-table:begin -->"
	endTable   = "<!-- recovery-quality-table:end -->"
)

func compile(t *testing.T, path string) *engine.Parser {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	p, err := engine.CompileSource(string(src), "main")
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return p
}

// generate returns the corpus: mutations of inputs generated from the ready-made grammar, with the byte offset of
// the first difference from the base.
func generate(t *testing.T, name, grammar string, size int, opts []sample.Option) []testCase {
	t.Helper()
	g, err := sample.New(compile(t, filepath.Join("../../testdata/grammars", grammar, grammar+".zp")), opts...)
	if err != nil {
		t.Fatal(err)
	}
	invalid, err := g.GenerateInvalid(8 * size)
	if err != nil {
		t.Fatal(err)
	}
	var out []testCase
	for _, inv := range invalid {
		// Only inputs with a separator: recovery works on the elements of a list, so an input without one
		// (a lone literal) has no place to recover at.
		if len(out) == size || !strings.Contains(inv.Base, ",") {
			continue
		}
		off := 0
		for off < len(inv.Input) && off < len(inv.Base) && inv.Input[off] == inv.Base[off] {
			off++
		}
		out = append(out, testCase{inv.Input, inv.Base, off, inv.Mutation})
	}
	return out
}

func readCorpus(t *testing.T, name string) []testCase {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("../../testdata/recovery", name+".txt"))
	if err != nil {
		t.Fatal(err)
	}
	var out []testCase
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var c testCase
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			t.Fatal(err)
		}
		out = append(out, c)
	}
	return out
}

func writeCorpus(t *testing.T, name string, cases []testCase) {
	t.Helper()
	var b strings.Builder
	for _, c := range cases {
		line, err := json.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join("../../testdata/recovery", name+".txt"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// errorRanges collects the ranges of the error values of a tree.
func errorRanges(n *core.Node, out *[][2]int, seen map[*core.Node]bool) {
	if n == nil || seen[n] {
		return
	}
	seen[n] = true
	switch n.Kind() {
	case core.KindError:
		*out = append(*out, [2]int{n.Start(), n.End()})
	case core.KindStruct:
		for _, v := range n.Fields() {
			if c, ok := v.(*core.Node); ok {
				errorRanges(c, out, seen)
			}
		}
	case core.KindTuple, core.KindList, core.KindTupleValue, core.KindListValue:
		for _, c := range n.Elems() {
			errorRanges(c, out, seen)
		}
	}
}

// outcome is the result of the recovering parse of one input.
type outcome struct {
	recovered bool    // the parse returned a tree and recovered errors
	failed    bool    // the parse failed without a tree
	errors    int     // number of recovered errors
	located   bool    // every error value overlaps the window around the mutation
	skipped   float64 // fraction of the input that the error values cover
}

func measure(p *engine.Parser, c testCase) outcome {
	node, err := p.Parse(c.Input)
	var recovered core.SyntaxErrors
	switch {
	case err == nil:
		// The mutation is accepted by the recovering grammar too (it is invalid only for the ready-made one):
		// there is nothing to repair.
		return outcome{}
	case errors.As(err, &recovered) && node != nil:
		var rs [][2]int
		errorRanges(node, &rs, map[*core.Node]bool{})
		o := outcome{recovered: true, errors: len(recovered), located: len(rs) > 0}
		covered := 0
		for _, r := range rs {
			covered += r[1] - r[0]
			if r[1] < c.Off-window || r[0] > c.Off+window {
				o.located = false
			}
		}
		o.skipped = float64(covered) / float64(max(len(c.Input), 1))
		return o
	default:
		return outcome{failed: true}
	}
}

type result struct {
	kinds                               map[string][2]int // per mutation kind: inputs, recovered
	name                                string
	inputs, accepted, recovered, failed int
	one, located                        int
	medianSkipped, p90Skipped           float64
}

// kind is the kind of a mutation: the first word of its description.
func kind(c testCase) string { return strings.Fields(c.Mutation)[0] }

func run(t *testing.T, name string) result {
	t.Helper()
	p := compile(t, filepath.Join("../../testdata/recovery", name+".zp"))
	r := result{name: name, kinds: map[string][2]int{}}
	var skipped []float64
	for _, c := range readCorpus(t, name) {
		r.inputs++
		o := measure(p, c)
		k := r.kinds[kind(c)]
		k[0]++
		if o.recovered {
			k[1]++
		}
		r.kinds[kind(c)] = k
		switch {
		case o.failed:
			r.failed++
		case o.recovered:
			r.recovered++
			if o.errors == 1 {
				r.one++
			}
			if o.located {
				r.located++
			}
			skipped = append(skipped, o.skipped)
		default:
			r.accepted++
		}
	}
	slices.Sort(skipped)
	if n := len(skipped); n > 0 {
		r.medianSkipped, r.p90Skipped = skipped[n/2], skipped[n*9/10]
	}
	return r
}

func pct(n, of int) string {
	if of == 0 {
		return "-"
	}
	return fmt.Sprintf("%.0f%%", 100*float64(n)/float64(of))
}

func table(rs []result) string {
	var b strings.Builder
	b.WriteString("| Language | Invalid inputs | Accepted by the recovering grammar | Recovered | Failed | One error | Located | Skipped, median | Skipped, 90th percentile |\n")
	b.WriteString("|:--|--:|--:|--:|--:|--:|--:|--:|--:|\n")
	for _, r := range rs {
		rejected := r.inputs - r.accepted
		fmt.Fprintf(&b, "| %s | %d | %d | %s | %s | %s | %s | %.0f%% | %.0f%% |\n", r.name, r.inputs, r.accepted,
			pct(r.recovered, rejected), pct(r.failed, rejected), pct(r.one, r.recovered), pct(r.located, r.recovered),
			100*r.medianSkipped, 100*r.p90Skipped)
	}
	b.WriteString("\n| Mutation | " + strings.Join(names(rs), " | ") + " |\n|:--|" + strings.Repeat("--:|", len(rs)) + "\n")
	for _, k := range []string{"delete", "insert", "replace", "duplicate", "swap", "truncate"} {
		fmt.Fprintf(&b, "| %s ", k)
		for _, r := range rs {
			fmt.Fprintf(&b, "| %s ", pct(r.kinds[k][1], r.kinds[k][0]))
		}
		b.WriteString("|\n")
	}
	return b.String()
}

func names(rs []result) []string {
	var out []string
	for _, r := range rs {
		out = append(out, r.name+" recovered")
	}
	return out
}

func TestRecoveryQuality(t *testing.T) {
	if *update {
		for _, l := range languages {
			writeCorpus(t, l.name, generate(t, l.name, l.grammar, l.size, l.opts))
		}
	}
	var rs []result
	for _, l := range languages {
		rs = append(rs, run(t, l.name))
	}
	got := table(rs)
	t.Log("\n" + got)
	doc, err := os.ReadFile(tablePath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(doc)
	i, j := strings.Index(s, beginTable), strings.Index(s, endTable)
	if i < 0 || j < i {
		t.Fatalf("%s lacks the table markers", tablePath)
	}
	want := s[:i+len(beginTable)] + "\n" + got + s[j:]
	if *update {
		if err := os.WriteFile(tablePath, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if want != s {
		t.Fatalf("the table in %s is stale; run go test ./internal/recoveryquality -update", tablePath)
	}
}
