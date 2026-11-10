package build

import (
	"regexp"
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	"zeroparse.dev/zp/internal/factstest"
	"zeroparse.dev/zp/internal/ir/core"
)

// TestTwinBodies checks that the value-free twin of every rule of the corpus grammars has the
// body of its rule: the two print alike once a call names its rule without telling the rule
// from its twin, so the twin evaluates the same expressions as the rule and differs only in the
// values that it builds (Evaluations of Rules in spec/runtime/parsing.md).
func TestTwinBodies(t *testing.T) {
	callee := regexp.MustCompile(`call \d+:`)
	twins := 0
	for _, path := range factstest.Corpus {
		g, err := compiletest.Parse(factstest.Read(t, path))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		prog, err := compiletest.Compile(g, compile.Options{})
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		m, err := CoreFrom(prog, nil)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, r := range m.Rules {
			if r.Twin == nil {
				continue
			}
			twins++
			rule := callee.ReplaceAllString(core.PrintNode(m, r.Body, false), "call ")
			twin := callee.ReplaceAllString(core.PrintNode(m, r.Twin.Body, false), "call ")
			if rule != twin {
				t.Errorf("%s: the twin of %s has another body:\n%s\nthe rule:\n%s", path, r.Name, twin, rule)
			}
		}
	}
	if twins == 0 {
		t.Fatal("no corpus rule has a twin")
	}
}

// TestTwinMemoPolicy checks that the value-free twin of every rule of the corpus grammars is
// memoized exactly when its rule is, in ordinary parses and in Documents alike (Evaluations of
// Rules in spec/runtime/parsing.md), so that a generated parser treats the calls of a twin as the
// calls of its rule even where the twin is called from fewer places.
func TestTwinMemoPolicy(t *testing.T) {
	twins := 0
	for _, path := range factstest.Corpus {
		g, err := compiletest.Parse(factstest.Read(t, path))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		prog, err := compiletest.Compile(g, compile.Options{})
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		m, err := CoreFrom(prog, nil)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, r := range m.Rules {
			if r.Twin == nil {
				continue
			}
			twins++
			if a, b := r.Policy, r.Twin.Policy; a.Memo != b.Memo || a.Transient != b.Transient || a.Leader != b.Leader {
				t.Errorf("%s: the twin of %s has the policy memo=%v transient=%v leader=%v, the rule memo=%v transient=%v leader=%v",
					path, r.Name, b.Memo, b.Transient, b.Leader, a.Memo, a.Transient, a.Leader)
			}
		}
	}
	if twins == 0 {
		t.Fatal("no corpus rule has a twin")
	}
}
