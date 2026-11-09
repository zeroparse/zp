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
// facts that compilation pins (internal/factstest). The rules on which they differ are pinned in
// core-differences.txt.
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
	factstest.GoldenFile(t, "core-differences.txt", diffs.String())
}
