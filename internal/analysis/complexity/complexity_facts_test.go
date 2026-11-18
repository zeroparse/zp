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
	"zeroparse.dev/zp/internal/frontend/parser"
	"zeroparse.dev/zp/internal/testing/factstest"
	"zeroparse.dev/zp/internal/testing/starttest"
)

// classifierCorpus returns the sources of the grammars the classifier's tests read: the pinned
// corpus, the test grammars and the examples, by path.
func classifierCorpus(t *testing.T) map[string]string {
	t.Helper()
	srcs := map[string]string{}
	for _, p := range factstest.Corpus {
		srcs[p] = factstest.Read(t, p)
	}
	for _, pattern := range []string{"../../../testdata/grammars/*/*.zp", "../../../examples/*/*.zp"} {
		paths, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range paths {
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			srcs[strings.TrimPrefix(p, "../../../")] = string(b)
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
// compiler rejects (a predicate that reads a capture out of its scope); of those Core IR cannot be
// built, and the test fails.
func testRuleFacts(t *testing.T, g *grammar.Grammar, rules []*grammar.RuleDef) *ruleFacts {
	t.Helper()
	canon, canonRules := testClassifierRules(t, g)
	rf, err := classifierFacts(canon, canonRules, nil)
	if err != nil {
		t.Fatalf("no Core IR of the rules: %v", err)
	}
	return rf
}

// renderRuleFacts writes the rule facts of rules that the compiled rules carry too, one line per
// rule, with the cycles numbered in the order of their first rule, so that two numberings of the
// same cycles render alike. The memoized rules are checked against the compiled rules instead
// (TestComplexityMemoPolicy).
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
		fmt.Fprintf(&b, "%s leader=%v member=%v cyclic=%v cycle=%d positional=%v vars=%v\n",
			n, rf.an.Leaders[n], rf.members[n], rf.an.Cyclic[n], cycle, rf.an.Positional[n], rf.vars[n])
	}
	return b.String()
}

// programRuleFacts returns the rule facts of rules as the rules of prog, the compiled grammar,
// carry them, or false if prog lacks one of rules.
func programRuleFacts(prog *compile.Program, rules []*grammar.RuleDef) (*ruleFacts, bool) {
	an := newRuleAnalysis(nil)
	rf := &ruleFacts{an: an, members: map[string]bool{}, vars: map[string][]string{}}
	for _, rd := range rules {
		r := prog.ByName[rd.Name]
		if r == nil {
			return nil, false
		}
		an.Leaders[r.Name], an.Cyclic[r.Name], an.Positional[r.Name] = r.Leader, r.Leader || r.Member, r.Positional
		if r.Leader || r.Member {
			an.Cycle[r.Name] = r.Cycle
		}
		rf.members[r.Name] = r.Member && !r.Transient && !r.Missing
		if len(r.Vars) > 0 {
			rf.vars[r.Name] = r.Vars
		}
	}
	return rf, true
}

// TestClassifierFactsMatchProgram requires the rule facts that the classifier reads from the
// Core IR module of its view of a grammar to equal those of the compiled rules, for every grammar
// of the corpus that compiles, as written and with every rule a start rule: the classifier and the
// compiler take the facts of the same rules.
func TestClassifierFactsMatchProgram(t *testing.T) {
	t.Parallel()
	for path, src := range classifierCorpus(t) {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			parsed, err := parser.Parse(src)
			if err != nil {
				t.Fatal(err)
			}
			for _, g := range []*grammar.Grammar{parsed, starttest.MarkAll(parsed)} {
				prog, err := compileStatic(g)
				if err != nil {
					continue
				}
				g, rules := testClassifierRules(t, g)
				rf := testRuleFacts(t, g, rules)
				want, ok := programRuleFacts(prog, rules)
				if !ok {
					t.Fatalf("a rule of the classifier is not compiled")
				}
				if got, want := renderRuleFacts(rules, rf), renderRuleFacts(rules, want); got != want {
					t.Errorf("facts differ\nfacts:\n%s\ncompiled:\n%s", got, want)
				}
			}
		})
	}
}

// TestClassifierFactsDetectsDifference is the negative control of TestClassifierFactsMatchProgram:
// changing one fact of one rule must change the rendering.
func TestClassifierFactsDetectsDifference(t *testing.T) {
	g, err := parser.Parse("@start\nrule a = a \"x\" / b\nrule b = \"y\"\n")
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
