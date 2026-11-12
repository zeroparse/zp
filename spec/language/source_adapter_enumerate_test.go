package language_test

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"zeroparse.dev/zp"
)

// enumerationAlphabet are the tokens that the exhaustive enumeration combines: each decides how a
// neighbour is read (a bracket after a name is an index or a class, a quote after a letter is a suffix
// or a literal, a hyphen is a range or an operator).
var enumerationAlphabet = []string{
	"a", "T", "node", "x.y", "$x", "$$", "1", "0x1", "1.5", "\"s\"", "'c'", "\"s\"i", "[a-z]", "[^a]",
	"(", ")", "{", "}", "[", "]", "=", "=>", ":=", ":", ";", ",", ".", "/", "*", "+", "?", "&", "!", "~", "-", "<", "|",
	"@", "#cut", "#atomic", "_", "_|_", "...", "//c\n", "/*c*/", "\n", "rule", "type", "var", "nil", "true",
}

// enumerationContexts are the heads after which the enumerated tokens stand.
// A context is its head and, after a NUL, the tail that closes it.
var enumerationContexts = []string{"rule a = \x00", "rule a = b => \x00", "type T = \x00", "type T = {\x00}", "rule a = #operators {\x00}",
	"@a(\x00)\nrule x = y", "rule a = b #\x00", "rule a = b #\x00()", "rule a = b => {\x00}", "rule a = (\x00)", "rule a = b[\x00]", "\x00"}

// TestCanonicalFrontendEnumeration compares the frontends on every combination of up to
// enumerationLength tokens of the alphabet after each context, joined by a space and directly.
func TestCanonicalFrontendEnumeration(t *testing.T) {
	acceptedTotal.Store(0)
	t.Run("contexts", func(t *testing.T) {
		for _, ctx := range enumerationContexts {
			for _, join := range []string{" ", ""} {
				t.Run(fmt.Sprintf("%q join %q", ctx, join), func(t *testing.T) {
					t.Parallel()
					enumerate(t, ctx, join)
				})
			}
		}
	})
	// A comparison where every source is rejected shows nothing about the AST.
	if n := acceptedTotal.Load(); n < 1000 {
		t.Errorf("only %d enumerated sources are accepted", n)
	}
}

func enumerate(t *testing.T, ctx, join string) {
	failed, accepted, total := 0, 0, 0
	stats := newHarmStats()
	var walk func(ctx, join string, toks []string, depth int)
	walk = func(ctx, join string, toks []string, depth int) {
		if failed >= 40 {
			return
		}
		if len(toks) > 0 {
			head, tail, _ := strings.Cut(ctx, "\x00")
			src := head + strings.Join(toks, join) + tail + "\n"
			total++
			stats.add(t, src, nil)
			if _, err := zp.ParseGrammar(src); err == nil {
				accepted++
			}
			if d := grammarDiff(t, adaptCanonical, src); d != "" {
				failed++
				t.Errorf("%q: %s", src, d)
			}
		}
		if depth == 0 {
			return
		}
		for _, tok := range enumerationAlphabet {
			walk(ctx, join, append(toks, tok), depth-1)
		}
	}
	walk(ctx, join, nil, enumerationLength)
	stats.report(t, total)
	acceptedTotal.Add(int64(accepted))
	t.Logf("%d of %d sources are accepted", accepted, total)
}

// acceptedTotal counts the enumerated sources that the hand-written parser accepts.
var acceptedTotal atomic.Int64
