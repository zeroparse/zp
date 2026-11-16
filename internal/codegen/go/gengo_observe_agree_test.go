package gogen_test

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/codegentest"
	"zeroparse.dev/zp/internal/engine"
)

// The engine and the generated Go parsers report the calls of a parse with the same events and add them
// to the same profiles. The tests here compare the two producers on every route that both have: Parse,
// Recognize, ParseStream and the parse of a Document after an edit, in both position units, against each
// engine backend.

// callsAgree is the relation between the events of two producers of the same parse that the section
// "Reported Calls" of the specification of tracing and profiling defines: the producers agree on every
// call that can match and may differ only in calls that cannot match, that is, calls skipped by dispatch
// or failing within the first character. So the events of a and b must pair up, with every field equal,
// except for:
//
//   - a call that one producer makes and the other does not, which must fail having examined no more
//     than the character at its position, and is set aside with the calls within it;
//   - a skipped event of b (only generated parsers report them; an engine never does), which must have
//     the reason SkipFirstCharacter and pair with a call of a of the same rule, position, level and depth
//     that fails within its first character, or with no call of a at all.
//
// It returns the calls it set aside and the first difference.
func callsAgree(a, b []core.TraceEvent, in string, unit core.PositionUnit) (aside setAside, msg string) {
	subtree := func(evs []core.TraceEvent, i int) (end int, exit core.TraceEvent, ok bool) {
		depth := 0
		for k := i; k < len(evs); k++ {
			switch evs[k].Kind {
			case core.TraceEnter:
				depth++
			case core.TraceExit:
				if depth--; depth == 0 {
					return k + 1, evs[k], true
				}
			}
		}
		return len(evs), core.TraceEvent{}, false
	}
	// cannotMatch reports whether the call that begins at evs[i] fails within its first character, and
	// returns the end of its events.
	cannotMatch := func(evs []core.TraceEvent, i int) (int, bool) {
		end, exit, ok := subtree(evs, i)
		return end, ok && !exit.Matched && exit.Examined <= firstCharEnd(in, evs[i].Pos, unit)
	}
	sameCall := func(x, y core.TraceEvent) bool {
		return x.Rule == y.Rule && x.Pos == y.Pos && x.Level == y.Level && x.Depth == y.Depth
	}
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		if i < len(a) && a[i].Kind == core.TraceSkipped {
			return aside, fmt.Sprintf("the first producer reports a skipped call of %s@%d", a[i].Rule, a[i].Pos)
		}
		if j < len(b) && b[j].Kind == core.TraceSkipped {
			s := b[j]
			if s.SkipReason != core.SkipFirstCharacter {
				return aside, fmt.Sprintf("skipped call of %s@%d with the reason %q", s.Rule, s.Pos, s.SkipReason)
			}
			if i < len(a) && a[i].Kind == core.TraceEnter && sameCall(a[i], s) {
				end, ok := cannotMatch(a, i)
				if !ok {
					return aside, fmt.Sprintf("the second producer skips %s@%d (level %d, depth %d), a call the first makes that does not fail within its first character",
						s.Rule, s.Pos, s.Level, s.Depth)
				}
				aside.add(a[i:end], &aside.a)
				aside.paired++
				i = end
			} else {
				aside.unpaired++
			}
			aside.b = append(aside.b, s)
			j++
			continue
		}
		if i < len(a) && j < len(b) && a[i].Kind == b[j].Kind && sameCall(a[i], b[j]) {
			x, y := a[i], b[j]
			if x.Kind == core.TraceEnter {
				if x.Lookahead != y.Lookahead {
					return aside, fmt.Sprintf("lookahead of the call of %s@%d: %v, %v", x.Rule, x.Pos, x.Lookahead, y.Lookahead)
				}
			} else {
				if x.Matched != y.Matched || x.End != y.End || x.Memo != y.Memo || x.Evals != y.Evals || x.Lookahead != y.Lookahead || x.ReusedElements != y.ReusedElements {
					return aside, fmt.Sprintf("exit of %s@%d: %s, %s", x.Rule, x.Pos, exitFields(x), exitFields(y))
				}
				if x.Examined != y.Examined {
					return aside, fmt.Sprintf("examined end of %s@%d: %d, %d", x.Rule, x.Pos, x.Examined, y.Examined)
				}
			}
			aside.compared++
			i++
			j++
			continue
		}
		if i < len(a) && a[i].Kind == core.TraceEnter {
			if end, ok := cannotMatch(a, i); ok {
				aside.add(a[i:end], &aside.a)
				i = end
				continue
			}
		}
		if j < len(b) && b[j].Kind == core.TraceEnter {
			if end, ok := cannotMatch(b, j); ok {
				aside.add(b[j:end], &aside.b)
				j = end
				continue
			}
		}
		describe := func(evs []core.TraceEvent, k int) string {
			if k >= len(evs) {
				return "the end"
			}
			e := evs[k]
			return fmt.Sprintf("%v of %s@%d (level %d, depth %d)", e.Kind, e.Rule, e.Pos, e.Level, e.Depth)
		}
		return aside, fmt.Sprintf("the first producer has %s, the second %s", describe(a, i), describe(b, j))
	}
	return aside, ""
}

func exitFields(e core.TraceEvent) string {
	return fmt.Sprintf("{end %d matched %v memo %v evals %d lookahead %v reused %d}", e.End, e.Matched, e.Memo, e.Evals, e.Lookahead, e.ReusedElements)
}

// setAside is what callsAgree set aside: the events of the calls only one producer made (a, b), the
// skipped events of b among b, and counts.
type setAside struct {
	a, b                       []core.TraceEvent
	compared, paired, unpaired int
}

func (s *setAside) add(evs []core.TraceEvent, to *[]core.TraceEvent) { *to = append(*to, evs...) }

// profileCounts are the counts of a profile that the calls set aside by callsAgree change by known amounts.
type profileCounts struct {
	Entered, Calls, Evals, MemoHits, Matched, Failed, Consumed, Examined, Wasted, ReusedElements int
}

// profileMinus returns the counts of s, totals under the key "", with those of the calls of aside taken
// out, as Profile counts them (TraceSkipped events are not counted here). Rows left with no count are
// dropped.
func profileMinus(s core.ProfileSnapshot, aside []core.TraceEvent) map[string]profileCounts {
	out := map[string]profileCounts{"": {Entered: s.Entered, Calls: s.Calls, Evals: s.Evals, MemoHits: s.MemoHits, Examined: s.Examined, ReusedElements: s.ReusedElements}}
	for _, r := range s.Rules {
		out[r.Rule] = profileCounts{r.Entered, r.Calls, r.Evals, r.MemoHits, r.Matched, r.Failed, r.Consumed, r.Examined, r.Wasted, r.ReusedElements}
	}
	for _, e := range aside {
		row, total := out[e.Rule], out[""]
		switch e.Kind {
		case core.TraceEnter:
			row.Entered--
			total.Entered--
		case core.TraceExit:
			examined := max(e.Examined-e.Pos, 0)
			row.Calls--
			total.Calls--
			row.Evals -= e.Evals
			total.Evals -= e.Evals
			row.Examined -= examined
			row.ReusedElements -= e.ReusedElements
			if e.Memo {
				row.MemoHits--
				total.MemoHits--
			}
			if e.Matched {
				row.Matched--
				row.Consumed -= e.End - e.Pos
			} else {
				row.Failed--
				if !e.Memo {
					row.Wasted -= examined
				}
			}
			if e.Depth == 1 {
				total.Examined -= examined
				total.ReusedElements -= e.ReusedElements
			}
		}
		out[e.Rule], out[""] = row, total
	}
	for k, c := range out {
		if k != "" && c == (profileCounts{}) {
			delete(out, k)
		}
	}
	return out
}

// profilesAgree compares the profiles of the two producers of a parse once the calls that callsAgree
// set aside are taken out, and returns the first difference.
func profilesAgree(a, b core.ProfileSnapshot, aside setAside) string {
	if a.Parses != b.Parses || a.Parses > 1 {
		return fmt.Sprintf("parses %d, %d", a.Parses, b.Parses)
	}
	if a.Skipped != 0 {
		return fmt.Sprintf("the first producer counts %d skipped calls", a.Skipped)
	}
	skipped := 0
	for _, e := range aside.b {
		if e.Kind == core.TraceSkipped {
			skipped++
		}
	}
	if b.Skipped != skipped {
		return fmt.Sprintf("the second producer counts %d skipped calls and reports %d", b.Skipped, skipped)
	}
	x, y := profileMinus(a, aside.a), profileMinus(b, aside.b)
	for k, c := range x {
		if y[k] != c {
			return fmt.Sprintf("profile row %q: %+v, %+v", k, c, y[k])
		}
	}
	for k, c := range y {
		if _, ok := x[k]; !ok {
			return fmt.Sprintf("profile row %q: none, %+v", k, c)
		}
	}
	return ""
}

// agreeProbeMain is the consumer of the generated packages: for each input, unit and entry it prints one
// JSON report with the events and the profile (without times) of an observed parse, and its error. The
// document entry parses a new Document of the input, observed; the edit entry parses it, replaces the
// character in its middle with itself and parses it again, observed.
const agreeProbeMain = `package main

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
	run  func(string, core.PositionUnit, ...zprt.ParseOption) error
}

var entries = [][]entry{
ENTRIES}

type oneByte struct{ s string }

func (r *oneByte) Read(b []byte) (int, error) {
	if r.s == "" {
		return 0, io.EOF
	}
	b[0], r.s = r.s[0], r.s[1:]
	return 1, nil
}

// middle returns the position and the end of the character in the middle of in, and the character.
func middle(in string, unit core.PositionUnit) (int, int, string) {
	r := []rune(in)
	k := len(r) / 2
	if k == len(r) {
		return 0, 0, ""
	}
	if unit == core.Bytes {
		pos := len(string(r[:k]))
		return pos, pos + len(string(r[k])), string(r[k])
	}
	return k, k + 1, string(r[k])
}

// untimed returns the snapshot of prof without its times.
func untimed(prof *core.Profile) core.ProfileSnapshot {
	snap := prof.Snapshot()
	snap.Time = 0
	for k := range snap.Rules {
		snap.Rules[k].Time, snap.Rules[k].SelfTime = 0, 0
	}
	return snap
}

func main() {
	var inputs [][][]byte
	data, _ := os.ReadFile(os.Args[1])
	if err := json.Unmarshal(data, &inputs); err != nil {
		panic(err)
	}
	for i := range entries {
		for j, in := range inputs[i] {
			for _, unit := range []core.PositionUnit{core.CodePoints, core.Bytes} {
				for _, e := range entries[i] {
					// Two trace functions and two profiles, each of which must receive the whole parse.
					var events, second []core.TraceEvent
					var prof, prof2 core.Profile
					err := e.run(string(in), unit, zprt.WithTrace(func(ev core.TraceEvent) { events = append(events, ev) }), zprt.WithProfile(&prof),
						zprt.WithTrace(func(ev core.TraceEvent) { second = append(second, ev) }), zprt.WithProfile(&prof2))
					// The same parse with a trace filter and a depth limit: the filter names a rule called
					// within the start rule, if any, and a name that is not a rule.
					filter := []string{"main", "no such rule"}
					for _, ev := range events {
						if ev.Depth == 2 {
							filter[0] = ev.Rule
							break
						}
					}
					var filtered []core.TraceEvent
					var filteredProf core.Profile
					e.run(string(in), unit, zprt.WithTraceFilter("main"), zprt.WithTraceFilter(filter...), zprt.WithTraceMaxDepth(1), zprt.WithTraceMaxDepth(MAXDEPTH),
						zprt.WithTrace(func(ev core.TraceEvent) { filtered = append(filtered, ev) }), zprt.WithProfile(&filteredProf))
					b, _ := json.Marshal(struct {
						Case, Input int
						Bytes       bool
						Entry       string
						Events      []core.TraceEvent
						Profile     core.ProfileSnapshot
						Second      []core.TraceEvent
						Profile2    core.ProfileSnapshot
						Filter      []string
						Filtered    []core.TraceEvent
						FilteredProfile core.ProfileSnapshot
						Err         string
					}{i, j, unit == core.Bytes, e.name, events, untimed(&prof), second, untimed(&prof2), filter, filtered, untimed(&filteredProf), fmt.Sprint(err)})
					fmt.Println(string(b))
				}
			}
		}
	}
}
`

// agreeReport is one report of the consumer: the events and the profile of the parse, those of the
// second trace function and profile given to it (Second, Profile2), and those of the same parse with the
// trace filter Filter and the depth limit agreeMaxDepth (Filtered, FilteredProfile).
type agreeReport struct {
	Case, Input     int
	Bytes           bool
	Entry           string
	Events          []core.TraceEvent
	Profile         core.ProfileSnapshot
	Second          []core.TraceEvent
	Profile2        core.ProfileSnapshot
	Filter          []string
	Filtered        []core.TraceEvent
	FilteredProfile core.ProfileSnapshot
	Err             string
}

// agreeLibPatchEnv names the environment variable through which the mutants of
// TestEngineObservationMutants change the library of the generated parsers: a JSON object with the
// file of the library (relative to the module root) and the text to replace (Old) with New. The go
// command's overlay does not reach the copy of the library that the probes import.
const agreeLibPatchEnv = "ZP_AGREE_LIB_PATCH"

// agreeLib returns the library that the probes import: the copy of the library, changed as
// agreeLibPatchEnv says if it is set.
func agreeLib(t *testing.T) string {
	t.Helper()
	spec := os.Getenv(agreeLibPatchEnv)
	if spec == "" {
		return codegentest.LibCopy()
	}
	var m struct{ File, Old, New string }
	if err := json.Unmarshal([]byte(spec), &m); err != nil {
		t.Fatalf("%s: %v", agreeLibPatchEnv, err)
	}
	dir, name := filepath.Split(filepath.FromSlash(m.File))
	if filepath.ToSlash(filepath.Clean(dir)) != "codegen/go/lib" {
		t.Fatalf("%s: %s is not a file of the library", agreeLibPatchEnv, m.File)
	}
	patched := 0
	lib := codegentest.LibCopyWith(t, func(file, src string) string {
		if file != name {
			return src
		}
		patched += strings.Count(src, m.Old)
		return strings.Replace(src, m.Old, m.New, 1)
	}, nil)
	if patched != 1 {
		t.Fatalf("%s: %q occurs %d times in %s", agreeLibPatchEnv, m.Old, patched, m.File)
	}
	return lib
}

// agreeMaxDepth is the depth limit of the filtered parses of the agreement.
const agreeMaxDepth = 3

// runAgreeProbes builds the cases with observability, the recognizer, ParseStream where the start rule
// has a #stream repetition and Document where it can, and returns the reports of the consumer.
func runAgreeProbes(t *testing.T, cases []codegentest.Case) []agreeReport {
	t.Helper()
	dir := t.TempDir()
	write := func(name string, content []byte) {
		path := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", []byte(codegentest.GoModAt("agreeprobe", agreeLib(t))))
	var imports, entries strings.Builder
	var inputs [][][]byte
	for i, c := range cases {
		g, err := parseTest(c.Src)
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		pkg := fmt.Sprintf("f%d", i)
		// A Document of a grammar with actions keeps its types in a package of their own.
		o := codegen.GenOptions{Start: "main", Recognize: true, Observability: true, Stream: true, Document: true, Module: "agreeprobe/" + pkg}
		if _, err := codegentest.GenerateCode(t, g, o); err != nil {
			if !strings.Contains(err.Error(), "needs a #stream repetition") {
				t.Fatalf("%s: %v", c.Name, err)
			}
			o.Stream = false
		}
		codegentest.GeneratePackageIn(t, dir, "agreeprobe", pkg, g, o)
		fmt.Fprintf(&imports, "\tf%d \"agreeprobe/f%d\"\n", i, i)
		fmt.Fprintf(&entries, `	{
		{"parse", func(in string, u core.PositionUnit, o ...zprt.ParseOption) error {
			_, err := f%[1]d.Parse(in, append(o, zprt.WithUnit(u))...)
			return err
		}},
		{"recognize", func(in string, u core.PositionUnit, o ...zprt.ParseOption) error { return f%[1]d.Recognize(in, append(o, zprt.WithUnit(u))...) }},
		{"document", func(in string, u core.PositionUnit, o ...zprt.ParseOption) error {
			d, err := f%[1]d.NewDocument(in, zprt.WithUnit(u))
			if err != nil {
				return err
			}
			_, err = d.Parse(o...)
			return err
		}},
		{"edit", func(in string, u core.PositionUnit, o ...zprt.ParseOption) error {
			d, err := f%[1]d.NewDocument(in, zprt.WithUnit(u))
			if err != nil {
				return err
			}
			d.Parse()
			pos, end, ch := middle(in, u)
			if err := d.Edit(pos, end, ch); err != nil {
				return err
			}
			_, err = d.Parse(o...)
			return err
		}},
`, i)
		if o.Stream {
			fmt.Fprintf(&entries, `		{"stream", func(in string, u core.PositionUnit, o ...zprt.ParseOption) error {
			return f%d.ParseStream(&oneByte{in}, func(*core.Node) error { return nil }, append(o, zprt.WithUnit(u))...)
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
	write("main.go", []byte(strings.NewReplacer("IMPORTS", imports.String(), "ENTRIES", entries.String(), "MAXDEPTH", fmt.Sprint(agreeMaxDepth)).Replace(agreeProbeMain)))
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
	var reports []agreeReport
	for _, l := range strings.Split(strings.TrimSuffix(string(out), "\n"), "\n") {
		var r agreeReport
		if err := json.Unmarshal([]byte(l), &r); err != nil {
			t.Fatalf("go run: malformed report %q: %v", l, err)
		}
		reports = append(reports, r)
	}
	return reports
}

// middleChar is the middle function of agreeProbeMain.
func middleChar(in string, unit core.PositionUnit) (int, int, string) {
	r := []rune(in)
	k := len(r) / 2
	if k == len(r) {
		return 0, 0, ""
	}
	if unit == core.Bytes {
		pos := len(string(r[:k]))
		return pos, pos + len(string(r[k])), string(r[k])
	}
	return k, k + 1, string(r[k])
}

// engineObservation runs the parse of the report r on the engine with the backend b, the trace filter
// rules and the depth limit maxDepth, and returns its events, its profile and its error.
func engineObservation(t *testing.T, prog *engine.Program, r agreeReport, in string, b engine.Backend, rules []string, maxDepth int) ([]core.TraceEvent, core.ProfileSnapshot, error) {
	t.Helper()
	var events []core.TraceEvent
	var prof core.Profile
	o := engine.ParseOptions{Expectations: true, Backend: b, Unit: core.CodePoints, Recognize: r.Entry == "recognize"}
	if r.Bytes {
		o.Unit = core.Bytes
	}
	traced := o
	traced.Trace = func(e core.TraceEvent) { events = append(events, e) }
	traced.Profiles = []*core.Profile{&prof}
	if rules != nil || maxDepth != 0 {
		traced.TraceSelection = &engine.TraceSelection{Rules: rules, MaxDepth: maxDepth}
	}
	var err error
	switch r.Entry {
	case "parse", "recognize":
		_, err = prog.ParseWith("main", in, traced)
	case "stream":
		err = prog.ParseStreamWith("main", &oneByteReader{in}, func(*core.Node) error { return nil }, traced)
	case "document", "edit":
		d, derr := prog.NewDocumentWith("main", in, o)
		if derr != nil {
			return nil, core.ProfileSnapshot{}, derr
		}
		if r.Entry == "edit" {
			d.Parse()
			pos, end, ch := middleChar(in, o.Unit)
			if err := d.Edit(pos, end, ch); err != nil {
				return nil, core.ProfileSnapshot{}, err
			}
		}
		_, err = d.ParseWith(traced)
	default:
		t.Fatalf("unknown entry %q", r.Entry)
	}
	snap := prof.Snapshot()
	snap.Time = 0
	for k := range snap.Rules {
		snap.Rules[k].Time, snap.Rules[k].SelfTime = 0, 0
	}
	return events, snap, err
}

// sameCallExamined is the relation between the events of two producers of a Document parse after an
// edit that the section "Reported Calls" of the specification of tracing and profiling defines: the
// engine may resume repetitions and reuse failed results, so the producers report different calls, but
// of the calls with the same rule, position, level, depth, end and outcome, if both make any, both make
// the same number, they pair one to one in the order they are made, and paired calls have the same
// Examined. A producer can make such a call more
// than once with different examined ends, within the growth of a left-recursive seed, because Examined
// depends on the seed, which those fields leave out. It returns the number of exits compared and the
// differences.
func sameCallExamined(a, b []core.TraceEvent) (compared int, diffs []string) {
	type key struct {
		rule                   string
		level, depth, pos, end int
		matched                bool
	}
	ends := func(evs []core.TraceEvent) (map[key][]int, []key) {
		m, order := map[key][]int{}, []key(nil)
		for _, e := range evs {
			if e.Kind == core.TraceExit {
				k := key{e.Rule, e.Level, e.Depth, e.Pos, e.End, e.Matched}
				if m[k] == nil {
					order = append(order, k)
				}
				m[k] = append(m[k], e.Examined)
			}
		}
		return m, order
	}
	ea, order := ends(a)
	eb, _ := ends(b)
	for _, k := range order {
		x, y := ea[k], eb[k]
		if y == nil {
			continue
		}
		compared += min(len(x), len(y))
		if !slices.Equal(x, y) {
			diffs = append(diffs, fmt.Sprintf("examined ends of %s@%d (level %d, depth %d, end %d, matched %v) in the order made: %v, %v", k.rule, k.pos, k.level, k.depth, k.end, k.matched, x, y))
		}
	}
	return compared, diffs
}

// swapGrowthExamined returns a copy of evs in which the examined ends of the first two exits of one call
// (the same rule, position, level, depth, end and outcome) that differ are swapped, or nil if no call
// has two. Such exits are steps of the growth of a left-recursive seed.
func swapGrowthExamined(evs []core.TraceEvent) []core.TraceEvent {
	type key struct {
		rule                   string
		level, depth, pos, end int
		matched                bool
	}
	first := map[key]int{}
	for i, e := range evs {
		if e.Kind != core.TraceExit {
			continue
		}
		k := key{e.Rule, e.Level, e.Depth, e.Pos, e.End, e.Matched}
		j, ok := first[k]
		if !ok {
			first[k] = i
			continue
		}
		if evs[j].Examined != e.Examined {
			out := slices.Clone(evs)
			out[i].Examined, out[j].Examined = out[j].Examined, out[i].Examined
			return out
		}
	}
	return nil
}

// oneByteReader returns one byte per Read, as the consumer's reader does.
type oneByteReader struct{ s string }

func (r *oneByteReader) Read(b []byte) (int, error) {
	if r.s == "" {
		return 0, io.EOF
	}
	b[0], r.s = r.s[0], r.s[1:]
	return 1, nil
}

// agreementStats counts what agreementMismatches compared, by entry.
type agreementStats struct {
	reports, compared, paired, unpaired, engineOnly, generatedOnly int
	// swaps counts the edit observations whose growth steps were swapped (swapGrowthExamined) to check
	// that sameCallExamined tells them apart.
	swaps int
	// filtered counts the events of the engine's filtered parses; partial, the parses whose filter and
	// depth limit left out some events but not all.
	filtered, partial int
}

// optionsHonored checks the observation of one producer with several trace functions and profiles and
// with a trace filter: the second trace function and profile (second, prof2) received the parse as the
// first did (events, prof), the filtered parse delivered the events that the filter rules and the depth
// limit agreeMaxDepth select (codegentest.SelectedEvents), and its profile recorded the whole parse. It
// returns the first difference.
func optionsHonored(producer string, events []core.TraceEvent, prof core.ProfileSnapshot, second []core.TraceEvent, prof2 core.ProfileSnapshot,
	rules []string, filtered []core.TraceEvent, filteredProf core.ProfileSnapshot) string {
	same := func(a, b any) bool {
		x, _ := json.Marshal(a)
		y, _ := json.Marshal(b)
		return string(x) == string(y)
	}
	switch want := codegentest.SelectedEvents(events, rules, agreeMaxDepth); {
	case !same(events, second):
		return fmt.Sprintf("%s: the second trace function received %d events, the first %d", producer, len(second), len(events))
	case !same(prof, prof2):
		return producer + ": the second profile differs from the first"
	case !same(want, filtered):
		return fmt.Sprintf("%s: the trace filter %q with depth limit %d delivered %d events, want %d", producer, rules, agreeMaxDepth, len(filtered), len(want))
	case !same(prof, filteredProf):
		return producer + ": the profile depends on the trace filter"
	}
	return ""
}

// agreementMismatches checks each report against the same parse on each engine backend with callsAgree
// and profilesAgree, and returns the differences and what it compared, by entry.
func agreementMismatches(t *testing.T, cases []codegentest.Case, reports []agreeReport) (map[string][]string, map[string]*agreementStats) {
	t.Helper()
	diffs, stats := map[string][]string{}, map[string]*agreementStats{}
	progs := map[int]*engine.Program{}
	for _, r := range reports {
		c := cases[r.Case]
		in := c.Inputs[r.Input]
		prog := progs[r.Case]
		if prog == nil {
			prog = mustCompile(t, c.Src)
			progs[r.Case] = prog
		}
		unit := core.CodePoints
		if r.Bytes {
			unit = core.Bytes
		}
		st := stats[r.Entry]
		if st == nil {
			st = &agreementStats{}
			stats[r.Entry] = st
		}
		st.reports++
		where := fmt.Sprintf("%s %q bytes=%v %s", c.Name, in, r.Bytes, r.Entry)
		if msg := optionsHonored("generated", r.Events, r.Profile, r.Second, r.Profile2, r.Filter, r.Filtered, r.FilteredProfile); msg != "" {
			diffs[r.Entry] = append(diffs[r.Entry], where+": "+msg)
		}
		for _, b := range []engine.Backend{engine.Closure, engine.Bytecode, engine.BytecodeIterative} {
			events, snap, err := engineObservation(t, prog, r, in, b, nil, 0)
			where := fmt.Sprintf("%s %v", where, b)
			if (err == nil) != (r.Err == "<nil>") {
				diffs[r.Entry] = append(diffs[r.Entry], fmt.Sprintf("%s: engine error %v, generated %s", where, err, r.Err))
				continue
			}
			filtered, filteredSnap, _ := engineObservation(t, prog, r, in, b, r.Filter, agreeMaxDepth)
			if msg := optionsHonored("engine", events, snap, events, snap, r.Filter, filtered, filteredSnap); msg != "" {
				diffs[r.Entry] = append(diffs[r.Entry], where+": "+msg)
				continue
			}
			st.filtered += len(filtered)
			if len(filtered) > 0 && len(filtered) < len(events) {
				st.partial++
			}
			if r.Entry == "edit" {
				n, d := sameCallExamined(events, r.Events)
				st.compared += n
				for _, msg := range d {
					diffs[r.Entry] = append(diffs[r.Entry], where+": "+msg)
				}
				if swapped := swapGrowthExamined(events); swapped != nil && len(d) == 0 {
					st.swaps++
					if _, d := sameCallExamined(swapped, r.Events); len(d) == 0 {
						diffs[r.Entry] = append(diffs[r.Entry], where+": swapped examined ends of growth steps not told apart")
					}
				}
				continue
			}
			aside, msg := callsAgree(events, r.Events, in, unit)
			if msg == "" {
				msg = profilesAgree(snap, r.Profile, aside)
			}
			if msg == "" {
				if _, m := callsAgree(filtered, r.Filtered, in, unit); m != "" {
					msg = "filtered: " + m
				}
			}
			if msg != "" {
				diffs[r.Entry] = append(diffs[r.Entry], where+": "+msg)
				continue
			}
			st.compared += aside.compared
			st.paired += aside.paired
			st.unpaired += aside.unpaired
			for _, e := range aside.a {
				if e.Kind == core.TraceEnter {
					st.engineOnly++
				}
			}
			for _, e := range aside.b {
				if e.Kind == core.TraceEnter {
					st.generatedOnly++
				}
			}
		}
	}
	return diffs, stats
}

// TestEngineObservationsAgreeWithGenerated checks the events and profiles of every engine backend against
// those of the generated parsers (callsAgree, profilesAgree) on every route both have, in both units, and
// that both producers honor several trace functions and profiles and the trace filter and depth limit
// (optionsHonored).
func TestEngineObservationsAgreeWithGenerated(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds generated consumers")
	}
	codegentest.NeedsBudgetControls(t)
	cases := examinedCases(t)
	diffs, stats := agreementMismatches(t, cases, runAgreeProbes(t, cases))
	for _, entry := range []string{"parse", "recognize", "stream", "document", "edit"} {
		st := stats[entry]
		if st == nil || st.compared == 0 {
			t.Errorf("no event of %s compared", entry)
			st = &agreementStats{}
		}
		t.Logf("%s: %d reports, %d events compared; %d skipped events paired, %d unpaired; %d calls of the engine only, %d of the generated parser only",
			entry, st.reports, st.compared, st.paired, st.unpaired, st.engineOnly, st.generatedOnly)
		t.Logf("%s: %d filtered engine events; %d parses filtered in part", entry, st.filtered, st.partial)
		if st.partial == 0 {
			t.Errorf("no parse of %s was filtered in part", entry)
		}
		if entry == "edit" {
			if st.swaps == 0 {
				t.Error("no edit observation had growth steps to swap")
			}
			t.Logf("edit: %d observations with swapped growth steps told apart", st.swaps)
		}
		for i, msg := range diffs[entry] {
			if i == 10 {
				t.Errorf("%d more differences of %s", len(diffs[entry])-i, entry)
				break
			}
			t.Error(msg)
		}
	}
}
