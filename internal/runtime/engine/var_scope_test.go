package engine

import (
	"testing"

	"zeroparse.dev/zp/internal/compile"
)

// TestCalleeBindingsUndoneAtReturn checks that a binding made in a called rule is not visible to
// its caller after the call returns ("Scope" and "Immutability" in spec/language/predicates.md),
// with and without memoization, in every backend, and in the element of a run site, at the
// thresholds where its walks take records: the records of a run site leave out the bindings of
// the rules its element calls, which rests on this.
func TestCalleeBindingsUndoneAtReturn(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, src string
		run       bool // the element of a run site calls the rule that binds
		ok, bad   []string
	}{
		{"caller after a call", `var x int = 0
rule main = (c {x == 0} "a")* $$
rule c = {x := 1} "b"`, false, []string{"", "ba", "bababa"}, []string{"bb", "b"}},
		{"caller between calls", `var x int = 0
rule main = c {x == 0} c {x == 0} $$
rule c = {x := 1} "b" {x == 1} / {x := 2} "a" {x == 2}`, false, []string{"bb", "ab", "aa"}, []string{"b", "abc"}},
		{"after the walk of a run site", `var x int = 0
rule main = (r "!" / [ab])* {x == 0} $$
rule r = c*
rule c = {x := 1} "a" {x == 1} / {x := 2} "b" {x == 2}`, true, []string{"ab!", "abab", "aab!ba!b"}, []string{"ab!c"}},
		{"inside the element of a run site", `var x int = 0
rule main = (r "!" / [ab])* $$
rule r = (c {x == 0})*
rule c = {x := 1} "a" / {x := 2} "b"`, true, []string{"ab!", "abab", "aab!ba!b"}, []string{"ab!c"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			run := func(t *testing.T) {
				for _, opts := range []compile.Options{{}, {DisableMemo: true}} {
					prog := mustCompile(t, c.src, opts)
					for _, in := range c.ok {
						if _, err := prog.ParseWith("main", in, ParseOptions{}); err != nil {
							t.Errorf("memo=%v %q: %v", !opts.DisableMemo, in, err)
						}
						checkBackends(t, prog, "main", in)
					}
					for _, in := range c.bad {
						if _, err := prog.ParseWith("main", in, ParseOptions{}); err == nil {
							t.Errorf("memo=%v %q: parsed", !opts.DisableMemo, in)
						}
						checkBackends(t, prog, "main", in)
					}
				}
			}
			if c.run {
				atHotSpans(t, run)
			} else {
				run(t)
			}
		})
	}
}
