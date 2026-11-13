package engine

import (
	"runtime"
	"slices"
	"sync"
	"testing"

	"zeroparse.dev/zp/internal/codegen"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/gentest"
	"zeroparse.dev/zp/internal/ir/core"
	irbuild "zeroparse.dev/zp/internal/ir/core/build"
)

// coreCorpusProgram compiles a corpus grammar for the Core IR tests, or reports false for a
// grammar that does not compile or that generated parsers refuse (#missing).
func coreCorpusProgram(t *testing.T, c gentest.Case) (*Program, bool) {
	t.Helper()
	g, err := parseTest(c.Src)
	if err != nil {
		return nil, false
	}
	prog, err := Compile(g, compile.Options{})
	if err != nil || prog.HasMissing {
		return nil, false
	}
	return prog, true
}

// TestCoreBuildCorpus builds, analyzes and verifies the Core IR of every grammar of the corpus
// and of testdata/grammars, and checks that the scope of every rule is the frame layout the
// generated Node rules use, which the typed rules share.
func TestCoreBuildCorpus(t *testing.T) {
	t.Parallel()
	cases := gentest.CorpusWithParsers(t)
	built := 0
	for _, ok := range mapParallel(len(cases), func(i int) bool { return checkCoreBuild(t, cases[i]) }) {
		if ok {
			built++
		}
	}
	if built < 100 {
		t.Fatalf("built %d grammars", built)
	}
}

// mapParallel returns f(0), ..., f(n-1), computed on at most GOMAXPROCS goroutines. f must not call
// t.Fatal, t.FailNow or t.Skip, which are for the goroutine of the test.
func mapParallel[T any](n int, f func(i int) T) []T {
	out := make([]T, n)
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	for i := range n {
		sem <- struct{}{}
		wg.Go(func() {
			defer func() { <-sem }()
			out[i] = f(i)
		})
	}
	wg.Wait()
	return out
}

// checkCoreBuild builds, analyzes and verifies the Core IR of the grammar c and compares its scopes
// with the generated Node rules. It reports whether it built the grammar.
func checkCoreBuild(t *testing.T, c gentest.Case) bool {
	prog, ok := coreCorpusProgram(t, c)
	if !ok {
		return false
	}
	m, err := irbuild.CoreFrom(prog.Static(), nil)
	if err != nil {
		t.Errorf("%s: %v", c.Name, err)
		return false
	}
	core.Analyze(m)
	if err := core.Verify(m); err != nil {
		t.Errorf("%s: %v", c.Name, err)
		return false
	}
	g, ok := coreGenerate(c.Src, codegen.GenOptions{})
	if !ok {
		return false
	}
	for _, r := range typedRules(g) {
		if _, ok := m.Rules[r.ID].Body.(*core.OperatorsRule); ok {
			continue
		}
		if got, want := core.SlotNames(m.Rules[r.ID].Scope), g.NodeScopes[r]; !slices.Equal(got, want) {
			t.Errorf("%s: rule %s (%d): scope %v, generated Node rule %v", c.Name, r.Name, r.ID, got, want)
		}
	}
	return true
}
