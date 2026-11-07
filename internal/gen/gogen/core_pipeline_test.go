package gogen_test

import (
	"fmt"
	"runtime"
	"sync"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gen/gogen"
	"zeroparse.dev/zp/internal/gentest"
	"zeroparse.dev/zp/internal/ir/core"
	irbuild "zeroparse.dev/zp/internal/ir/core/build"
)

// legacyTypedCore is the sequence of passes that Generator.TypedCore ran before the pipeline of Core
// IR (design 063), as the oracle of the pipeline: it must give the same module for every grammar and
// mode.
func legacyTypedCore(g *gogen.Generator, opts gen.GenOptions) (*core.Module, error) {
	m, err := irbuild.CoreFrom(g.Prog, map[core.Node]grammar.Expr{})
	if err != nil {
		return nil, err
	}
	leanTyped := g.Ordinary && g.GoBudgetControls
	if leanTyped && !g.Observability && !opts.DisableTerminalPrograms {
		core.Terminals(m, gogen.CoreLocalRule)
	}
	core.RewriteTerms(m, core.TermOptions{TextDispatch: !g.DisableLiteralTextDispatch, FieldCompare: !g.BoxedComparisons})
	core.Analyze(m)
	core.Guards(m, g.Ordinary && !g.DisableGuardedClasses)
	core.Scans(m)
	if leanTyped {
		core.ClassRuns(m)
	}
	core.Analyze(m)
	if err := core.Verify(m); err != nil {
		return nil, err
	}
	return m, nil
}

// pipelinedTypedCore runs the pipeline with the mode and the flags that the generator g has.
func pipelinedTypedCore(g *gogen.Generator, opts gen.GenOptions, verifyEach bool) (*core.Module, []string, error) {
	m, err := irbuild.CoreFrom(g.Prog, map[core.Node]grammar.Expr{})
	if err != nil {
		return nil, nil, err
	}
	env := &core.Env{
		Mode: core.Mode{Lean: g.Ordinary && g.GoBudgetControls, Ordinary: g.Ordinary, Observe: g.Observability},
		Flag: func(name string) bool {
			switch name {
			case core.PassTerminals:
				return !opts.DisableTerminalPrograms
			case core.FlagTextDispatch:
				return !g.DisableLiteralTextDispatch
			case core.FlagFieldCompare:
				return !g.BoxedComparisons
			case core.FlagGuardedClasses:
				return !g.DisableGuardedClasses
			}
			return true
		},
		Local: gogen.CoreLocalRule,
	}
	ran, err := core.Run(m, env, core.RunOptions{VerifyEach: verifyEach})
	return m, ran, err
}

// The pipeline gives the module of the sequence that TypedCore ran before, for every grammar of the
// corpus and every mode, with or without verification between the passes.
func TestCorePipelineMatchesTheLegacySequence(t *testing.T) {
	modes := []struct {
		name string
		opts gen.GenOptions
	}{
		{"lean", gen.GenOptions{Types: true, TypedOnly: true}},
		{"lean observable", gen.GenOptions{Types: true, TypedOnly: true, Observability: true}},
		{"controlled", gen.GenOptions{Types: true, TypedOnly: true, LimitControls: true}},
		{"lean boxed", gen.GenOptions{Types: true, TypedOnly: true, DisableLiteralTextDispatch: true, BoxedComparisons: true}},
		{"lean without guarded classes", gen.GenOptions{Types: true, TypedOnly: true, DisableGuardedClasses: true}},
		{"lean without terminals", gen.GenOptions{Types: true, TypedOnly: true, DisableTerminalPrograms: true}},
		{"document", gen.GenOptions{Types: true, Document: true}},
	}
	cases := append(gentest.CorpusWithParsers(t), gentest.CoreExtraGrammars...)
	compared := 0
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	for _, mode := range modes {
		for _, c := range cases {
			wg.Go(func() {
				sem <- struct{}{}
				defer func() { <-sem }()
				pg, err := parseTest(c.Src)
				if err != nil {
					return
				}
				var g *gogen.Generator
				opts := mode.opts
				opts.Module = "example.com/p"
				if _, err := gogen.GenerateParser(pg, gentest.Compile, opts, &gogen.Hooks{Generator: func(x *gogen.Generator) { g = x }}); err != nil || g == nil {
					return
				}
				want, err := legacyTypedCore(g, opts)
				if err != nil || len(want.Rules) == 0 {
					return
				}
				for _, each := range []bool{false, true} {
					got, _, err := pipelinedTypedCore(g, opts, each)
					if err != nil {
						t.Errorf("%s: %s (verify each %v): %v", mode.name, c.Name, each, err)
						return
					}
					if a, b := core.Print(want, true), core.Print(got, true); a != b {
						t.Errorf("%s: %s (verify each %v): the pipeline gives another module than the legacy sequence", mode.name, c.Name, each)
						return
					}
				}
				mu.Lock()
				compared++
				mu.Unlock()
			})
		}
	}
	wg.Wait()
	if compared < 500 {
		t.Fatalf("compared %d modules", compared)
	}
	fmt.Println("compared", compared)
}
