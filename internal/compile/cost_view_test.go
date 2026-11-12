package compile

import (
	"maps"
	"slices"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/complexity"
	"zeroparse.dev/zp/internal/syntax"
)

// costViewPorted are the rules that the cost view built on Core IR (internal/complexity) applies.
var costViewPorted = []complexity.Law{complexity.N1, complexity.N2, complexity.N3, complexity.N4, complexity.N5,
	complexity.N6, complexity.N8, complexity.N9, complexity.N10, complexity.N11, complexity.N12}

// costViewLawSet is a set of rules that the differential tests compare the views with, for the
// classifier's view or, with matching, for the view of matching (NormalView with matching).
type costViewLawSet struct {
	laws     []complexity.Law
	matching bool
}

// costViewLawSets returns the sets that the differential tests compare: the ported rules together
// and each alone for the classifier's view, and the ported rules with N7 and N13 together, N7
// alone and N13 alone for the view of matching. The first set holds every rule compared.
func costViewLawSets() []costViewLawSet {
	all := append(slices.Clone(costViewPorted), complexity.N7, complexity.N13)
	sets := []costViewLawSet{{all, true}, {costViewPorted, false}}
	for _, l := range costViewPorted {
		sets = append(sets, costViewLawSet{[]complexity.Law{l}, false})
	}
	return append(sets, costViewLawSet{[]complexity.Law{complexity.N7}, true}, costViewLawSet{[]complexity.Law{complexity.N13}, true})
}

// TestCostViewMatchesCases checks the cost view built on Core IR against the view of this package
// on the grammars of the view's unit tests, which reach the conditions of the rules that the
// corpus does not: with each set of costViewLawSets, both count the same rewrites of each rule,
// write the same rules, and build expressions that stand for the same text. A grammar that the
// classifier's input cannot be built from is skipped; most can.
func TestCostViewMatchesCases(t *testing.T) {
	var srcs []string
	for _, c := range normalViewCases {
		srcs = append(srcs, "rule main = "+c.body+normalViewHelpers)
	}
	for _, c := range predicateCaptureCases {
		srcs = append(srcs, c.src+"\n")
	}
	for _, c := range cheapRuleCases {
		srcs = append(srcs, c.src+"\n")
	}
	for _, c := range valueFlowCases {
		srcs = append(srcs, c.src+"\n")
	}
	for _, c := range textCases {
		srcs = append(srcs, "rule main = "+c.body+"\n")
	}
	for _, c := range cutTupleCases {
		srcs = append(srcs, c.src)
	}
	for _, c := range valueShapeCases {
		srcs = append(srcs, c.src+"\n")
	}
	compared := 0
	seen := map[complexity.Law]int{}
	for _, src := range srcs {
		g, err := syntax.Parse(src)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		cs, err := NewCostSource(g, nil)
		if err != nil {
			t.Logf("skipped: %v\n%s", err, src)
			continue
		}
		compared++
		for _, set := range costViewLawSets() {
			laws, matching := set.laws, set.matching
			names := make([]string, len(laws))
			for i, l := range laws {
				names[i] = string(l)
			}
			rv, err := CostViewRules(g, matching, names, nil)
			if err != nil {
				t.Fatalf("%s: %v", src, err)
			}
			v := complexity.Build(&complexity.Source{Module: cs.Module, Origin: cs.Origin, Facts: cs.Facts},
				complexity.Options{Labels: matching, Inline: matching, Laws: laws}, nil)
			applied := map[string]int{}
			for l, n := range v.Applied {
				applied[string(l)] = n
				if len(laws) > 1 {
					seen[l] += n
				}
			}
			if !maps.Equal(applied, rv.Applied) {
				t.Errorf("%slaws %v: applied %v, want %v", src, names, applied, rv.Applied)
			}
			if got, want := v.BuiltTexts(), rv.BuiltTexts(); !slices.Equal(got, want) {
				t.Errorf("%slaws %v: built texts\n  %q\nwant\n  %q", src, names, got, want)
			}
			rules, err := v.Grammar(cs.Rules)
			if err != nil {
				t.Errorf("%slaws %v: %v", src, names, err)
				continue
			}
			for i, r := range rules {
				if got, want := grammar.FormatExpr(r.Expr), grammar.FormatExpr(rv.Rules[i].Expr); got != want {
					t.Errorf("%slaws %v: rule %s is\n  %s\nwant\n  %s", src, names, r.Name, got, want)
				}
			}
		}
	}
	t.Logf("%d of %d grammars compared; rewrites: %v", compared, len(srcs), seen)
	if compared < len(srcs)*9/10 {
		t.Errorf("only %d of %d grammars compared", compared, len(srcs))
	}
	for _, l := range costViewLawSets()[0].laws {
		if seen[l] == 0 {
			t.Errorf("no grammar applies %s: the comparison does not test it", l)
		}
	}
}
