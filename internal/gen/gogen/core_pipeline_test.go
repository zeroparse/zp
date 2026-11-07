package gogen_test

import (
	"bytes"
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

// legacyRun runs the passes in the hand-written sequences that Generator.TypedCore and
// gen.Generator.StartRecFlow had before the pipeline of Core IR (design 063), as the oracle of the
// pipeline: for every grammar and mode it must give the same module, and so the same parser. The
// sequence of StartRecFlow is the one with PrepareScans.
func legacyRun(m *core.Module, env *core.Env, _ core.RunOptions) ([]string, error) {
	if env.PrepareScans == nil {
		if env.Mode.Lean && !env.Mode.Observe && env.On(core.PassTerminals) {
			core.Terminals(m, env.Local)
		}
		core.RewriteTerms(m, core.TermOptions{TextDispatch: env.On(core.FlagTextDispatch), FieldCompare: env.On(core.FlagFieldCompare)})
		core.Analyze(m)
		core.Guards(m, env.Mode.Ordinary && env.On(core.FlagGuardedClasses))
		core.Scans(m)
		if env.Mode.Lean {
			core.ClassRuns(m)
		}
	} else {
		core.RewriteTerms(m, core.TermOptions{TextDispatch: env.On(core.FlagTextDispatch), FieldCompare: env.On(core.FlagFieldCompare)})
		core.Analyze(m)
		core.Guards(m, env.Mode.Ordinary && env.On(core.FlagGuardedClasses))
		finish := env.PrepareScans(m)
		core.Scans(m)
		finish()
	}
	core.Analyze(m)
	return nil, core.Verify(m)
}

// typedEnv returns the environment in which Generator.TypedCore runs the pipeline for g.
func typedEnv(g *gogen.Generator, opts gen.GenOptions) *core.Env {
	return &core.Env{
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
}

// pipelinedTypedCore runs the pipeline, or the legacy sequence if legacy is set, on the typed rules of g.
func pipelinedTypedCore(g *gogen.Generator, opts gen.GenOptions, legacy, verifyEach bool) (*core.Module, error) {
	m, err := irbuild.CoreFrom(g.Prog, map[core.Node]grammar.Expr{})
	if err != nil {
		return nil, err
	}
	run := core.Run
	if legacy {
		run = legacyRun
	}
	_, err = run(m, typedEnv(g, opts), core.RunOptions{VerifyEach: verifyEach})
	return m, err
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
				want, err := pipelinedTypedCore(g, opts, true, false)
				if err != nil || len(want.Rules) == 0 {
					return
				}
				for _, each := range []bool{false, true} {
					got, err := pipelinedTypedCore(g, opts, false, each)
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
}

// The parsers generated through the pipeline are the bytes that the legacy sequences give, in both
// routes that run Core passes (the typed rules, and the Node and recognizer tables), for every grammar of
// the corpus.
func TestGeneratedParsersMatchTheLegacySequence(t *testing.T) {
	sets := []gen.GenOptions{
		{Types: true, TypedOnly: true},
		{Types: true, TypedOnly: true, Observability: true},
		{Types: true, TypedOnly: true, LimitControls: true},
		{Types: true, TypedOnly: true, DisableLiteralTextDispatch: true, BoxedComparisons: true},
		{Types: true, TypedOnly: true, DisableGuardedClasses: true},
		{Types: true, TypedOnly: true, DisableTerminalPrograms: true},
		{Types: true, Recognize: true},
		{Recognize: true},
		{Recognize: true, LimitControls: true, Observability: true},
		{Recognize: true, DisableGuardedClasses: true, DisableLiteralTextDispatch: true},
		{Document: true, Types: true},
		{Stream: true, Types: true},
	}
	cases := append(gentest.CorpusWithParsers(t), gentest.CoreExtraGrammars...)
	var mu sync.Mutex
	compared := 0
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	for i, opts := range sets {
		for _, c := range cases {
			wg.Go(func() {
				sem <- struct{}{}
				defer func() { <-sem }()
				pg, err := parseTest(c.Src)
				if err != nil {
					return
				}
				opts := opts
				opts.Module, opts.Runtime = "example.com/p", gen.RuntimeImport
				want, err := gogen.GenerateParser(pg, gentest.Compile, opts, &gogen.Hooks{CoreRun: legacyRun})
				if err != nil {
					return
				}
				got, err := gogen.GenerateParser(pg, gentest.Compile, opts, nil)
				if err != nil {
					t.Errorf("set %d: %s: %v, which the legacy sequence generates", i, c.Name, err)
					return
				}
				if len(got) != len(want) {
					t.Errorf("set %d: %s: %d files, want %d", i, c.Name, len(got), len(want))
					return
				}
				for j := range got {
					if got[j].Path != want[j].Path || !bytes.Equal(got[j].Content, want[j].Content) {
						t.Errorf("set %d: %s: %s differs from the legacy sequence's", i, c.Name, got[j].Path)
					}
				}
				mu.Lock()
				compared++
				mu.Unlock()
			})
		}
	}
	wg.Wait()
	if compared < 1500 {
		t.Fatalf("compared %d parsers", compared)
	}
}
