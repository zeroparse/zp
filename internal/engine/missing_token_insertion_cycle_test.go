package engine

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/engine/state"
)

// insertionCycle is left-recursive only through an insertion: without insertion, a calls itself
// only after "(" consumed input.
const insertionCycle = `rule main = a $$
rule a = "(" #missing a "x" / "y"`

// TestMissingTokensInsertionOnlyLeftRecursion checks that a rule left-recursive only through an
// insertion is grown when insertion is enabled, and only then: parses without insertion run the
// program of the grammar's own flags, and each insertion of "(" before a seed grows it by one "x".
func TestMissingTokensInsertionOnlyLeftRecursion(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, insertionCycle)
	if prog.InsertionFlags == nil || prog.ByName["a"].Leader {
		t.Fatalf("flags: insertion %v, leader without insertion %v", prog.InsertionFlags, prog.ByName["a"].Leader)
	}
	for input, ok := range map[string]bool{"y": true, "(yx": true, "((yxx": true, "yx": false, "": false} {
		n, err := prog.ParseWith("main", input, ParseOptions{})
		if ok != (err == nil && n != nil) {
			t.Fatalf("without insertion, %q: %v %v", input, n, err)
		}
	}
	if prog.ins != nil {
		t.Fatal("a parse without insertion built the insertion program")
	}
	for _, controlled := range []bool{false, true} {
		if controlled && !state.BudgetControlsEnabled {
			continue
		}
		for _, recognize := range []bool{false, true} {
			o := ParseOptions{Expectations: true, MissingTokens: 1000, Recognize: recognize}
			if controlled {
				o.Context = context.Background()
				o.Limits.Compute = 1 << 24
			}
			if n, err := prog.ParseWith("main", "y", o); err != nil || !recognize && len(missingNodes(t, n)) != 0 {
				t.Fatalf("controlled %v, recognize %v: y: %v %v", controlled, recognize, n, err)
			}
			for xs := 1; xs <= 4; xs++ {
				input := "y" + strings.Repeat("x", xs)
				// The insertions are all at the start, where their diagnostics are one syntax error.
				n, _ := requireInsertions(t, prog, input, o, 1)
				if !recognize && len(missingNodes(t, n)) != xs {
					t.Fatalf("controlled %v: %q: missing nodes %d", controlled, input, len(missingNodes(t, n)))
				}
			}
		}
	}
	if prog.ins == nil || !prog.ins.ByName["a"].Leader || prog.ins.ByName["a"].Memo {
		t.Fatal("insertion parses did not run the program with the insertion flags")
	}
}

// TestMissingTokensInsertionOnlyLeftRecursionLimits checks that the limit of missing tokens only
// caps an insertion-only left recursion: every parse with a positive limit either gives the result
// of an unbounded one or fails with the limit error, since abandoned attempts, the growth rounds'
// included, count toward the limit.
func TestMissingTokensInsertionOnlyLeftRecursionLimits(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, insertionCycle)
	render := func(n *core.Node, err error) string {
		data, jerr := json.Marshal(n)
		if jerr != nil {
			t.Fatal(jerr)
		}
		var recovered core.SyntaxErrors
		if errors.As(err, &recovered) {
			err = nil
		}
		return string(data) + "|" + strings.Repeat("e", len(recovered)) + "|" + func() string {
			if err != nil {
				return err.Error()
			}
			return ""
		}()
	}
	for _, input := range []string{"y", "yx", "yxx", "yxxx", "(yx", "x", ""} {
		want := render(prog.ParseWith("main", input, ParseOptions{Expectations: true, MissingTokens: 1000}))
		capped := false
		for limit := 1; limit <= 50; limit++ {
			n, err := prog.ParseWith("main", input, ParseOptions{Expectations: true, MissingTokens: limit})
			var exceeded *core.MissingTokenLimitError
			if errors.As(err, &exceeded) {
				if capped {
					t.Fatalf("%q: limit %d exceeded after a smaller limit sufficed", input, limit)
				}
				continue
			}
			capped = true
			if got := render(n, err); got != want {
				t.Fatalf("%q at limit %d: %s, want %s", input, limit, got, want)
			}
		}
		if !capped {
			t.Fatalf("%q: every limit up to 50 was exceeded", input)
		}
	}
}

// TestMissingTokensInsertionOnlyLeftRecursionRejected checks that the backends without
// missing-token insertion keep rejecting a grammar whose left recursion exists only through an
// insertion.
func TestMissingTokensInsertionOnlyLeftRecursionRejected(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, insertionCycle)
	for _, b := range []Backend{Bytecode, BytecodeIterative} {
		if _, err := prog.ParseWith("main", "y", ParseOptions{Backend: b}); err == nil || !strings.Contains(err.Error(), "missing") {
			t.Fatalf("backend %v: %v", b, err)
		}
	}
	if err := prog.ParseStream("main", strings.NewReader("y"), func(*core.Node) error { return nil }); err == nil {
		t.Fatal("stream parsing accepted the grammar")
	}
	if _, err := prog.NewDocument("main", "y"); err == nil {
		t.Fatal("documents accepted the grammar")
	}
	if _, err := prog.MarshalBinary("main"); err == nil {
		t.Fatal("saved the grammar as a compiled grammar")
	}
}
