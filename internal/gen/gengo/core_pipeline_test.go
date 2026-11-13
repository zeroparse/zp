package gengo_test

import (
	"runtime"
	"sync"
	"testing"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/gen"
	"zeroparse.dev/zp/internal/gen/gengo"
	"zeroparse.dev/zp/internal/gentest"
	"zeroparse.dev/zp/internal/ir/core"
	irbuild "zeroparse.dev/zp/internal/ir/core/build"
)

// typedEnv returns the environment in which Generator.TypedCore runs the pipeline for g.
func typedEnv(g *gengo.Generator) *core.Env {
	return &core.Env{
		Mode:  core.Mode{Lean: g.Ordinary, Ordinary: g.Ordinary, Observe: g.Observability},
		Flag:  g.Opt.On,
		Local: gengo.CoreLocalRule,
	}
}

// pipelinedTypedCore runs the pipeline on the typed rules of g.
func pipelinedTypedCore(g *gengo.Generator, verifyEach bool) (*core.Module, error) {
	m, err := irbuild.CoreFrom(g.Prog, map[core.Node]grammar.Expr{})
	if err != nil {
		return nil, err
	}
	_, err = core.Run(m, typedEnv(g), core.RunOptions{VerifyEach: verifyEach})
	return m, err
}

// The pipeline runs on the typed rules of every grammar of the corpus in every mode, and the module
// verifies after every pass and gives the same Core IR with or without verification between the passes
// (verification only reads).
func TestCorePipelineVerifiesEveryPass(t *testing.T) {
	modes := []struct {
		name string
		opts gen.GenOptions
	}{
		{"lean", gen.GenOptions{Types: true, TypedOnly: true}},
		{"lean observable", gen.GenOptions{Types: true, TypedOnly: true, Observability: true}},
		{"controlled", gen.GenOptions{Types: true, TypedOnly: true, LimitControls: true}},
		{"lean boxed", gen.GenOptions{Types: true, TypedOnly: true, NoOpt: []string{"text-dispatch", "field-compare"}}},
		{"lean without guarded classes", gen.GenOptions{Types: true, TypedOnly: true, NoOpt: []string{"guarded-classes"}}},
		{"lean without terminals", gen.GenOptions{Types: true, TypedOnly: true, NoOpt: []string{"terminals"}}},
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
				var g *gengo.Generator
				opts := mode.opts
				opts.Module = "example.com/p"
				if _, err := gengo.GenerateParser(pg, gentest.Compile, gentest.WithRuntime(opts), &gengo.Hooks{Generator: func(x *gengo.Generator) { g = x }}); err != nil || g == nil {
					return
				}
				want, err := pipelinedTypedCore(g, false)
				if err != nil {
					t.Errorf("%s: %s: %v", mode.name, c.Name, err)
					return
				}
				if len(want.Rules) == 0 {
					return
				}
				got, err := pipelinedTypedCore(g, true)
				if err != nil {
					t.Errorf("%s: %s: %v", mode.name, c.Name, err)
					return
				}
				if a, b := core.Print(want, true), core.Print(got, true); a != b {
					t.Errorf("%s: %s: verifying between the passes changes the module", mode.name, c.Name)
					return
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
