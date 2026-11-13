package complexity

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
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

// testClassifierRules returns the rules the classifier reads for g (classifierRules).
func testClassifierRules(t *testing.T, g *grammar.Grammar) (*grammar.Grammar, []*grammar.RuleDef) {
	t.Helper()
	g, rules, err := classifierRules(g, nil)
	if err != nil {
		t.Fatal(err)
	}
	return g, rules
}

// testRuleFacts returns the rule facts of rules, the rules of g, for a test of the normal view,
// which may take rules that the classifier would canonicalize first: the facts are those of the
// canonical rules of g (testClassifierRules), by name. The view tests also take grammars that the
// compiler rejects (a predicate that reads a capture out of its scope), of which Core IR cannot
// be built; for those it takes the analyses of the grammar, with the leaders and the rules off
// every cycle that are not transient memoized (none of those grammars has #missing).
func testRuleFacts(t *testing.T, g *grammar.Grammar, rules []*grammar.RuleDef) *ruleFacts {
	t.Helper()
	canon, canonRules := testClassifierRules(t, g)
	if rf, err := classifierFacts(canon, canonRules, nil); err == nil {
		return rf
	} else if _, cerr := compileStatic(starttest.MarkAll(g)); cerr == nil {
		t.Fatalf("no Core IR of a grammar that compiles: %v", err)
	}
	an := compile.AnalyzeControlled(rules, nil)
	transient := compile.TransientRules(rules, nil)
	memo := map[string]bool{}
	for _, r := range rules {
		memo[r.Name] = an.Leaders[r.Name] || !an.Cyclic[r.Name] && !transient[r.Name]
	}
	return &ruleFacts{an: an, memo: memo, members: map[string]bool{}, vars: compile.RuleVariables(rules)}
}

// renderRuleFacts writes the rule facts of rules that the analyses of the grammar compute too, one
// line per rule, with the cycles numbered in the order of their first rule, so that two numberings
// of the same cycles render alike. The memoized rules are checked against the compiled rules
// instead (TestComplexityMemoPolicy).
func renderRuleFacts(rules []*grammar.RuleDef, rf *ruleFacts) string {
	var b strings.Builder
	cycles := map[int]int{}
	for _, r := range rules {
		n := r.Name
		cycle := 0
		if rf.an.Cyclic[n] {
			c, ok := cycles[rf.an.Cycle[n]]
			if !ok {
				c = len(cycles) + 1
				cycles[rf.an.Cycle[n]] = c
			}
			cycle = c
		}
		fmt.Fprintf(&b, "%s nullable=%v succeeds=%v leader=%v cyclic=%v cycle=%d positional=%v recovering=%v vars=%v\n",
			n, rf.an.Nullable[n], rf.an.Succeeding[n], rf.an.Leaders[n], rf.an.Cyclic[n], cycle, rf.an.Positional[n],
			rf.an.Recovering[n] != "", rf.vars[n])
	}
	return b.String()
}

// astRuleFacts returns the rule facts of rules as the analyses of the grammar compute them.
func astRuleFacts(rules []*grammar.RuleDef) *ruleFacts {
	return &ruleFacts{an: compile.AnalyzeControlled(rules, nil), vars: compile.RuleVariables(rules)}
}

// TestClassifierFactsMatchAnalysis requires the rule facts that the classifier reads from the
// Core IR module of a grammar to equal those that the analyses of the grammar compute, for every
// grammar of the corpus, as written and with every rule a start rule.
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
				g, rules := testClassifierRules(t, g)
				rf := testRuleFacts(t, g, rules)
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
	g, rules := testClassifierRules(t, g)
	rf := testRuleFacts(t, g, rules)
	want := renderRuleFacts(rules, rf)
	for _, flip := range []func(*ruleFacts){
		func(f *ruleFacts) { f.an.Leaders = map[string]bool{} },
		func(f *ruleFacts) { f.vars = map[string][]string{"b": {"v"}} },
		func(f *ruleFacts) { f.an.Cyclic["b"], f.an.Cycle["b"] = true, f.an.Cycle["a"]+1 },
	} {
		mutant := *rf
		an := *rf.an
		an.Leaders, an.Cyclic, an.Cycle = maps.Clone(an.Leaders), maps.Clone(an.Cyclic), maps.Clone(an.Cycle)
		mutant.an = &an
		flip(&mutant)
		if renderRuleFacts(rules, &mutant) == want {
			t.Errorf("a changed fact renders alike:\n%s", want)
		}
	}
	if !slices.ContainsFunc(rules, func(r *grammar.RuleDef) bool { return rf.an.Leaders[r.Name] }) {
		t.Errorf("the control grammar has no leader:\n%s", want)
	}
}
