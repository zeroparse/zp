package complexity_test

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/complexity"
	"zeroparse.dev/zp/internal/grammaranalysis"
)

var update = flag.Bool("update", false, "rewrite testdata/cost-view.golden")

// TestViewSnapshot pins the cost view of every corpus grammar: for the view that the complexity
// classifier reads, the rewrites it counts, the rules it changes, and each expression it builds
// with its position and the source text that messages quote for it; for the view of matching
// (labels dropped, calls inlined), which is much larger, the rewrites it counts and a digest of
// the same text. A change of the view shows here, and -update rewrites the snapshot.
func TestViewSnapshot(t *testing.T) {
	srcs := corpus(t)
	var b strings.Builder
	for _, p := range slices.Sorted(maps.Keys(srcs)) {
		cs := sourceOf(t, srcs[p])
		for _, matching := range []bool{false, true} {
			v := complexity.Build(viewSource(cs), complexity.Options{Labels: matching, Inline: matching}, nil)
			rules, text, err := v.GrammarText(cs.Rules)
			if err != nil {
				t.Fatalf("%s: %v", p, err)
			}
			applied := map[string]int{}
			for l, n := range v.Applied {
				applied[string(l)] = n
			}
			s := viewSnapshot(cs.Rules, rules, text, applied)
			fmt.Fprintf(&b, "== %s matching=%v\n", p, matching)
			if matching {
				head, _, _ := strings.Cut(s, "\n")
				fmt.Fprintf(&b, "%s\nsha256 %x\n", head, sha256.Sum256([]byte(s)))
				continue
			}
			b.WriteString(s)
		}
	}
	const golden = "testdata/cost-view.golden"
	if *update {
		if err := os.WriteFile(golden, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if got := b.String(); got != string(want) {
		gl, wl := strings.Split(got, "\n"), strings.Split(string(want), "\n")
		for i := range min(len(gl), len(wl)) {
			if gl[i] != wl[i] {
				t.Fatalf("the cost view differs from %s at line %d:\n got  %s\n want %s\n(go test -run TestViewSnapshot -update rewrites it)", golden, i+1, gl[i], wl[i])
			}
		}
		t.Fatalf("the cost view differs from %s: %d lines, want %d", golden, len(gl), len(wl))
	}
}

// viewSnapshot writes the cost view of one grammar for the snapshot test: the rewrites it counts,
// the rules it changes, and each expression it builds with its position and the source text it
// stands for. source are the rules the view is built from and rules the view's rules.
func viewSnapshot(source, rules []*grammar.RuleDef, text map[grammar.Expr]string, applied map[string]int) string {
	var b strings.Builder
	laws := slices.Sorted(maps.Keys(applied))
	b.WriteString("applied:")
	for _, l := range laws {
		fmt.Fprintf(&b, " %s=%d", l, applied[l])
	}
	b.WriteString("\n")
	src := map[grammar.Expr]bool{}
	for _, r := range source {
		grammaranalysis.WalkExpr(r.Expr, func(x grammar.Expr) { src[x] = true })
	}
	for i, r := range rules {
		if r.Expr == source[i].Expr {
			continue
		}
		fmt.Fprintf(&b, "rule %s = %s\n", r.Name, grammar.FormatExpr(r.Expr))
		var built []string
		grammaranalysis.WalkExpr(r.Expr, func(x grammar.Expr) {
			if src[x] {
				return
			}
			pos := grammar.Pos{}
			if s, ok := x.(interface{ Source() *grammar.SourceInfo }); ok && s.Source() != nil {
				pos = s.Source().Diagnostic()
			}
			built = append(built, fmt.Sprintf("  %d:%d %s = %q", pos.Line, pos.Col, grammar.FormatExpr(x), text[x]))
		})
		slices.Sort(built)
		b.WriteString(strings.Join(built, "\n"))
		if len(built) > 0 {
			b.WriteString("\n")
		}
	}
	return b.String()
}
