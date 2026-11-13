package gogen_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gentest"
)

// memoryLimitGrammar builds nodes, child and field lists, memo entries, typed values and strings
// concatenated by actions.
const memoryLimitGrammar = `type Pair = { K string; V string }
rule main = items:item* $$
rule item = k:key "=" v:val ";" => Pair{K: text($k) + "!", V: text($v)}
rule key = [a-z]+ #atomic
rule val = [0-9]+ #atomic / key`

// The memory limit of a generated parser holds at the final count and fails one byte below it, for
// every entry point; the count does not depend on what a pooled parser parsed before.
func TestGeneratedMemoryLimit(t *testing.T) {
	t.Parallel()
	gentest.NeedsBudgetControls(t)
	if testing.Short() {
		t.Skip("builds generated parsers")
	}
	g, err := parseTest(memoryLimitGrammar)
	if err != nil {
		t.Fatal(err)
	}
	typed, err := gentest.Generate(t, g, gen.GenOptions{Package: "mem", Module: "mem", Start: "main", Runtime: gen.RuntimeImport, Types: true, Document: true, LimitControls: true})
	if err != nil {
		t.Fatal(err)
	}
	gs, err := parseTest(gentest.Records)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := gentest.Generate(t, gs, gen.GenOptions{Package: "memstream", Module: "mem/memstream", Start: "main", Runtime: gen.RuntimeImport, Stream: true, Types: true, LimitControls: true})
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"go.mod":                      []byte(gentest.GoMod("mem")),
		"parser.go":                   gentest.MergeGenerated(typed),
		"mem_test.go":                 []byte(memoryLimitConsumer),
		"memstream/parser.go":         gentest.MergeGenerated(stream),
		"memstream/memstream_test.go": []byte(memoryLimitStreamConsumer),
	}
	// Streams whose elements carry concatenated strings in struct values, and that recover.
	for _, c := range []struct{ pkg, grammar, consumer string }{
		{"memconcat", gentest.RecordsConcat, memoryLimitConcatConsumer},
		{"memrecover", gentest.RecordsRecover, memoryLimitRecoverConsumer},
	} {
		g, err := parseTest(c.grammar)
		if err != nil {
			t.Fatal(err)
		}
		out, err := gentest.Generate(t, g, gen.GenOptions{Package: c.pkg, Module: "mem/" + c.pkg, Start: "main", Runtime: gen.RuntimeImport, Stream: true, Types: true, LimitControls: true})
		if err != nil {
			t.Fatal(err)
		}
		files[c.pkg+"/parser.go"] = gentest.MergeGenerated(out)
		files[c.pkg+"/"+c.pkg+"_test.go"] = []byte(c.consumer)
	}
	dir := t.TempDir()
	for _, sub := range []string{"memstream", "memconcat", "memrecover"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "test", "-count=1", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
}

const memoryLimitHelpers = `
// minMemory returns the smallest Memory under which parse succeeds, by bisection: the count of a
// parse without returns only grows, so a parse succeeds exactly when the limit is at least its
// final count.
func minMemory(t *testing.T, parse func(mem uint64) error) uint64 {
	t.Helper()
	if err := parse(Unlimited - 1); err != nil {
		t.Fatalf("parse with the largest finite limit: %v", err)
	}
	lo, hi := uint64(0), uint64(1<<40)
	for hi-lo > 1 {
		mid := lo + (hi-lo)/2
		if parse(mid) == nil {
			hi = mid
		} else {
			lo = mid
		}
	}
	return hi
}

// checkThreshold checks both directions of the limit at the final count m: m succeeds, and m-1
// fails with a LimitError that reports the count.
func checkThreshold(t *testing.T, name string, m uint64, parse func(mem uint64) error) {
	t.Helper()
	if err := parse(m); err != nil {
		t.Fatalf("%s: limit %d (the final count): %v", name, m, err)
	}
	err := parse(m - 1)
	var le *LimitError
	if !errors.As(err, &le) || le.Kind != LimitMemory || le.Limit != m-1 || le.Used != m || le.Requested != 0 || !errors.Is(err, ErrLimit) {
		t.Fatalf("%s: limit %d: error %v (%+v), want LimitMemory used %d", name, m-1, err, le, m)
	}
	if want := fmt.Sprintf("parse memory limit exceeded: limit %d, used %d", m-1, m); err.Error() != want {
		t.Fatalf("%s: message %q, want %q", name, err.Error(), want)
	}
}

func memory(mem uint64, opts ...ParseOption) []ParseOption {
	return append(opts, WithLimits(Limits{Compute: Unlimited, Memory: mem}))
}
`

const memoryLimitConsumer = `package mem

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func input(n int) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "k%c=%d;", 'a'+rune(i%26), i)
	}
	return b.String()
}
` + memoryLimitHelpers + `
func TestMemoryLimitThreshold(t *testing.T) {
	text := input(300)
	for _, unit := range []PositionUnit{CodePoints, Bytes} {
		entries := map[string]func(mem uint64) error{
			"Parse": func(mem uint64) error { _, err := Parse(text, memory(mem, WithUnit(unit))...); return err },
			"ParseValue": func(mem uint64) error { _, err := ParseValue(text, memory(mem, WithUnit(unit))...); return err },
			"Document.Parse": func(mem uint64) error {
				d, err := NewDocument(text, WithUnit(unit))
				if err != nil {
					t.Fatal(err)
				}
				_, err = d.Parse(memory(mem)...)
				return err
			},
			"Document.ParseValue": func(mem uint64) error {
				d, err := NewDocument(text, WithUnit(unit))
				if err != nil {
					t.Fatal(err)
				}
				_, err = d.ParseValue(memory(mem)...)
				return err
			},
		}
		for name, parse := range entries {
			name = fmt.Sprintf("%s/%d", name, unit)
			m := minMemory(t, parse)
			if m < uint64(len(text)) {
				t.Fatalf("%s: final count %d is below the input length %d", name, m, len(text))
			}
			checkThreshold(t, name, m, parse)
			// A pooled parser that ran a larger parse counts the same.
			if _, err := Parse(input(3000), WithUnit(unit)); err != nil {
				t.Fatal(err)
			}
			if _, err := ParseValue(input(3000), WithUnit(unit)); err != nil {
				t.Fatal(err)
			}
			checkThreshold(t, name, m, parse)
		}
	}
}

// A finite Memory alone makes the parse controlled; zero and Unlimited leave it unlimited.
func TestMemoryLimitAlone(t *testing.T) {
	text := input(50)
	_, err := Parse(text, WithLimits(Limits{Memory: 1}))
	var le *LimitError
	if !errors.As(err, &le) || le.Kind != LimitMemory || le.Limit != 1 {
		t.Fatalf("Memory 1: %v", err)
	}
	for _, mem := range []uint64{0, Unlimited} {
		if _, err := Parse(text, WithLimits(Limits{Memory: mem})); err != nil {
			t.Fatalf("Memory %d: %v", mem, err)
		}
		if _, err := ParseValue(text, WithLimits(Limits{Memory: mem})); err != nil {
			t.Fatalf("Memory %d: %v", mem, err)
		}
	}
	// A later WithLimits keeps the Memory an earlier one set.
	_, err = Parse(text, WithLimits(Limits{Memory: 1}), WithLimits(Limits{Depth: 50}))
	if !errors.As(err, &le) || le.Kind != LimitMemory {
		t.Fatalf("merged limits: %v", err)
	}
}

// A Document keeps its memo for later parses under a limit its count fits.
func TestMemoryLimitDocumentReuse(t *testing.T) {
	d, err := NewDocument(input(300))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Parse(); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Parse(memory(Unlimited - 1)...); err != nil {
		t.Fatal(err)
	}
	if d.Stats().Reused == 0 {
		t.Fatal("a parse under a finite Memory did not reuse the retained memo")
	}
}
`

const memoryLimitStreamConsumer = `package memstream

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func records(n int) string {
	var b strings.Builder
	b.WriteString("#records\n")
	for i := range n {
		fmt.Fprintf(&b, "k%c=%d\n", 'a'+rune(i%26), i)
	}
	return b.String()
}
` + memoryLimitHelpers + `
// A stream returns what its commits let go of, so its count follows the window, not the length.
func TestMemoryLimitStream(t *testing.T) {
	for _, unit := range []PositionUnit{CodePoints, Bytes} {
		stream := func(text string) func(uint64) error {
			return func(mem uint64) error {
				return ParseStream(strings.NewReader(text), func(*Node) error { return nil }, memory(mem, WithUnit(unit))...)
			}
		}
		short, long := minMemory(t, stream(records(2000))), minMemory(t, stream(records(40000)))
		checkThreshold(t, "stream", long, stream(records(40000)))
		batch := minMemory(t, func(mem uint64) error { _, err := Parse(records(40000), memory(mem, WithUnit(unit))...); return err })
		if 5*long > 6*short || long*4 > batch {
			t.Fatalf("unit %d: stream of 40,000 records needs %d bytes, of 2,000 records %d, batch %d", unit, long, short, batch)
		}
	}
}

// A typed stream returns the typed values of the elements it delivered too.
func TestMemoryLimitStreamAST(t *testing.T) {
	for _, unit := range []PositionUnit{CodePoints, Bytes} {
		stream := func(text string) func(uint64) error {
			return func(mem uint64) error {
				return ParseStreamValue(strings.NewReader(text), func(StreamValue) error { return nil }, memory(mem, WithUnit(unit))...)
			}
		}
		short, long := minMemory(t, stream(records(2000))), minMemory(t, stream(records(40000)))
		checkThreshold(t, "typed stream", long, stream(records(40000)))
		batch := minMemory(t, func(mem uint64) error { _, err := ParseValue(records(40000), memory(mem, WithUnit(unit))...); return err })
		t.Logf("unit %d: typed stream of 2,000 records %d bytes, of 40,000 %d, batch %d", unit, short, long, batch)
		if 5*long > 6*short || long*4 > batch {
			t.Fatalf("unit %d: typed stream of 40,000 records needs %d bytes, of 2,000 records %d, batch %d", unit, long, short, batch)
		}
	}
}
`

const memoryLimitStreamLines = `
// streamRecords returns a stream of a header and n lines, the line of record i given by line.
func streamRecords(n int, line func(i int) string) string {
	var b strings.Builder
	b.WriteString("#records\n")
	for i := range n {
		b.WriteString(line(i))
	}
	return b.String()
}

func recordLine(i int) string { return fmt.Sprintf("k%c=%d\n", 'a'+rune(i%26), i) }

// streamMemory is minMemory for a stream whose result may report recovered errors: only a memory
// limit error counts as failure.
func streamMemory(t *testing.T, parse func(mem uint64) error) uint64 {
	t.Helper()
	isMem := func(err error) bool {
		var le *LimitError
		return errors.As(err, &le) && le.Kind == LimitMemory
	}
	if err := parse(Unlimited - 1); isMem(err) {
		t.Fatalf("parse with the largest finite limit: %v", err)
	}
	lo, hi := uint64(0), uint64(1<<40)
	for hi-lo > 1 {
		mid := lo + (hi-lo)/2
		if isMem(parse(mid)) {
			lo = mid
		} else {
			hi = mid
		}
	}
	return hi
}
`

const memoryLimitConcatConsumer = `package memconcat

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)
` + memoryLimitHelpers + memoryLimitStreamLines + `
// A stream returns the strings that actions concatenated for the elements it delivered, in Node
// and in struct-typed elements.
func TestMemoryLimitStreamConcat(t *testing.T) {
	for _, unit := range []PositionUnit{CodePoints, Bytes} {
		node := func(text string) func(uint64) error {
			return func(mem uint64) error {
				return ParseStream(strings.NewReader(text), func(*Node) error { return nil }, memory(mem, WithUnit(unit))...)
			}
		}
		typed := func(text string) func(uint64) error {
			return func(mem uint64) error {
				return ParseStreamValue(strings.NewReader(text), func(StreamValue) error { return nil }, memory(mem, WithUnit(unit))...)
			}
		}
		for name, stream := range map[string]func(string) func(uint64) error{"node": node, "typed": typed} {
			short, long := minMemory(t, stream(streamRecords(2000, recordLine))), minMemory(t, stream(streamRecords(40000, recordLine)))
			checkThreshold(t, name, long, stream(streamRecords(40000, recordLine)))
			t.Logf("%s unit %d: 2,000 records %d bytes, 40,000 %d", name, unit, short, long)
			if 5*long > 6*short {
				t.Fatalf("%s unit %d: stream of 40,000 records needs %d bytes, of 2,000 records %d", name, unit, long, short)
			}
		}
	}
}
`

const memoryLimitRecoverConsumer = `package memrecover

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)
` + memoryLimitHelpers + memoryLimitStreamLines + `
func malformed(i int) string {
	if i%2 == 0 {
		return "!!\n"
	}
	return recordLine(i)
}

// A diagnostic limit hands the recovered errors to its tail at each commit, so the count of the
// stream stays bounded, also when every line is an error; without one the errors are kept for the
// result and grow the count.
func TestMemoryLimitStreamRecover(t *testing.T) {
	blank := func(int) string { return "\n" }
	for _, unit := range []PositionUnit{CodePoints, Bytes} {
		stream := func(text string, opts ...ParseOption) func(uint64) error {
			return func(mem uint64) error {
				return ParseStream(strings.NewReader(text), func(*Node) error { return nil }, memory(mem, append(opts, WithUnit(unit))...)...)
			}
		}
		for _, c := range []struct {
			name  string
			line  func(int) string
			opts  []ParseOption
			grows bool
			scale int // of the shorter stream, so that one-byte lines also fill the read buffer
		}{
			{"recover", malformed, []ParseOption{WithDiagnosticLimit(8)}, false, 1},
			{"recover-newlines", blank, []ParseOption{WithDiagnosticLimit(8)}, false, 5},
			{"recover-kept", malformed, nil, true, 1},
		} {
			short := streamMemory(t, stream(streamRecords(2000*c.scale, c.line), c.opts...))
			long := streamMemory(t, stream(streamRecords(40000, c.line), c.opts...))
			t.Logf("%s unit %d: 2,000 lines %d bytes, 40,000 %d", c.name, unit, short, long)
			if c.grows != (long >= 5*short) || !c.grows && 5*long > 6*short {
				t.Fatalf("%s unit %d: stream of 40,000 lines needs %d bytes, of 2,000 lines %d (grows %v)", c.name, unit, long, short, c.grows)
			}
		}
	}
}
`
