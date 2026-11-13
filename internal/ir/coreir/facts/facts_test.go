package facts_test

import (
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/compiletest"
	"zeroparse.dev/zp/internal/factstest"
	"zeroparse.dev/zp/internal/ir/coreir"
	"zeroparse.dev/zp/internal/ir/coreir/build"
	"zeroparse.dev/zp/internal/ir/coreir/facts"
)

// TestFactsGolden computes the facts of every corpus grammar on its Core IR module, with the
// rules' policies cleared so that no fact is read back from compilation, and compares them byte
// for byte with the pinned facts (internal/factstest).
func TestFactsGolden(t *testing.T) {
	t.Parallel()
	for _, path := range factstest.Corpus {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			g, err := compiletest.Parse(factstest.Read(t, path))
			if err != nil {
				t.Fatal(err)
			}
			prog, err := compiletest.Compile(g, compile.Options{})
			if err != nil {
				t.Fatal(err)
			}
			origin := map[coreir.Node]grammar.Expr{}
			m, err := build.CoreFrom(prog, origin)
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range m.Rules {
				r.Policy = coreir.Policy{}
			}
			f := facts.Compute(m, nil)
			factstest.Golden(t, path, factstest.Render(factstest.CoreRules(m, f, origin)))
		})
	}
}
