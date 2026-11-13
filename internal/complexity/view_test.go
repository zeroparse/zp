package complexity_test

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/complexity"
	"zeroparse.dev/zp/internal/factstest"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/syntax"
)

// corpus returns the sources of the grammars the classifier's tests read: the pinned corpus, the
// test grammars and the examples, by path.
func corpus(t *testing.T) map[string]string {
	t.Helper()
	srcs := map[string]string{}
	for _, p := range factstest.Corpus {
		srcs[p] = factstest.Read(t, p)
	}
	for _, pattern := range []string{"../../testdata/grammars/*/*.zp", "../../examples/*/*.zp"} {
		paths, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range paths {
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			srcs[strings.TrimPrefix(p, "../../")] = string(b)
		}
	}
	return srcs
}

// sourceOf returns the cost source of the grammar src.
func sourceOf(t *testing.T, src string) *compile.CostSource {
	t.Helper()
	g, err := syntax.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	cs, err := compile.NewCostSource(g, nil)
	if err != nil {
		t.Fatal(err)
	}
	return cs
}

func viewSource(cs *compile.CostSource) *complexity.Source {
	return &complexity.Source{Module: cs.Module, Origin: cs.Origin, Facts: cs.Facts}
}

// formatRules formats the body of each rule, one rule per line.
func formatRules(rules []*grammar.RuleDef) string {
	var b strings.Builder
	for _, r := range rules {
		fmt.Fprintf(&b, "%s = %s\n", r.Name, grammar.FormatExpr(r.Expr))
	}
	return b.String()
}

// innerAttrNodes returns the nodes of the module that the attributes of an expression of several
// attributes are built into, below the outermost one, which has the expression as its origin: one
// node per attribute, a Label for @error and @diagnostic, a Recover for #recover and a Missing for
// #missing, and the Stream of the repetition for #stream.
func innerAttrNodes(cs *compile.CostSource) map[core.Node]bool {
	inner := map[core.Node]bool{}
	for _, r := range cs.Module.Rules {
		core.Walk(r.Body, func(n core.Node) {
			e, ok := cs.Origin[n].(*grammar.Attributed)
			if !ok || len(e.Attrs) < 2 {
				return
			}
			operand := func(x core.Node) core.Node {
				switch x := x.(type) {
				case *core.Label:
					return x.Body
				case *core.Recover:
					return x.Body
				case *core.Missing:
					return x.Lit
				case *core.Stream:
					return x.Repeat
				}
				return nil
			}
			x := n
			for i := len(e.Attrs) - 1; i >= 0 && x != nil; i-- {
				if e.Attrs[i].Name == "stream" {
					continue
				}
				if x != n {
					inner[x] = true
				}
				x = operand(x)
			}
			if _, ok := x.(*core.Stream); ok && x != n {
				inner[x] = true
			}
		})
	}
	return inner
}

// TestViewOrigins checks that the module a view is built from gives back the rules it is built
// from: every node has an origin but the inner nodes of an expression of several attributes
// (innerAttrNodes), so a rewrite keeps the expression of every node it does not change, and a
// view that rewrites nothing writes the rules as they are.
func TestViewOrigins(t *testing.T) {
	srcs := corpus(t)
	paths := make([]string, 0, len(srcs))
	for p := range srcs {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	missing := map[string]int{}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			cs := sourceOf(t, srcs[p])
			inner := innerAttrNodes(cs)
			for _, r := range cs.Module.Rules {
				core.Walk(r.Body, func(n core.Node) {
					if _, ok := cs.Origin[n]; !ok && !inner[n] {
						missing[fmt.Sprintf("%T", n)]++
					}
				})
			}
			v := complexity.Build(viewSource(cs), complexity.Options{Laws: []complexity.Law{}}, nil)
			rules, err := v.Grammar(cs.Rules)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := formatRules(rules), formatRules(cs.Rules); got != want {
				t.Errorf("identity view differs:\n%s\nwant:\n%s", got, want)
			}
		})
	}
	if len(missing) > 0 {
		t.Errorf("nodes without an origin, by kind: %v", missing)
	}
}

// TestViewMatchesNoLaws is the base of the differential tests of the laws: with no law applied,
// the view on the grammar's rules (compile.CostViewRules) rewrites nothing and writes the rules as
// the identity view does.
func TestViewMatchesNoLaws(t *testing.T) {
	srcs := corpus(t)
	for p, src := range srcs {
		t.Run(p, func(t *testing.T) {
			t.Parallel()
			g, err := syntax.Parse(src)
			if err != nil {
				t.Fatal(err)
			}
			rv, err := compile.CostViewRules(g, false, []string{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(rv.Applied) != 0 {
				t.Errorf("rewrites with no law: %v", rv.Applied)
			}
			cs := sourceOf(t, src)
			v := complexity.Build(viewSource(cs), complexity.Options{Laws: []complexity.Law{}}, nil)
			rules, err := v.Grammar(cs.Rules)
			if err != nil {
				t.Fatal(err)
			}
			if d := diffRules(rules, rv.Rules); d != "" {
				t.Errorf("view differs:\n%s", d)
			}
		})
	}
}

// TestViewGrammarNeedsOrigins is the negative control of TestViewOrigins: without the origin of
// a node it cannot write, Grammar fails instead of writing something else.
func TestViewGrammarNeedsOrigins(t *testing.T) {
	cs := sourceOf(t, "rule main = \"a\" \"b\"*\n")
	v := complexity.Build(viewSource(cs), complexity.Options{Laws: []complexity.Law{}}, nil)
	for n := range v.Origin {
		switch n.(type) {
		case *core.Seq, *core.Lit:
			delete(v.Origin, n)
		}
	}
	if _, err := v.Grammar(cs.Rules); err == nil {
		t.Fatal("Grammar wrote a literal that it has no origin of")
	}
}

// ported are the rules that the view built on Core IR applies.
var ported = []complexity.Law{complexity.N1, complexity.N2, complexity.N3, complexity.N4, complexity.N5,
	complexity.N6, complexity.N8, complexity.N9, complexity.N10, complexity.N11, complexity.N12}

// offCorpus are the ported rules that no grammar of the corpus applies. The grammars of the
// normal view's unit tests apply them (compile.TestCostViewMatchesCases).
var offCorpus = []complexity.Law{complexity.N3, complexity.N5, complexity.N9, complexity.N10}

// TestViewLabelsAndInlineNeedOptions checks that N7 and N13 apply only when Labels and Inline
// select them, whatever Laws holds.
func TestViewLabelsAndInlineNeedOptions(t *testing.T) {
	cs := sourceOf(t, "rule main = a @error(\"e\") \"x\"\nrule a = \"y\" \"z\"\n")
	all := []complexity.Law{complexity.N7, complexity.N13}
	for _, opts := range []complexity.Options{{}, {Laws: all}, {Labels: true, Inline: true}} {
		v := complexity.Build(viewSource(cs), opts, nil)
		on := opts.Labels && opts.Inline
		for _, l := range all {
			if got := v.Applied[l] > 0; got != on {
				t.Errorf("options %+v: %s applied %v, want %v", opts, l, got, on)
			}
		}
	}
}

// lawSet is a set of rules that the differential test compares the views with, for the
// classifier's view or, with matching, for the view of matching (compile.NormalView with
// matching).
type lawSet struct {
	laws     []complexity.Law
	matching bool
}

// lawSets returns the sets that the differential test compares: the ported rules together and
// each alone for the classifier's view, and the ported rules with N7 and N13 together, N7 alone
// and N13 alone for the view of matching. The first set holds every rule compared.
func lawSets() []lawSet {
	all := append(slices.Clone(ported), complexity.N7, complexity.N13)
	sets := []lawSet{{all, true}, {ported, false}}
	for _, l := range ported {
		sets = append(sets, lawSet{[]complexity.Law{l}, false})
	}
	return append(sets, lawSet{[]complexity.Law{complexity.N7}, true}, lawSet{[]complexity.Law{complexity.N13}, true})
}

// TestViewMatchesLaws checks the view against the view on the grammar's rules
// (compile.CostViewRules) on the corpus: with each set of lawSets, both count the same rewrites of
// each rule and write the same rules.
func TestViewMatchesLaws(t *testing.T) {
	sets := lawSets()
	srcs := corpus(t)
	seen := map[complexity.Law]int{}
	var mu sync.Mutex
	t.Run("corpus", func(t *testing.T) {
		for p, src := range srcs {
			t.Run(p, func(t *testing.T) {
				t.Parallel()
				g, err := syntax.Parse(src)
				if err != nil {
					t.Fatal(err)
				}
				cs := sourceOf(t, src)
				for _, set := range sets {
					laws, matching := set.laws, set.matching
					names := make([]string, len(laws))
					for i, l := range laws {
						names[i] = string(l)
					}
					rv, err := compile.CostViewRules(g, matching, names, nil)
					if err != nil {
						t.Fatal(err)
					}
					v := complexity.Build(viewSource(cs), complexity.Options{Labels: matching, Inline: matching, Laws: laws}, nil)
					applied := map[string]int{}
					for l, n := range v.Applied {
						applied[string(l)] = n
					}
					if !maps.Equal(applied, rv.Applied) {
						t.Errorf("laws %v: applied %v, want %v", names, applied, rv.Applied)
					}
					if got, want := v.BuiltTexts(), rv.BuiltTexts(); !slices.Equal(got, want) {
						t.Errorf("laws %v: built texts\n  %q\nwant\n  %q", names, got, want)
					}
					rules, err := v.Grammar(cs.Rules)
					if err != nil {
						t.Fatalf("laws %v: %v", names, err)
					}
					if d := diffRules(rules, rv.Rules); d != "" {
						t.Errorf("laws %v: view differs:\n%s", names, d)
					}
					if len(laws) > 1 {
						mu.Lock()
						for l, n := range v.Applied {
							seen[l] += n
						}
						mu.Unlock()
					}
				}
			})
		}
	})
	t.Logf("rewrites on the corpus: %v", seen)
	for _, l := range sets[0].laws {
		switch off := slices.Contains(offCorpus, l); {
		case seen[l] == 0 && !off:
			t.Errorf("the corpus applies no %s: the comparison does not test it", l)
		case seen[l] > 0 && off:
			t.Errorf("the corpus applies %s, which offCorpus lists", l)
		}
	}
}

// diffRules returns the rules whose bodies differ in got and want, formatted, or "".
func diffRules(got, want []*grammar.RuleDef) string {
	var b strings.Builder
	for i := range max(len(got), len(want)) {
		if i >= len(got) || i >= len(want) {
			fmt.Fprintf(&b, "%d rules, want %d\n", len(got), len(want))
			break
		}
		g, w := grammar.FormatExpr(got[i].Expr), grammar.FormatExpr(want[i].Expr)
		if g != w {
			fmt.Fprintf(&b, "%s:\n  got  %s\n  want %s\n", got[i].Name, g, w)
		}
	}
	return b.String()
}
