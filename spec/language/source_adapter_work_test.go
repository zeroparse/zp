//go:build zp_workcount

package language_test

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/frontend/parser"
)

// scaleWorkSources are the sources of scaleAllocSources and valid grammars whose declarations have
// comments, attributes and types, which the builder of the grammar looks up for each declaration.
var scaleWorkSources = append([]struct {
	name string
	make func(n int) string
}{
	{"rules with header comments", func(n int) string { return strings.Repeat("rule /* a */ a /* b */ = x // c\n", n/32) }},
	{"rules with attributes", func(n int) string { return strings.Repeat("// d\n@inline\nrule a = x\n", n/24) }},
	{"types with comments", func(n int) string { return strings.Repeat("type /* a */ T = { /* b */ X string }\n", n/40) }},
	{"aliases and variables", func(n int) string { return strings.Repeat("type /* a */ A ~ /* b */ T\nvar /* c */ v = 1\n", n/40) }},
}, scaleAllocSources...)

// workCost returns the units of work that run counts in the lexer and in the builder of the grammar
// (see parser.Work): the code points of the source that the lexer reads, and the code points,
// comments, comment gaps and tokens that the builder, its layout replay and the error messages read.
// The counts do not depend on the time, the load or the allocations, so a loop that rescans the source
// or the tokens is seen even when it allocates nothing. The test does not run in parallel, so that the
// counters hold the work of this parse alone.
func workCost(run func()) (lexer, builder uint64) {
	parser.ResetWork()
	run()
	return parser.Work()
}

// TestSourceAPIWorkScalesLinearly checks that the lexer and the builder of the grammar do work that
// grows linearly with the source: for 8 times as large a source, the work that each counts grows at
// most scaleLimit times, through Parse and through ParsePartial. The lexer reads every code point, so
// its work is at least the size of the source; work under it was not measured.
func TestSourceAPIWorkScalesLinearly(t *testing.T) {
	apis := []struct {
		name  string
		parse func(string)
	}{
		{"Parse", func(src string) { parser.Parse(src) }},
		{"ParsePartial", func(src string) { parser.ParsePartial(src) }},
	}
	for _, api := range apis {
		for _, c := range scaleWorkSources {
			t.Run(api.name+"/"+c.name, func(t *testing.T) {
				small, large := c.make(2000), c.make(2000*scaleFactor)
				l1, b1 := workCost(func() { api.parse(small) })
				l2, b2 := workCost(func() { api.parse(large) })
				if l1 < uint64(len(small)) || l2 < uint64(len(large)) {
					t.Fatalf("the lexer read %d and %d units of sources of %d and %d bytes: the work was not counted", l1, l2, len(small), len(large))
				}
				if b1 == 0 || b2 == 0 {
					t.Fatalf("the builder read %d and %d units: the work was not counted", b1, b2)
				}
				lr, br := float64(l2)/float64(l1), float64(b2)/float64(b1)
				t.Logf("lexer %d -> %d (%.2f), builder %d -> %d (%.2f)", l1, l2, lr, b1, b2, br)
				if lr > scaleLimit {
					t.Errorf("the work of the lexer grows %.1f times for %d times as large a source (limit %d)", lr, scaleFactor, scaleLimit)
				}
				if br > scaleLimit {
					t.Errorf("the work of the builder grows %.1f times for %d times as large a source (limit %d)", br, scaleFactor, scaleLimit)
				}
			})
		}
	}
}
