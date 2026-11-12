package complexity_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// TestViewOrigins checks that the module a view is built from gives back the rules it is built
// from: every node has an origin, so a rewrite keeps the expression of every node it does not
// change, and a view that rewrites nothing writes the rules as they are.
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
			for _, r := range cs.Module.Rules {
				core.Walk(r.Body, func(n core.Node) {
					if _, ok := cs.Origin[n]; !ok {
						missing[fmt.Sprintf("%T", n)]++
					}
				})
			}
			v := complexity.Build(viewSource(cs), complexity.Options{}, nil)
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
			want, applied, err := compile.CostViewRules(g, false, []string{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(applied) != 0 {
				t.Errorf("rewrites with no law: %v", applied)
			}
			cs := sourceOf(t, src)
			v := complexity.Build(viewSource(cs), complexity.Options{}, nil)
			rules, err := v.Grammar(cs.Rules)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := formatRules(rules), formatRules(want); got != want {
				t.Errorf("view differs:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

// TestViewGrammarNeedsOrigins is the negative control of TestViewOrigins: without the origin of
// a node it cannot write, Grammar fails instead of writing something else.
func TestViewGrammarNeedsOrigins(t *testing.T) {
	cs := sourceOf(t, "rule main = \"a\" \"b\"*\n")
	v := complexity.Build(viewSource(cs), complexity.Options{}, nil)
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
