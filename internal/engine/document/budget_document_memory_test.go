//go:build !zp_budget_controls_reference

package document_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	. "zeroparse.dev/zp/internal/engine"
	"zeroparse.dev/zp/internal/limits"
)

const documentMemoryGrammar = `type Pair = { K string; V string }
rule main = items:item* $$
rule item = k:key "=" v:val ";" => Pair{K: text($k) + "!", V: text($v)}
rule key = [a-z]+ #atomic
rule val = [0-9]+ #atomic / key`

func documentMemoryInput(n int) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "k%c=%d;", 'a'+rune(i%26), i)
	}
	return b.String()
}

// minDocumentMemory returns the smallest Memory under which parse succeeds, by bisection.
func minDocumentMemory(t *testing.T, parse func(mem uint64) error) uint64 {
	t.Helper()
	if err := parse(limits.Unlimited - 1); err != nil {
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

func memoryLimitError(err error) (*limits.LimitError, bool) {
	var le *limits.LimitError
	return le, errors.As(err, &le) && le.Kind == limits.LimitMemory
}

func TestDocumentMemoryLimit(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, documentMemoryGrammar)
	small := documentMemoryInput(20)
	for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
		t.Run(b.String(), func(t *testing.T) {
			// A first parse: the final count succeeds, one less fails and reports the count.
			parseFresh := func(mem uint64) error {
				d, err := prog.NewDocumentWith("main", small, ParseOptions{Backend: b})
				if err != nil {
					t.Fatal(err)
				}
				_, err = d.ParseWith(ParseOptions{Backend: b, Limits: limits.Limits{Memory: mem}})
				return err
			}
			m := minDocumentMemory(t, parseFresh)
			if err := parseFresh(m); err != nil {
				t.Fatalf("limit %d: %v", m, err)
			}
			if le, ok := memoryLimitError(parseFresh(m - 1)); !ok || le.Used != m || le.Limit != m-1 {
				t.Fatalf("limit %d: %+v, want used %d", m-1, le, m)
			}

			// The retained bytes count: a limit that admits them but not the parse fails.
			d, err := prog.NewDocumentWith("main", small, ParseOptions{Backend: b})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := d.Parse(); err != nil {
				t.Fatal(err)
			}
			r := d.RetainedBytes()
			if r == 0 {
				t.Fatal("a parsed Document retains no bytes")
			}
			if le, ok := memoryLimitError(func() error {
				_, err := d.ParseWith(ParseOptions{Backend: b, Limits: limits.Limits{Memory: r}})
				return err
			}()); !ok || le.Used <= r {
				t.Fatalf("limit %d, the retained bytes: %+v", r, le)
			}

			// A Document whose caches alone pass the limit drops them and parses from scratch.
			large := documentMemoryInput(2000)
			d, err = prog.NewDocumentWith("main", large, ParseOptions{Backend: b})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := d.Parse(); err != nil {
				t.Fatal(err)
			}
			if err := d.Edit(0, len(large), small); err != nil {
				t.Fatal(err)
			}
			r = d.RetainedBytes()
			if r <= m {
				t.Fatalf("retained %d bytes, want more than %d", r, m)
			}
			n, err := d.ParseWith(ParseOptions{Backend: b, Limits: limits.Limits{Memory: m}})
			if err != nil {
				t.Fatalf("parse after dropping the history: %v", err)
			}
			want, err := prog.ParseWith("main", small, ParseOptions{Backend: b})
			if err != nil {
				t.Fatal(err)
			}
			if n.String() != want.String() {
				t.Fatalf("tree %s, want %s", n, want)
			}
			if after := d.RetainedBytes(); after >= r {
				t.Fatalf("retained %d bytes after the parse, %d before: the history was not dropped", after, r)
			}
		})
	}
}
