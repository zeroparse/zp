package compile

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/desugar"
	"zeroparse.dev/zp/internal/factstest"
	"zeroparse.dev/zp/internal/starttest"
	"zeroparse.dev/zp/internal/syntax"
)

// classifierCorpus returns the sources of the grammars the classifier's tests read: the pinned
// corpus, the test grammars and the examples, by path.
func classifierCorpus(t *testing.T) map[string]string {
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

// classifierRules returns the rules the classifier reads for g: inline actions lowered, attributes
// normalized and each rule once.
func classifierRules(t *testing.T, g *grammar.Grammar) (*grammar.Grammar, []*grammar.RuleDef) {
	t.Helper()
	if desugar.HasInlineAction(g, nil) {
		lowered, errs := desugar.LowerInlineActionsControlled(g, desugar.InlineNames{}, nil)
		if errs != nil {
			t.Fatal(errs)
		}
		g = lowered
	}
	g, err := desugar.NormalizeAttributesControlled(g, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	rules, errs := desugar.UniqueRules(g, nil)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	return g, rules
}

// renderRuleFacts writes the rule facts of rules one line per rule, with the cycles numbered in
// the order of their first rule, so that two numberings of the same cycles render alike.
func renderRuleFacts(rules []*grammar.RuleDef, rf *ruleFacts) string {
	var b strings.Builder
	cycles := map[int]int{}
	for _, r := range rules {
		n := r.Name
		cycle := 0
		if rf.an.cyclic[n] {
			c, ok := cycles[rf.an.cycle[n]]
			if !ok {
				c = len(cycles) + 1
				cycles[rf.an.cycle[n]] = c
			}
			cycle = c
		}
		fmt.Fprintf(&b, "%s nullable=%v succeeds=%v leader=%v cyclic=%v cycle=%d positional=%v recovering=%v memo=%v member=%v vars=%v\n",
			n, rf.an.nullable[n], rf.an.succeeding[n], rf.an.leaders[n], rf.an.cyclic[n], cycle, rf.an.positional[n],
			rf.an.recovering[n] != "", rf.memo[n], rf.members[n], rf.vars[n])
	}
	return b.String()
}

// astRuleFacts returns the rule facts of rules as the analyses of the grammar compute them.
func astRuleFacts(rules []*grammar.RuleDef) *ruleFacts {
	an := AnalyzeControlled(rules, nil)
	memo, members := memoizedRules(rules, an, nil)
	return &ruleFacts{an: an, memo: memo, members: members, vars: ruleVariables(rules)}
}

// TestClassifierFactsMatchAnalysis requires the rule facts that the classifier reads from the
// Core IR module of a grammar to equal those that the analyses of the grammar compute, for every
// grammar of the corpus, as written and with every rule a start rule (which makes rules
// memoized that are otherwise transient).
func TestClassifierFactsMatchAnalysis(t *testing.T) {
	t.Parallel()
	for path, src := range classifierCorpus(t) {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			parsed, err := syntax.Parse(src)
			if err != nil {
				t.Fatal(err)
			}
			for _, g := range []*grammar.Grammar{parsed, starttest.MarkAll(parsed)} {
				g, rules := classifierRules(t, g)
				rf, err := classifierFacts(g, rules, nil)
				if err != nil {
					t.Fatal(err)
				}
				if got, want := renderRuleFacts(rules, rf), renderRuleFacts(rules, astRuleFacts(rules)); got != want {
					t.Errorf("facts differ\nfacts:\n%s\nanalysis:\n%s", got, want)
				}
			}
		})
	}
}

// TestClassifierFactsDetectsDifference is the negative control of TestClassifierFactsMatchAnalysis:
// changing one fact of one rule must change the rendering.
func TestClassifierFactsDetectsDifference(t *testing.T) {
	g, err := syntax.Parse("@start\nrule a = a \"x\" / b\nrule b = \"y\"\n")
	if err != nil {
		t.Fatal(err)
	}
	g, rules := classifierRules(t, g)
	rf, err := classifierFacts(g, rules, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := renderRuleFacts(rules, rf)
	for _, flip := range []func(*ruleFacts){
		func(f *ruleFacts) { f.an.leaders = map[string]bool{} },
		func(f *ruleFacts) { f.memo["b"] = !f.memo["b"] },
		func(f *ruleFacts) { f.an.cyclic["b"], f.an.cycle["b"] = true, f.an.cycle["a"]+1 },
	} {
		mutant := *rf
		an := *rf.an
		an.leaders, an.cyclic, an.cycle = maps.Clone(an.leaders), maps.Clone(an.cyclic), maps.Clone(an.cycle)
		mutant.an, mutant.memo = &an, maps.Clone(rf.memo)
		flip(&mutant)
		if renderRuleFacts(rules, &mutant) == want {
			t.Errorf("a changed fact renders alike:\n%s", want)
		}
	}
	if !slices.ContainsFunc(rules, func(r *grammar.RuleDef) bool { return rf.an.leaders[r.Name] }) {
		t.Errorf("the control grammar has no leader:\n%s", want)
	}
}
