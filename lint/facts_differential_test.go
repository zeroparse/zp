package lint

import (
	"testing"

	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	"zeroparse.dev/zp/internal/desugar"
	"zeroparse.dev/zp/internal/factstest"
	"zeroparse.dev/zp/internal/frontend"
)

// TestRuleTablesMatchFacts requires, for every corpus grammar that compiles, that the rule tables
// of the analysis (nullability as the engine finds it, left-recursion cycles and rules that always
// succeed) equal the rule facts of the front end's Core module.
func TestRuleTablesMatchFacts(t *testing.T) {
	t.Parallel()
	for _, path := range factstest.Corpus {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			g, err := compiletest.Parse(factstest.Read(t, path))
			if err != nil {
				t.Fatal(err)
			}
			res, err := frontend.Run(g, compile.Options{}, nil)
			if err != nil {
				return // the grammar does not compile
			}
			lowered, _, err := desugar.LowerInlineActions(g)
			if err != nil {
				t.Fatal(err)
			}
			a := newAnalysis(lowered)
			byName := map[string]int{}
			for _, r := range res.Module.Rules {
				if r != nil {
					byName[r.Name] = r.ID
				}
			}
			for _, r := range a.order {
				id, ok := byName[r.Name]
				if !ok {
					t.Errorf("%s: no rule in the module", r.Name)
					continue
				}
				f := res.Facts.Rules[id]
				if got, want := a.engineNullable[r.Name], f.Nullable; got != want {
					t.Errorf("%s: engineNullable = %v, Nullable = %v", r.Name, got, want)
				}
				if got, want := a.leftRecursive[r.Name], f.Cyclic(); got != want {
					t.Errorf("%s: leftRecursive = %v, Cyclic = %v", r.Name, got, want)
				}
				if got, want := a.engineSucceeding[r.Name], f.Succeeds; got != want {
					t.Errorf("%s: engineSucceeding = %v, Succeeds = %v", r.Name, got, want)
				}
			}
		})
	}
}
