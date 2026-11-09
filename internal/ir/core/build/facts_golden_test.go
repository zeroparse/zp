package build

import (
	"fmt"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	"zeroparse.dev/zp/internal/factstest"
	"zeroparse.dev/zp/internal/ir/core"
)

// TestCoreFactsGolden compares the rule facts that Core IR's analysis computes on the module of
// every corpus grammar (the nullability and the first terminal of each rule's body) with the
// facts that compilation pins (internal/factstest): they must agree on every rule.
func TestCoreFactsGolden(t *testing.T) {
	var diffs strings.Builder
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
		core.Analyze(m)
		pinned := factstest.Load(t, path)
		for _, r := range prog.RuleList {
			body := m.Rules[r.ID].Body.Attrs()
			want := pinned[r.Name]
			if want == nil {
				t.Fatalf("%s: rule %s has no pinned facts", path, r.Name)
			}
			if got := fmt.Sprint(body.Nullable); got != want["null"] {
				fmt.Fprintf(&diffs, "%s %s null core=%s facts=%s\n", path, r.Name, got, want["null"])
			}
			first := factstest.Terminal{}
			if f := body.First; f != nil {
				first = factstest.Terminal{Exp: m.Exps[f.Exp], Depth: f.Depth}
			}
			if got := first.String(); got != want["first"] {
				fmt.Fprintf(&diffs, "%s %s first core=%s facts=%s\n", path, r.Name, got, want["first"])
			}
		}
	}
	if diffs.Len() > 0 {
		t.Errorf("Core IR's facts differ from the pinned ones:\n%s", diffs.String())
	}
}

// TestRecoverAndMissingNullability checks the two nullabilities of the recovery and insertion
// nodes: a recovery succeeds without consuming input only as its body does, since a skip that
// consumes nothing is no recovery; a #missing literal does so only when insertion is enabled.
func TestRecoverAndMissingNullability(t *testing.T) {
	for _, tc := range []struct {
		src       string
		null, ins bool
	}{
		{`rule a = ("x" #recover(skip="y")) a "z" / "w"`, false, false},
		{`rule a = ("x" #recover(skip="y"*)) "z"`, false, false},
		{`rule a = ("x"? #recover(skip="y")) "z"`, true, true},
		{`rule a = ("x" #missing #recover(skip="y")) "z"`, false, true},
		{`rule a = "(" #missing a "x" / "y"`, false, true},
	} {
		m, err := CoreFrom(compiletest.Program(t, tc.src, compile.Options{}), nil)
		if err != nil {
			t.Fatal(err)
		}
		core.Analyze(m)
		var found []core.Node
		var walk func(core.Node)
		walk = func(n core.Node) {
			switch n.(type) {
			case *core.Recover, *core.Missing:
				found = append(found, n)
				return
			}
			for _, c := range core.Children(n) {
				walk(c)
			}
		}
		walk(m.Rules[0].Body)
		if len(found) != 1 {
			t.Fatalf("%s: %d recovery or insertion nodes, want 1", tc.src, len(found))
		}
		if at := found[0].Attrs(); at.Nullable != tc.null || at.NullableIns != tc.ins {
			t.Errorf("%s: %T nullable=%v nullable-ins=%v, want %v %v", tc.src, found[0], at.Nullable, at.NullableIns, tc.null, tc.ins)
		}
	}
}
