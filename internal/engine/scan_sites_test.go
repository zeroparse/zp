package engine

import (
	"fmt"
	"io"
	"math/rand"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/iotest"

	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/engine/core"
	"zeroparse.dev/zp/internal/gentest"
	"zeroparse.dev/zp/runtime/parse"
)

// scanSiteGrammars are the grammars with scan sites of the tests of all targets.
var scanSiteGrammars = gentest.ScanSiteGrammars

// scanSiteInputs are random inputs over alphabet and inputs with long runs that the walks restart at.
func scanSiteInputs(alphabet string, rng *rand.Rand) []string {
	return gentest.ScanSiteInputs(alphabet, 300, rng)
}

// TestScanSitesMatchParsesWithoutRecords checks that the records of scan sites change no result:
// values and syntax errors with their expectations, against walks without records and a program
// compiled without scan sites, and that the calls reach the records.
func TestScanSitesMatchParsesWithoutRecords(t *testing.T) {
	atHotSpans(t, scanSitesMatchWithoutRecords)
}

func scanSitesMatchWithoutRecords(t *testing.T) {
	rng := rand.New(rand.NewSource(131))
	for _, g := range scanSiteGrammars {
		prog := mustCompile(t, g.Src)
		if prog.NumScanSites == 0 {
			t.Errorf("%s: no scan sites", g.Name)
		}
		plain := compileWithoutScanSites(t, g.Src)
		inputs := scanSiteInputs(g.Alphabet, rng)
		hits := core.ScanHits.Load()
		for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
			hitsB := core.ScanHits.Load()
			if diffs := skipRunMismatches(t, prog, inputs, b); len(diffs) > 0 {
				t.Errorf("%s, %s: %d parses differ without records, first:\n%s", g.Name, b, len(diffs), diffs[0])
			}
			for _, in := range inputs {
				for _, exp := range []bool{true, false} {
					o := ParseOptions{Backend: b, Expectations: exp}
					n, err := prog.ParseWith("main", in, o)
					wn, werr := plain.ParseWith("main", in, o)
					if got, want := skipDump(t, n, err), skipDump(t, wn, werr); got != want {
						t.Fatalf("%s, %s: input %q\n got  %s\n want %s", g.Name, b, in, got, want)
					}
				}
			}
			if core.ScanHotSpan <= 2 && core.ScanHits.Load() == hitsB {
				t.Errorf("%s, %s: no call was answered from a record", g.Name, b)
			}
		}
		if core.ScanHotSpan <= 2 && core.ScanHits.Load() == hits {
			t.Errorf("%s: no call was answered from a record", g.Name)
		}
	}
}

// TestScanSitesAreMarked checks which repetitions are scan sites: those of one character without a
// maximum in the rules that call no rule, whatever the rule does with their values.
func TestScanSitesAreMarked(t *testing.T) {
	for _, c := range []struct {
		name, src string
		sites     int
	}{
		{"a pure rule called twice", "rule main = r \"a\" / r \"b\"\nrule r = [x]*", 1},
		{"a pure rule called once", "rule main = r \"a\"\nrule r = [x]*", 1},
		{"a rule that calls a rule", "rule main = r \"a\" / r \"b\"\nrule r = s*\nrule s = \"x\"", 0},
		{"a rule with a capture", "rule main = r \"a\" / r \"b\"\nrule r = c:[x]*", 1},
		{"a rule with an action", "rule main = r \"a\" / r \"b\"\nrule r = [x]* => 1", 1},
		{"a rule with a predicate", "var n int\nrule main = r \"a\" / r \"b\"\nrule r = ([x] { n < 3 })*", 0},
		{"a rule with a lookahead", "rule main = r \"a\" / r \"b\"\nrule r = (![y] [x])*", 0},
		{"a rule with a pragma", "rule main = r \"a\" / r \"b\"\nrule r = [x]* #atomic", 1},
		{"repetitions in the calling rule", "rule main = [x]* \"a\" [y]* / [x]+ \"b\"", 3},
		{"a repetition of a literal", "rule main = r \"a\"\nrule r = \"x\"*", 0},
		{"a repetition with a maximum", "rule main = r \"a\"\nrule r = [x]#repeat(0,9)", 0},
		{"a repetition of a choice", "rule main = r \"a\"\nrule r = (\"x\" / \"yz\")*", 0},
		{"a repetition in a stream", "rule main = [x]* #stream", 0},
	} {
		if n := mustCompile(t, c.src).NumScanSites; n != c.sites {
			t.Errorf("%s: %d scan sites, want %d", c.name, n, c.sites)
		}
	}
}

// TestScanSitesReuseRecords checks that a parse that restarts a scan at one position in each level
// of a recursion fits a budget that is linear in the input with records, and exceeds it without them.
func TestScanSitesReuseRecords(t *testing.T) {
	if !core.BudgetControlsEnabled {
		t.Skip("budget controls are unavailable in the reference build")
	}
	defer func() { core.SkipRecordsOff = false }()
	prog := mustCompile(t, scanSiteGrammars[2].Src)
	in := strings.Repeat("(", 400) + strings.Repeat(" ", 400)
	o := ParseOptions{Backend: Closure, Limits: parse.Limits{Compute: 200_000}}
	core.SkipRecordsOff = false
	if _, err := prog.ParseWith("main", in, o); err != nil {
		t.Errorf("the parse with records: %v", err)
	}
	core.SkipRecordsOff = true
	if _, err := prog.ParseWith("main", in, o); err == nil {
		t.Errorf("the parse without records fits the budget")
	}
}

// TestScanSitesInDocuments checks that a Document parse after edits inside and next to a run that a
// record covers gives the tree and errors of a fresh parse of the text.
func TestScanSitesInDocuments(t *testing.T) {
	defer func(n int) { core.ScanHotSpan = n }(core.ScanHotSpan)
	core.ScanHotSpan = 2
	hits := core.ScanHits.Load()
	for _, g := range scanSiteGrammars {
		for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
			if d := scanDocumentMismatch(t, g.Src, g.Alphabet, b); d != "" {
				t.Errorf("%s, %s: %s", g.Name, b, d)
			}
		}
	}
	if core.ScanHits.Load() == hits {
		t.Errorf("no call was answered from a record")
	}
}

// scanDocumentMismatch edits Documents of the grammar src and returns the first difference from a
// fresh parse of the edited text.
func scanDocumentMismatch(t *testing.T, src, alphabet string, b Backend) string {
	t.Helper()
	rng := rand.New(rand.NewSource(133))
	prog := mustCompile(t, src)
	var pool []string
	for _, in := range scanSiteInputs(alphabet, rng) {
		if len(in) >= 40 {
			pool = append(pool, in)
		}
	}
	for i := 0; i < 40; i++ {
		o := ParseOptions{Backend: b, Expectations: true}
		d, err := prog.NewDocumentWith("main", pool[rng.Intn(len(pool))], o)
		if err != nil {
			t.Fatal(err)
		}
		d.Parse()
		for j := 0; j < 6; j++ {
			cur := d.Text()
			a := rng.Intn(len(cur) + 1)
			b, ins := a, ""
			switch rng.Intn(3) {
			case 0:
				b = min(len(cur), a+1+rng.Intn(3))
			case 1:
				ins = string(alphabet[rng.Intn(len(alphabet))])
			default:
				b = min(len(cur), a+rng.Intn(3))
				ins = string(alphabet[rng.Intn(len(alphabet))])
			}
			if err := d.Edit(a, b, ins); err != nil {
				t.Fatalf("edit: %v", err)
			}
			n, err := d.Parse()
			wn, werr := prog.ParseWith("main", d.Text(), o)
			if got, want := skipDump(t, n, err), skipDump(t, wn, werr); got != want {
				return "text " + d.Text() + "\n got  " + got + "\n want " + want
			}
		}
	}
	return ""
}

// TestScanSitesNegativeControls checks that the budget test sees a record that is not reused: one
// that keeps the position of its walk and one that is never made. The record is a shortcut that the
// walk completes: the iteration after the jump fails at the end and examines the input there, so a
// record without its end only costs the walk again, and one without its examined range examines the
// same input. Neither changes a result, and the budget is where they show.
func TestScanSitesNegativeControls(t *testing.T) {
	if !core.BudgetControlsEnabled {
		t.Skip("budget controls are unavailable in the reference build")
	}
	defer func() { core.SkipRecordMutation = 0 }()
	prog := mustCompile(t, scanSiteGrammars[2].Src)
	in := strings.Repeat("(", 400) + strings.Repeat(" ", 400)
	o := ParseOptions{Backend: Closure, Limits: parse.Limits{Compute: 200_000}}
	for _, m := range []struct {
		name string
		mut  int
	}{{"end", core.SkipKeepPos}, {"making", core.SkipNeverRecord}} {
		core.SkipRecordMutation = m.mut
		if _, err := prog.ParseWith("main", in, o); err == nil {
			t.Errorf("a record without its %s goes unseen", m.name)
		}
	}
}

// TestScanSitesInStreams checks that a stream that delivers a run of a scan site in pieces, so
// that a call ends where the available input does, emits what a parse of the whole text builds.
func TestScanSitesInStreams(t *testing.T) {
	defer func(n int) { core.ScanHotSpan = n }(core.ScanHotSpan)
	core.ScanHotSpan = 2
	prog := mustCompile(t, `rule main = row* #stream $$
rule row = ws "x" ws ";" / ws "x" ws "!" / ws "y" ws ";"
rule ws = [ ]*`)
	for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
		scanSitesInStreams(t, prog, b)
	}
}

// scanSitesInStreams parses streams of random rows split by readers that deliver the text in small
// pieces and compares them with the unrecorded parse of the stream, on the backend b. A walk that
// reaches the end of the loaded input must not record, and the others may, so some parse takes a
// record. On the bytecode backends the walks of ws build lists, which the valued ITER serves.
func scanSitesInStreams(t *testing.T, prog *Program, b Backend) {
	t.Helper()
	hits := core.ScanHits.Load()
	defer func() {
		if core.ScanHits.Load() == hits {
			t.Errorf("%s: no stream parse was answered from a record", b)
		}
	}()
	rng := rand.New(rand.NewSource(135))
	for i := 0; i < 200; i++ {
		var sb strings.Builder
		for j := 0; j < 1+rng.Intn(5); j++ {
			sb.WriteString(strings.Repeat(" ", rng.Intn(50)))
			sb.WriteString(string("xy"[rng.Intn(2)]))
			sb.WriteString(strings.Repeat(" ", rng.Intn(50)))
			sb.WriteString(string(";!"[rng.Intn(2)]))
		}
		text := sb.String()
		var want []string
		o := ParseOptions{Backend: b, Expectations: true}
		core.SkipRecordsOff = true
		werr := prog.ParseStreamWith("main", strings.NewReader(text), func(n *parse.Node) error { want = append(want, skipDump(t, n, nil)); return nil }, o)
		core.SkipRecordsOff = false
		for _, mk := range []func(io.Reader) io.Reader{iotest.OneByteReader, iotest.HalfReader, iotest.DataErrReader} {
			var got []string
			err := prog.ParseStreamWith("main", mk(strings.NewReader(text)), func(n *parse.Node) error { got = append(got, skipDump(t, n, nil)); return nil }, o)
			if (err == nil) != (werr == nil) || strings.Join(got, "|") != strings.Join(want, "|") {
				t.Fatalf("%s: text %q: got %v (%v), want %v (%v)", b, text, got, err, want, werr)
			}
		}
	}
}

// scanSteps returns the compute steps that the parse of in charges.
func scanSteps(t *testing.T, prog *Program, in string, b Backend) uint64 {
	t.Helper()
	var steps uint64
	o := ParseOptions{Backend: b, Limits: parse.Limits{Compute: 1 << 40}, Trace: func(e TraceEvent) {
		if e.Kind == TraceExit && e.Rule == "main" {
			steps = core.TraceParser(e).Control.Used
		}
	}}
	prog.ParseWith("main", in, o)
	return steps
}

// TestScanSitesChargeBounds checks the cost that a scan site adds to a parse. The charge of a parse
// is the same each time it is made, and a site adds to the charge of the walks without it at most two
// steps of each walk that reaches the hot span (the lookup and the slot, or the lookup and the
// record), which is
// the number of its steps divided by the span at most. Reuse only lowers the charge.
func TestScanSitesChargeBounds(t *testing.T) {
	if !core.BudgetControlsEnabled {
		t.Skip("budget controls are unavailable in the reference build")
	}
	for _, span := range []int{64, 1} {
		func() {
			defer func(n int) { core.ScanHotSpan = n }(core.ScanHotSpan)
			core.ScanHotSpan = span
			rng := rand.New(rand.NewSource(137))
			for _, g := range scanSiteGrammars {
				prog := mustCompile(t, g.Src)
				plain := compileWithoutScanSites(t, g.Src)
				for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
					for _, in := range scanSiteInputs(g.Alphabet, rng) {
						got, again, base := scanSteps(t, prog, in, b), scanSteps(t, prog, in, b), scanSteps(t, plain, in, b)
						if got == 0 || got != again {
							t.Fatalf("%s, %s, span %d: input %q charged %d and %d steps", g.Name, b, span, in, got, again)
						}
						if bound := base + 2*(base/uint64(span)) + 2; got > bound {
							t.Fatalf("%s, %s, span %d: input %q charged %d steps, %d without scan sites (bound %d)", g.Name, b, span, in, got, base, bound)
						}
					}
				}
			}
		}()
	}
}

// TestScanSitesCompileToInstructions checks that the bytecode of the scan sites holds the
// instructions that serve them, is accepted by the module validator and is rejected when it names a scan site
// that the module cannot hold.
func TestScanSitesCompileToInstructions(t *testing.T) {
	prog := mustCompile(t, `rule main = (w / r "?" / [ab ])* $$
rule w = (r "!" / r ".") #atomic
rule r = [ab]*`)
	m := prog.Module()
	var scan, iter int
	for _, in := range m.Code {
		switch {
		case in.Op == bytecode.OpScanSite:
			scan++
		case in.Op == bytecode.OpIter && in.B&4 != 0:
			iter++
		}
	}
	if scan == 0 || iter == 0 {
		t.Fatalf("%d SCANSITE and %d scan ITER instructions, want both", scan, iter)
	}
	if err := compile.ValidateModule(m); err != nil {
		t.Fatalf("the module of scan sites is rejected: %v", err)
	}
	for ip, in := range m.Code {
		if in.Op != bytecode.OpScanSite && !(in.Op == bytecode.OpIter && in.B&4 != 0) {
			continue
		}
		bad := *m
		bad.Code = append([]bytecode.Instr(nil), m.Code...)
		if in.Op == bytecode.OpScanSite {
			bad.Code[ip].C = 0
		} else {
			bad.Code[ip].B = 4 | 2
		}
		if err := compile.ValidateModule(&bad); err == nil {
			t.Errorf("instruction %d (%v) with an invalid scan site is accepted", ip, bad.Code[ip].Op)
		}
	}
}

// TestScanSitesWithBoundsBeyondInstructions checks the repetitions whose minimum does not fit an
// instruction operand: they are walked without a scan site and give what the program without
// scan sites gives.
func TestScanSitesWithBoundsBeyondInstructions(t *testing.T) {
	for _, src := range []string{
		`rule main = (w / r "?" / [ab ])* $$
rule w = (r "!" / r ".") #atomic
rule r = [ab] #repeat(5000000000,)`,
		`rule main = (r "!" / r "?" / [ab ])* $$
rule r = [ab] #repeat(5000000000,)`,
	} {
		prog := mustCompile(t, src)
		plain := compileWithoutScanSites(t, src)
		for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
			for _, in := range []string{"", "ab!", strings.Repeat("a", 200) + "!", strings.Repeat("ab", 90) + "?"} {
				o := ParseOptions{Backend: b, Expectations: true}
				n, err := prog.ParseWith("main", in, o)
				wn, werr := plain.ParseWith("main", in, o)
				if got, want := skipDump(t, n, err), skipDump(t, wn, werr); got != want {
					t.Fatalf("%s: input %q\n got  %s\n want %s", b, in, got, want)
				}
			}
		}
	}
}

// compileWithoutScanSites compiles src with the marking of scan sites off (see compileWithout).
func compileWithoutScanSites(t *testing.T, src string) *Program {
	t.Helper()
	return compileWithout(t, src, &compile.NoScanSites)
}

// compileWithout compiles src with the marking switched off by the flag. A parse under a limit builds
// the controlled graph of the closure backend, and recognition builds the recognizer, when they first
// run, and both read the flag then. The program parses once with each so that they are built before
// the flag is cleared again; without that the program a test takes for plain marks its sites on the
// closure backend.
func compileWithout(t *testing.T, src string, flag *bool) *Program {
	t.Helper()
	*flag = true
	defer func() { *flag = false }()
	prog := mustCompile(t, src)
	rec, err := prog.Recognizer()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []*Program{prog, rec} {
		for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
			p.ParseWith("main", "", ParseOptions{Backend: b, Limits: parse.Limits{Compute: 1 << 40}})
		}
	}
	return prog
}

// scanSpanProbe restarts the walk of ws at the start of each run of spaces in five alternatives.
const scanSpanProbe = `rule main = (ws "a" / ws "b" / ws "c" / ws "d" / ws "e")* $$
rule ws = [ ]*`

// TestScanSitesHitAtTheirOwnHotSpan checks that walks longer than the hot span of scan sites and
// shorter than that of run sites are answered from records on every backend: the hot span of scan
// sites is its own (core.ScanHotSpan), not that of run sites.
func TestScanSitesHitAtTheirOwnHotSpan(t *testing.T) {
	if core.ScanHotSpan >= core.RunHotSpan {
		t.Fatalf("the hot span of scan sites (%d) is not below that of run sites (%d)", core.ScanHotSpan, core.RunHotSpan)
	}
	prog := mustCompile(t, scanSpanProbe)
	rec, err := prog.Recognizer()
	if err != nil {
		t.Fatal(err)
	}
	// Parsing walks the scan sites that build values and recognition those that do not.
	for name, p := range map[string]*Program{"parse": prog, "recognize": rec} {
		for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
			for _, n := range []int{core.ScanHotSpan + 4, 40, core.RunHotSpan - 1} {
				in := strings.Repeat(" ", n) + "e"
				before := core.ScanHits.Load()
				if _, err := p.ParseWith("main", in, ParseOptions{Backend: b}); err != nil {
					t.Fatalf("%s, %s: run of %d: %v", name, b, n, err)
				}
				if core.ScanHits.Load() == before {
					t.Errorf("%s, %s: a run of %d spaces was not answered from a record", name, b, n)
				}
			}
		}
	}
}

// TestScanSitesInConcurrentParses runs parses that reach records on one program from many goroutines
// (it is meaningful under -race): the counters that tests read are atomic and parses share no
// other state.
func TestScanSitesInConcurrentParses(t *testing.T) {
	prog := mustCompile(t, scanSpanProbe)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
					in := strings.Repeat(" ", 20+(g+i)%50) + "e"
					if _, err := prog.ParseWith("main", in, ParseOptions{Backend: b, Limits: parse.Limits{Compute: 1 << 40}}); err != nil {
						t.Errorf("%s: %v", b, err)
						return
					}
				}
			}
		}(g)
	}
	wg.Wait()
}

// TestScanSitesChargeIsPinned pins the steps that parses of scanSpanProbe charge with and without
// scan sites, on each backend, for a run of spaces as long as the hot span and one four characters
// longer. The charge of a hit walk is one step for the lookup, one more for the slot of a record it
// finds, one for a record it stores, and none for the characters the record skips (see "Scan Sites"
// in spec/runtime/parsing.md); each of the units, and the characters that are charged, change these
// numbers (the mutations that drop the slot or the store change them).
func TestScanSitesChargeIsPinned(t *testing.T) {
	if !core.BudgetControlsEnabled {
		t.Skip("budget controls are unavailable in the reference build")
	}
	defer func(n int) { core.ScanHotSpan = n }(core.ScanHotSpan)
	core.ScanHotSpan = 16
	prog, plain := mustCompile(t, scanSpanProbe), compileWithoutScanSites(t, scanSpanProbe)
	type steps struct{ scan, plain uint64 }
	want := map[Backend]map[int]steps{
		Closure:           {16: {374, 432}, 20: {430, 520}},
		Bytecode:          {16: {347, 373}, 20: {391, 441}},
		BytecodeIterative: {16: {327, 353}, 20: {371, 421}},
	}
	for b, byLen := range want {
		for n, w := range byLen {
			in := strings.Repeat(" ", n) + "e"
			if got := (steps{scanSteps(t, prog, in, b), scanSteps(t, plain, in, b)}); got != w {
				t.Errorf("%s, run of %d: charged %d steps with scan sites and %d without, want %d and %d", b, n, got.scan, got.plain, w.scan, w.plain)
			}
		}
	}
}

// scanExamined returns the rule, the position and the examined range (the high-water mark) at each
// exit of a rule in the parse of in, and its result.
func scanExamined(t *testing.T, prog *Program, in string, o ParseOptions) (out []string, tree string) {
	t.Helper()
	o.Trace = func(e TraceEvent) {
		if e.Kind == TraceExit {
			out = append(out, fmt.Sprintf("%s@%d hw=%d", e.Rule, e.Pos, core.TraceParser(e).Hw))
		}
	}
	n, err := prog.ParseWith("main", in, o)
	return out, skipDump(t, n, err)
}

// scanLookahead examines further than its scan sites do before they are walked, so a walk that
// does not hand the examined range back to its caller shortens the range the rules report. The first
// alternative of r reads past the end of the walk that follows the second, and the lookahead of main
// does so before the walks of ws.
const scanLookahead = `rule main = &(ws "z") (ws "a" / ws "b" / ws "c" / ws "d" / ws "z")* $$
rule ws = [ ]*`

// scanInvalidBytes walks over invalid UTF-8, whose bytes are examined with the three that follow
// them, so that the walk examines past the character that ends it.
const scanInvalidBytes = `rule main = (r "?" / r "." / r ":" / r ";" / r "!")* $$
rule r = [^!]*`

const scanEarlierExamined = `rule main = (r "!" / r "?" / r "." / r ":" / r ";")* $$
rule r = ("xxxxxxxxxxxxxxxxxxxxyzzzzzB" / "x") [x]*`

// TestScanSitesKeepTheExaminedRange checks that the range of input that rules report to have
// examined (the high-water mark that the memo and the incremental parse rely on) is the same with
// and without scan sites, for walks that look up a record, find one and store one.
func TestScanSitesKeepTheExaminedRange(t *testing.T) {
	defer func(n int) { core.ScanHotSpan = n }(core.ScanHotSpan)
	rng := rand.New(rand.NewSource(139))
	grammars := append([]struct{ Name, Src, Alphabet string }{
		{"a lookahead that examines further", scanLookahead, " abdz"},
		{"a rule that examined further before the walk", scanEarlierExamined, "xyz!?.:;"},
		{"invalid bytes examined with the bytes that follow", scanInvalidBytes, "\xffz!?é"},
	}, scanSiteGrammars...)
	for _, span := range []int{16, 2} {
		core.ScanHotSpan = span
		for _, g := range grammars {
			prog, plain := mustCompile(t, g.Src), compileWithoutScanSites(t, g.Src)
			inputs := append([]string{strings.Repeat(" ", 30) + "z", strings.Repeat(" ", 20) + "z ", strings.Repeat("x", 20) + "yzzzzzA!", strings.Repeat("x", 20) + "yzzzzzA?", strings.Repeat("\xff", 20) + "!zzz", strings.Repeat("\xff", 20) + "!z", strings.Repeat("é", 20) + "!zzz", strings.Repeat("é", 20) + "?"}, scanSiteInputs(g.Alphabet, rng)...)
			for _, unit := range []parse.PositionUnit{parse.CodePoints, parse.Bytes} {
				for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
					o := ParseOptions{Backend: b, Unit: unit}
					for _, in := range inputs {
						got, gotTree := scanExamined(t, prog, in, o)
						want, wantTree := scanExamined(t, plain, in, o)
						if !slices.Equal(got, want) || gotTree != wantTree {
							t.Fatalf("%s, %s, %v, span %d: input %q\n got  %v %s\n want %v %s", g.Name, b, unit, span, in, got, gotTree, want, wantTree)
						}
					}
				}
			}
		}
	}
}

// scanSharedTable is a grammar whose walks of ws are, in order: two in lookaheads, which turn the site
// hot there; three outside, which turn it hot there and store a record at the third; and a lookahead
// walk and a walk outside that follow.
const scanSharedTable = `rule main = (&(ws "x") ws "a" / &(ws "y") ws "b" / ws "c" / ws "d" / ws "f" / &(ws "e") ws "e")* $$
rule ws = [ ]*`

// scanSharedTableFromLookahead walks ws three times in lookaheads, the third of which stores a record,
// and then four times outside, the third and fourth of which find it.
const scanSharedTableFromLookahead = `rule main = (&(ws "p") ws "a" / &(ws "q") ws "b" / &(ws "r") ws "c" / ws "d" / ws "f" / ws "g" / ws "e")* $$
rule ws = [ ]*`

// TestScanSitesShareRecordsWithLookaheads checks that, in recognition, a walk in a lookahead is
// answered from the record of a walk outside it: a site has one table for both, as their records
// are the same (see scanSlots in internal/engine/core/scan_sites.go). A parse that builds values
// walks lookaheads with a site of its own (the one without values), so they share nothing there;
// the results are the same either way.
func TestScanSitesShareRecordsWithLookaheads(t *testing.T) {
	for _, src := range []string{scanSharedTable, scanSharedTableFromLookahead} {
		prog, plain := mustCompile(t, src), compileWithoutScanSites(t, src)
		rec, err := prog.Recognizer()
		if err != nil {
			t.Fatal(err)
		}
		in := strings.Repeat(" ", 30) + "e"
		for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
			n, err := prog.ParseWith("main", in, ParseOptions{Backend: b})
			wn, werr := plain.ParseWith("main", in, ParseOptions{Backend: b})
			if skipDump(t, n, err) != skipDump(t, wn, werr) {
				t.Errorf("%s: the results differ", b)
			}
			before := core.ScanHits.Load()
			if _, err := rec.ParseWith("main", in, ParseOptions{Backend: b}); err != nil {
				t.Fatal(err)
			}
			if hits := core.ScanHits.Load() - before; hits != 2 {
				t.Errorf("%s: %d walks were answered from a record, want 2", b, hits)
			}
		}
	}
}

// documentSteps returns the steps that the first parse of a Document of text and the parse after
// replacing text[a:b] with ins charge.
func documentSteps(t *testing.T, prog *Program, b Backend, text string, a, e int, ins string) (first, edit uint64) {
	t.Helper()
	var steps uint64
	o := ParseOptions{Backend: b, Limits: parse.Limits{Compute: 1 << 40}, Trace: func(ev TraceEvent) {
		if ev.Kind == TraceExit && ev.Rule == "main" {
			steps = core.TraceParser(ev).Control.Used
		}
	}}
	d, err := prog.NewDocumentWith("main", text, o)
	if err != nil {
		t.Fatal(err)
	}
	d.Parse()
	first = steps
	if err := d.Edit(a, e, ins); err != nil {
		t.Fatal(err)
	}
	d.Parse()
	return first, steps
}

// TestScanSitesResumeInDocuments checks that a Document resumes the repetition of a scan site that is
// also a resumable run, as it does one that is not a scan site: the parse after an edit charges the
// steps it charges without scan sites. The first edit is at the
// end of a long run, where the whole run before the edit is resumed (and the parse charges less than
// the first), and the second inside the first sixteen characters, where the walk that follows the
// resumed part goes on past the hot span.
func TestScanSitesResumeInDocuments(t *testing.T) {
	if !core.BudgetControlsEnabled {
		t.Skip("budget controls are unavailable in the reference build")
	}
	const src = `rule main = (ws "a" / ws "b" / ws "c" / ws "d" / ws "e")* $$
rule ws = [ ]*`
	prog, plain := mustCompile(t, src), compileWithoutScanSites(t, src)
	cases := []struct {
		name          string
		text          string
		a, e          int
		ins           string
		lessThanFirst bool
	}{
		{"an edit at the end of a long run", strings.Repeat(" ", 3000) + "a", 3000, 3001, "b", true},
		{"an edit inside the hot span", strings.Repeat(" ", 40) + "e", 3, 3, "  ", false},
	}
	// Edits at every position of a run of 40 blanks: some leave a resumed prefix of exactly the hot
	// span, after which the walk goes on.
	for a := 1; a < 36; a++ {
		cases = append(cases, struct {
			name          string
			text          string
			a, e          int
			ins           string
			lessThanFirst bool
		}{fmt.Sprintf("an edit at %d of a run of 40", a), strings.Repeat(" ", 40) + "e", a, a, " ", false})
	}
	for _, c := range cases {
		for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
			first, edit := documentSteps(t, prog, b, c.text, c.a, c.e, c.ins)
			_, want := documentSteps(t, plain, b, c.text, c.a, c.e, c.ins)
			if edit != want || (c.lessThanFirst && edit >= first) {
				t.Errorf("%s, %s: the parse after the edit charged %d steps, %d without scan sites, and the first parse %d", c.name, b, edit, want, first)
			}
		}
	}
}
