//go:build !zp_budget_controls_reference

package engine

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/gentest"
	"zeroparse.dev/zp/internal/value"
	"zeroparse.dev/zp/runtime/parse"
)

// memoryGrammar builds nodes, child and field lists, memo entries and strings concatenated by
// actions.
const memoryGrammar = `type Pair = { K string; V string }
rule main = items:item* $$
rule item = k:key "=" v:val ";" => Pair{K: text($k) + "!", V: text($v)}
rule key = [a-z]+ #atomic
rule val = [0-9]+ #atomic / key`

func memoryInput(n int) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "k%c=%d;", 'a'+rune(i%26), i)
	}
	return b.String()
}

// minMemory returns the smallest Memory under which parse succeeds, by bisection: the count of a
// parse without returns only grows, so a parse succeeds exactly when the limit is at least its
// final count.
func minMemory(t *testing.T, parseFn func(mem uint64) error) uint64 {
	t.Helper()
	if err := parseFn(parse.Unlimited - 1); err != nil {
		t.Fatalf("parse with the largest finite limit: %v", err)
	}
	lo, hi := uint64(0), uint64(1<<40)
	for hi-lo > 1 {
		mid := lo + (hi-lo)/2
		if parseFn(mid) == nil {
			hi = mid
		} else {
			lo = mid
		}
	}
	return hi
}

// checkMemoryThreshold checks both directions of the limit at the final count m: m succeeds, and
// m-1 fails with a LimitError that reports the count.
func checkMemoryThreshold(t *testing.T, m uint64, parseFn func(mem uint64) error) {
	t.Helper()
	if err := parseFn(m); err != nil {
		t.Fatalf("limit %d (the final count): %v", m, err)
	}
	err := parseFn(m - 1)
	var le *parse.LimitError
	if !errors.As(err, &le) || le.Kind != parse.LimitMemory || le.Limit != m-1 || le.Used != m || le.Requested != 0 {
		t.Fatalf("limit %d: error %v (%+v), want LimitMemory used %d", m-1, err, le, m)
	}
	if !errors.Is(err, parse.ErrLimit) {
		t.Fatalf("limit %d: %v is not ErrLimit", m-1, err)
	}
}

func TestMemoryLimitThreshold(t *testing.T) {
	t.Parallel()
	input := memoryInput(300)
	for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
		for _, u := range []parse.PositionUnit{parse.CodePoints, parse.Bytes} {
			t.Run(fmt.Sprintf("%s/%s", b, u), func(t *testing.T) {
				t.Parallel()
				prog := mustCompile(t, memoryGrammar)
				parseFn := func(mem uint64) error {
					_, err := prog.ParseWith("main", input, ParseOptions{Backend: b, Unit: u, Limits: parse.Limits{Compute: parse.Unlimited, Memory: mem}})
					return err
				}
				m := minMemory(t, parseFn)
				// The decoded input and the nodes alone take more than the input's length.
				if m < uint64(len(input)) {
					t.Fatalf("final count %d is below the input length %d", m, len(input))
				}
				checkMemoryThreshold(t, m, parseFn)
				// A pooled parser that ran a larger parse, and a fresh program, count the same.
				if _, err := prog.ParseWith("main", memoryInput(3000), ParseOptions{Backend: b, Unit: u}); err != nil {
					t.Fatal(err)
				}
				checkMemoryThreshold(t, m, parseFn)
				fresh := mustCompile(t, memoryGrammar)
				if got := minMemory(t, func(mem uint64) error {
					_, err := fresh.ParseWith("main", input, ParseOptions{Backend: b, Unit: u, Limits: parse.Limits{Compute: parse.Unlimited, Memory: mem}})
					return err
				}); got != m {
					t.Fatalf("fresh program: final count %d, pooled %d", got, m)
				}
			})
		}
	}
}

// A finite Memory alone makes the parse controlled; Unlimited and zero leave it unlimited.
func TestMemoryLimitAlone(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, memoryGrammar)
	input := memoryInput(50)
	for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
		_, err := prog.ParseWith("main", input, ParseOptions{Backend: b, Limits: parse.Limits{Compute: parse.Unlimited, Memory: 1}})
		var le *parse.LimitError
		if !errors.As(err, &le) || le.Kind != parse.LimitMemory || le.Error() != fmt.Sprintf("parse memory limit exceeded: limit 1, used %d", le.Used) {
			t.Fatalf("%s: %v", b, err)
		}
		for _, mem := range []uint64{0, parse.Unlimited} {
			if _, err := prog.ParseWith("main", input, ParseOptions{Backend: b, Limits: parse.Limits{Compute: parse.Unlimited, Memory: mem}}); err != nil {
				t.Fatalf("%s: memory %d: %v", b, mem, err)
			}
		}
	}
}

// streamRecords returns a stream of the records grammars: a header and n lines, the line of
// record i given by line.
func streamRecords(n int, line func(i int) string) string {
	var b strings.Builder
	b.WriteString("#records\n")
	for i := range n {
		b.WriteString(line(i))
	}
	return b.String()
}

func recordLine(i int) string { return fmt.Sprintf("k%c=%d\n", 'a'+rune(i%26), i) }

// A stream returns what its commits let go of, so its count follows the window, not the length:
// the nodes, copied text and concatenated strings of the elements it delivered, and the recovered
// errors that a diagnostic limit hands to its tail. Without a diagnostic limit the recovered errors
// are kept for the result, so the count grows with them.
func TestMemoryLimitStream(t *testing.T) {
	t.Parallel()
	malformed := func(i int) string {
		if i%2 == 0 {
			return "!!\n"
		}
		return recordLine(i)
	}
	blank := func(int) string { return "\n" }
	cases := []struct {
		name, grammar string
		line          func(int) string
		diagLimit     int
		grows         bool // the count grows with the stream
		// scale multiplies the 2,000 lines of the shorter stream, so that a stream of one-byte
		// lines also fills the read buffer and reaches the window of the longer one.
		scale int
	}{
		{"plain", gentest.Records, recordLine, 0, false, 1},
		{"concat", gentest.RecordsConcat, recordLine, 0, false, 1},
		{"recover", gentest.RecordsRecover, malformed, 8, false, 1},
		{"recover-newlines", gentest.RecordsRecover, blank, 8, false, 5},
		{"recover-kept", gentest.RecordsRecover, malformed, 0, true, 1},
	}
	for _, c := range cases {
		prog := mustCompile(t, c.grammar)
		for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
			t.Run(c.name+"/"+b.String(), func(t *testing.T) {
				t.Parallel()
				opts := func(mem uint64) ParseOptions {
					return ParseOptions{Backend: b, Limits: parse.Limits{Compute: parse.Unlimited, Memory: mem},
						DiagnosticLimit: c.diagLimit, DiagnosticLimitSet: c.diagLimit > 0}
				}
				stream := func(text string) func(uint64) error {
					return func(mem uint64) error {
						return prog.ParseStreamWith("main", strings.NewReader(text), func(*value.Node) error { return nil }, opts(mem))
					}
				}
				short := streamMemory(t, stream(streamRecords(2000*c.scale, c.line)))
				long := streamMemory(t, stream(streamRecords(40000, c.line)))
				t.Logf("%d lines %d bytes, 40,000 lines %d", 2000*c.scale, short, long)
				if c.grows {
					if long < 5*short {
						t.Fatalf("stream of 40,000 lines needs %d bytes, of 2,000 lines %d; want the kept errors to grow the count", long, short)
					}
					return
				}
				if 5*long > 6*short { // within 1.2: the window, not the length
					t.Fatalf("stream of 40,000 lines needs %d bytes, of 2,000 lines %d", long, short)
				}
				if c.name == "plain" {
					batch := minMemory(t, func(mem uint64) error {
						_, err := prog.ParseWith("main", streamRecords(40000, c.line), opts(mem))
						return err
					})
					if long*4 > batch {
						t.Fatalf("stream of 40,000 records needs %d bytes, batch %d", long, batch)
					}
				}
			})
		}
	}
}

// streamMemory is minMemory for a stream whose result may report recovered errors: only a memory
// limit error counts as failure.
func streamMemory(t *testing.T, parseFn func(mem uint64) error) uint64 {
	t.Helper()
	isMem := func(err error) bool {
		var le *parse.LimitError
		return errors.As(err, &le) && le.Kind == parse.LimitMemory
	}
	if err := parseFn(parse.Unlimited - 1); isMem(err) {
		t.Fatalf("parse with the largest finite limit: %v", err)
	}
	lo, hi := uint64(0), uint64(1<<40)
	for hi-lo > 1 {
		mid := lo + (hi-lo)/2
		if isMem(parseFn(mid)) {
			lo = mid
		} else {
			hi = mid
		}
	}
	return hi
}
