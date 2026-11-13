package engine

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
)

var depthBackends = []Backend{Closure, Bytecode, BytecodeIterative}

// MaxDepth limits the nesting of recursive rules: the rules that call one another around a cycle.
// The same input meets the limit at the same nesting on every backend, and a rule that is not on a
// cycle never counts.
func TestMaxDepthCountsRecursion(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, `rule main = chain $$
rule chain = nest
rule nest = "(" nest ")" / leaf
rule leaf = digit "!"
rule digit = [0-9]`)
	// n parentheses make n+1 entries of nest, the only recursive rule.
	for n := 0; n < 6; n++ {
		in := strings.Repeat("(", n) + "1!" + strings.Repeat(")", n)
		for _, b := range depthBackends {
			for _, limit := range []int{1, 2, n, n + 1, n + 2} {
				if limit < 1 {
					continue
				}
				_, err := prog.ParseWith("main", in, ParseOptions{Backend: b, Limits: core.Limits{Depth: uint64(limit)}})
				if want := n+1 > limit; (err != nil) != want {
					t.Errorf("%v: %d nested, limit %d: error %v, want an error: %v", b, n, limit, err, want)
				}
				if err != nil && err.Error() != fmt.Sprintf("nesting too deep: more than %d nested recursive rule calls", limit) {
					t.Errorf("%v: %d nested, limit %d: error %v", b, n, limit, err)
				}
				var le *core.LimitError
				if err != nil && (!errors.As(err, &le) || *le != (core.LimitError{Kind: core.LimitDepth, Limit: uint64(limit), Used: uint64(limit), Requested: 1})) {
					t.Errorf("%v: %d nested, limit %d: error %#v, want a LimitDepth error", b, n, limit, err)
				}
			}
		}
	}
}

// A choice skips an alternative whose first character does not match only when running it would
// not meet the depth limit first: an alternative that enters a recursive rule before its first
// character meets the limit even on input it would not match.
func TestMaxDepthBeforeSkippedAlternative(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, `rule main = item $$
rule item = nest / "x"
rule nest = "(" item ")"`)
	// n parentheses make 2n+1 nested recursive calls (item and nest), and the innermost item tries
	// nest once more before "x".
	for n := 0; n < 4; n++ {
		in := strings.Repeat("(", n) + "x" + strings.Repeat(")", n)
		for _, b := range depthBackends {
			for limit := 1; limit <= 2*n+3; limit++ {
				_, err := prog.ParseWith("main", in, ParseOptions{Backend: b, Limits: core.Limits{Depth: uint64(limit)}})
				if want := 2*n+2 > limit; (err != nil) != want {
					t.Errorf("%v: %d nested, limit %d: error %v, want an error: %v", b, n, limit, err, want)
				}
				var le *core.LimitError
				if err != nil && (!errors.As(err, &le) || le.Kind != core.LimitDepth) {
					t.Errorf("%v: %d nested, limit %d: error %#v, want a LimitDepth error", b, n, limit, err)
				}
			}
		}
	}
}

// A grammar without recursion never meets a limit of one, however long the input.
func TestMaxDepthIgnoresRulesOutsideCycles(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, `rule main = a* $$
rule a = b "," c ";"
rule b = d
rule c = d
rule d = [0-9]+`)
	in := strings.Repeat("12,34;", 500)
	for _, b := range depthBackends {
		if _, err := prog.ParseWith("main", in, ParseOptions{Backend: b, Limits: core.Limits{Depth: 1}}); err != nil {
			t.Errorf("%v: %v", b, err)
		}
	}
}

// The iterative VM runs input far deeper than a recursive backend could on the stack, and the
// recursive backends stop at their limit with an error, not a crash.
func TestMaxDepthDeepInput(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, `rule main = nest $$
rule nest = "(" nest ")" / leaf
rule leaf = "x"`)
	const n = 3 * DefaultMaxDepth
	in := strings.Repeat("(", n) + "x" + strings.Repeat(")", n)
	for _, b := range []Backend{Closure, Bytecode} {
		if _, err := prog.ParseWith("main", in, ParseOptions{Backend: b}); err == nil || !strings.Contains(err.Error(), "nesting too deep") {
			t.Errorf("%v: got %v, want a nesting error", b, err)
		}
	}
	if _, err := prog.ParseWith("main", in, ParseOptions{Backend: BytecodeIterative}); err != nil {
		t.Errorf("iterative: %v", err)
	}
}

// An unlimited depth is accepted by the iterative VM only, whose call stack is on the heap; the
// backends that nest rule calls on the goroutine stack reject it before parsing, in every entry
// point, and a Document rejects it at each Parse.
func TestUnlimitedDepth(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, `rule main = nest $$
rule nest = "(" nest ")" / leaf
rule leaf = "x"`)
	const n = 2 * DefaultMaxDepthIterative / 1000
	in := strings.Repeat("(", n) + "x" + strings.Repeat(")", n)
	unlimited := core.Limits{Depth: core.Unlimited}
	if _, err := prog.ParseWith("main", in, ParseOptions{Backend: BytecodeIterative, Limits: unlimited}); err != nil {
		t.Errorf("iterative: %v", err)
	}
	if _, err := prog.ParseWith("main", in, ParseOptions{Backend: BytecodeIterative, Limits: core.Limits{Depth: n}}); err == nil {
		t.Error("iterative: a finite depth below the nesting accepted")
	}
	for _, b := range []Backend{Default, Closure, Bytecode} {
		o := ParseOptions{Backend: b, Limits: unlimited}
		check := func(what string, err error) {
			t.Helper()
			if err == nil || !strings.Contains(err.Error(), "unlimited depth needs the bytecode-iterative backend") {
				t.Errorf("%v %s: error %v, want the option error", b, what, err)
			}
		}
		_, err := prog.ParseWith("main", "x", o)
		check("ParseWith", err)
		_, err = prog.ParsePrefixWith("main", "x", o)
		check("ParsePrefixWith", err)
		_, err = prog.NewDocumentWith("main", "x", o)
		check("NewDocumentWith", err)
		d, err := prog.NewDocumentWith("main", "x", ParseOptions{Backend: b})
		if err != nil {
			t.Fatal(err)
		}
		_, err = d.ParseWith(o)
		check("Document.ParseWith", err)
	}
}

// The #stream repetition of the start rule streams whether the start rule is recursive or not, and
// that of a rule that is not the start rule does not.
func TestStreamStartRuleNeedsNoCount(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, src string
	}{
		{"plain start rule", `rule main = (item ";")* #stream "-" $$
rule item = "x"`},
		{"recursive start rule", `rule main = (item ";")* #stream tail $$
rule item = "x"
rule tail = "-" / "(" main ")"`},
		{"stream rule below the start rule", `rule main = (item ";")* #stream other $$
rule item = "x"
rule other = ("-")+ #stream`},
	} {
		prog := mustCompile(t, c.src)
		for _, b := range depthBackends {
			count := 0
			err := prog.ParseStreamWith("main", strings.NewReader("x;x;x;-"), func(*core.Node) error { count++; return nil }, ParseOptions{Backend: b})
			if err != nil || count != 3 {
				t.Errorf("%s/%v: emitted %d, error %v; want 3", c.name, b, count, err)
			}
		}
	}
}
